package goevent

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// redisDriver implements the Driver interface using Redis pub/sub
// Enables distributed event handling across multiple processes/servers
type redisDriver struct {
	client    *redis.Client
	pubClient *redis.Client // Separate client for publishing to avoid blocking
	config    *RedisConfig

	// Subscription management
	subscriptions map[string]*redisSubscription
	subsMu        sync.RWMutex

	// Wait group for local async handlers only
	wg sync.WaitGroup

	// Shutdown coordination
	ctx    context.Context
	cancel context.CancelFunc
}

// redisSubscription represents a single event subscription
type redisSubscription struct {
	eventName string
	handlers  []redisHandler
	pubsub    *redis.PubSub
	cancel    context.CancelFunc
}

// redisHandler wraps an EventHandler with its execution mode
type redisHandler struct {
	handler EventHandler
	isAsync bool
}

// redisMessage is the serialized message format sent via Redis
type redisMessage struct {
	EventName string          `json:"event_name"`
	EventType string          `json:"event_type"` // For deserialization (e.g., "*goevent.UserCreatedEvent")
	EventData json.RawMessage `json:"event_data"` // The entire event serialized as JSON
	HandleID  string          `json:"handle_id"`  // For tracking (though distributed tracking is limited)
	Timestamp int64           `json:"timestamp"`  // Unix timestamp
}

// newRedisDriver creates a new Redis driver
func newRedisDriver(config *RedisConfig) (Driver, error) {
	if config == nil {
		return nil, fmt.Errorf("RedisConfig cannot be nil")
	}

	// Set default values
	config.setDefaults()

	// Create Redis client options
	opts := &redis.Options{
		Addr:         config.Addr,
		Password:     config.Password,
		DB:           config.DB,
		PoolSize:     config.PoolSize,
		MinIdleConns: config.MinIdleConns,
		DialTimeout:  config.DialTimeout,
		ReadTimeout:  config.ReadTimeout,
		WriteTimeout: config.WriteTimeout,
		TLSConfig:    config.TLSConfig,
	}

	// Create clients
	client := redis.NewClient(opts)
	pubClient := redis.NewClient(opts)

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	// Create driver context
	driverCtx, driverCancel := context.WithCancel(context.Background())

	return &redisDriver{
		client:        client,
		pubClient:     pubClient,
		config:        config,
		subscriptions: make(map[string]*redisSubscription),
		ctx:           driverCtx,
		cancel:        driverCancel,
	}, nil
}

// Publish serializes and publishes an event to Redis
func (rd *redisDriver) Publish(eventName string, handle *DispatchHandle, event Event) error {
	// Serialize the entire event to JSON
	eventData, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	// Check size limit
	if len(eventData) > rd.config.MaxEventSize {
		return fmt.Errorf("event size %d exceeds maximum %d bytes", len(eventData), rd.config.MaxEventSize)
	}

	// Create message
	msg := redisMessage{
		EventName: eventName,
		EventType: event.Name(), // Use event.Name() for cross-service compatibility
		EventData: eventData,
		HandleID:  handle.id,
		Timestamp: time.Now().Unix(),
	}

	// Serialize message
	msgData, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	// Publish with retries
	return rd.publishWithRetry(eventName, msgData)
}

// publishWithRetry attempts to publish with exponential backoff
func (rd *redisDriver) publishWithRetry(eventName string, msgData []byte) error {
	channel := rd.channelName(eventName)

	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		ctx, cancel := context.WithTimeout(rd.ctx, rd.config.WriteTimeout)
		err := rd.pubClient.Publish(ctx, channel, msgData).Err()
		cancel()

		if err == nil {
			return nil
		}

		lastErr = err

		// If connection error, retry with backoff
		if isConnectionError(err) && attempt < 2 {
			time.Sleep(time.Duration(attempt+1) * 100 * time.Millisecond)
			continue
		}

		// Non-retryable error or final attempt
		break
	}

	return fmt.Errorf("failed to publish after 3 attempts: %w", lastErr)
}

// Subscribe registers a handler for an event
func (rd *redisDriver) Subscribe(eventName string, handler EventHandler, isAsync bool) error {
	rd.subsMu.Lock()
	defer rd.subsMu.Unlock()

	sub, exists := rd.subscriptions[eventName]
	if !exists {
		// Create new subscription
		channel := rd.channelName(eventName)
		pubsub := rd.client.Subscribe(rd.ctx, channel)

		// Test subscription
		ctx, cancel := context.WithTimeout(rd.ctx, 2*time.Second)
		_, err := pubsub.Receive(ctx)
		cancel()

		if err != nil {
			pubsub.Close()
			return fmt.Errorf("failed to subscribe to channel %s: %w", channel, err)
		}

		// Create subscription context
		subCtx, subCancel := context.WithCancel(rd.ctx)

		sub = &redisSubscription{
			eventName: eventName,
			handlers:  make([]redisHandler, 0),
			pubsub:    pubsub,
			cancel:    subCancel,
		}

		rd.subscriptions[eventName] = sub

		// Start message processing goroutine
		go rd.processMessages(subCtx, sub)
	}

	// Add handler to subscription
	sub.handlers = append(sub.handlers, redisHandler{
		handler: handler,
		isAsync: isAsync,
	})

	return nil
}

// processMessages processes incoming messages for a subscription
func (rd *redisDriver) processMessages(ctx context.Context, sub *redisSubscription) {
	ch := sub.pubsub.Channel()

	for {
		select {
		case msg := <-ch:
			if msg == nil {
				// Channel closed
				return
			}

			// Deserialize message
			var redisMsg redisMessage
			if err := json.Unmarshal([]byte(msg.Payload), &redisMsg); err != nil {
				log.Printf("goevent: failed to unmarshal message: %v", err)
				continue
			}

			// Reconstruct event
			event, err := rd.deserializeEvent(redisMsg)
			if err != nil {
				log.Printf("goevent: failed to deserialize event: %v", err)
				continue
			}

			// Create a local handle for tracking
			handle := &DispatchHandle{
				id:       redisMsg.HandleID,
				errors:   make([]*EventError, 0),
				done:     make(chan struct{}),
				isLocal:  false, // This is a remote event
			}

			// Execute all handlers for this subscription
			for _, h := range sub.handlers {
				if h.isAsync {
					rd.wg.Add(1)
					handle.wg.Add(1)
					go func(handler EventHandler) {
						defer rd.wg.Done()
						defer handle.wg.Done()
						handler(handle, event)
					}(h.handler)
				} else {
					h.handler(handle, event)
				}
			}

		case <-ctx.Done():
			return
		}
	}
}

// deserializeEvent reconstructs an Event from a redisMessage
func (rd *redisDriver) deserializeEvent(msg redisMessage) (Event, error) {
	event, err := globalRegistry.Create(msg.EventType, msg.EventData)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to deserialize event %s (type: %s): %w",
			msg.EventName, msg.EventType, err,
		)
	}
	return event, nil
}

// Wait blocks until all local async handlers complete
func (rd *redisDriver) Wait() {
	rd.wg.Wait()
}

// Close cleanly shuts down the Redis driver
func (rd *redisDriver) Close() error {
	// Signal shutdown
	rd.cancel()

	// Wait for handlers to complete
	rd.Wait()

	// Close all subscriptions
	rd.subsMu.Lock()
	for _, sub := range rd.subscriptions {
		sub.cancel()
		sub.pubsub.Close()
	}
	rd.subsMu.Unlock()

	// Close clients
	if err := rd.client.Close(); err != nil {
		return fmt.Errorf("failed to close Redis client: %w", err)
	}

	if err := rd.pubClient.Close(); err != nil {
		return fmt.Errorf("failed to close Redis pub client: %w", err)
	}

	return nil
}

// channelName returns the Redis channel name for an event
func (rd *redisDriver) channelName(eventName string) string {
	return rd.config.ChannelPrefix + eventName
}

// isConnectionError checks if an error is a connection-related error
func isConnectionError(err error) bool {
	if err == nil {
		return false
	}
	// Check for common Redis connection errors
	return err == redis.Nil ||
		err == context.DeadlineExceeded ||
		err.Error() == "redis: client is closed"
}
