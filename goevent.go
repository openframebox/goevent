// Package goevent provides a type-safe, flexible event bus wrapper for Go.
//
// GoEvent supports two drivers:
//   - Memory driver (default): In-memory EventBus for same-process communication
//   - Redis driver: Redis pub/sub for distributed multi-process/multi-server communication
//
// Features:
//   - Type-safe interfaces instead of reflection-based handlers
//   - Configurable sync/async execution per listener
//   - Per-event waiting with DispatchHandle
//   - Built-in error collection and reporting
//   - Thread-safe operations with proper synchronization
//
// Basic usage (memory driver):
//
//	bus := goevent.New()
//	bus.RegisterListener(&MyListener{})
//	handle := bus.Dispatch(&MyEvent{})
//	handle.Wait()  // Wait for completion
//
// Distributed usage (Redis driver):
//
//	bus := goevent.NewWithConfig(&goevent.Config{
//	    Driver: goevent.DriverRedis,
//	    Redis: &goevent.RedisConfig{
//	        Addr: "localhost:6379",
//	    },
//	})
//	bus.RegisterListener(&MyListener{})
//	handle := bus.Dispatch(&MyEvent{})
package goevent

import (
	"fmt"
	"sync"
)

// GoEvent is a type-safe event bus with pluggable drivers
type GoEvent struct {
	driver           Driver
	wg               sync.WaitGroup
	errorsMu         sync.Mutex
	errors           []*EventError
	asyncListenersMu sync.RWMutex
	asyncListeners   map[string]int // tracks count of async listeners per event
}

// New creates a new GoEvent instance with the default in-memory driver
// This provides backward compatibility with existing code
func New() *GoEvent {
	return NewWithConfig(nil)
}

// NewWithConfig creates a new GoEvent instance with custom configuration
// If cfg is nil, defaults to memory driver
func NewWithConfig(cfg *Config) *GoEvent {
	if cfg == nil {
		cfg = &Config{Driver: DriverMemory}
	}

	var driver Driver
	var err error

	switch cfg.Driver {
	case DriverRedis:
		if cfg.Redis == nil {
			panic("Redis driver requires RedisConfig")
		}
		driver, err = newRedisDriver(cfg.Redis)
		if err != nil {
			panic(fmt.Sprintf("failed to create Redis driver: %v", err))
		}
	case DriverMemory:
		fallthrough
	default:
		driver = newMemoryDriver()
	}

	return &GoEvent{
		driver:         driver,
		errors:         make([]*EventError, 0),
		asyncListeners: make(map[string]int),
	}
}

// RegisterListener registers one or more listeners to the event bus
// If a listener implements ListenerWithOptions and Options().Async is true,
// it will execute asynchronously. Otherwise, it executes synchronously.
func (ge *GoEvent) RegisterListener(listeners ...Listener) {
	for _, listener := range listeners {
		ge.registerSingleListener(listener)
	}
}

func (ge *GoEvent) registerSingleListener(listener Listener) {
	// Check if listener has custom options
	isAsync := false
	if listenerWithOpts, ok := listener.(ListenerWithOptions); ok {
		isAsync = listenerWithOpts.Options().Async
	}

	eventName := listener.EventName()

	// Create normalized EventHandler
	handler := func(handle *DispatchHandle, event Event) {
		// Call the listener's OnEvent handler
		if err := listener.OnEvent(event); err != nil {
			eventError := &EventError{
				EventName:    eventName,
				ListenerType: fmt.Sprintf("%T", listener),
				Err:          err,
			}

			// Record error to both the dispatch handle and global errors
			handle.recordError(eventError)
			ge.recordError(eventError)
		}
	}

	// Track async listener count for this event
	if isAsync {
		ge.asyncListenersMu.Lock()
		ge.asyncListeners[eventName]++
		ge.asyncListenersMu.Unlock()

		// Wrap async handler with WaitGroup tracking
		wrappedHandler := func(handle *DispatchHandle, event Event) {
			defer handle.wg.Done()
			defer ge.wg.Done()
			handler(handle, event)
		}

		ge.driver.Subscribe(eventName, wrappedHandler, true)
	} else {
		// Synchronous subscription
		ge.driver.Subscribe(eventName, handler, false)
	}
}

// UnregisterListenersForEvent removes all listeners for a specific event
// After calling this, no listeners will handle events with this name
// This is thread-safe and can be called while events are being dispatched
//
// Example:
//
//	evt.UnregisterListenersForEvent("user.created")
func (ge *GoEvent) UnregisterListenersForEvent(eventName string) error {
	// Update async listener count
	ge.asyncListenersMu.Lock()
	delete(ge.asyncListeners, eventName)
	ge.asyncListenersMu.Unlock()

	// Delegate to driver
	return ge.driver.Unsubscribe(eventName)
}

// Dispatch publishes an event to all registered listeners and returns a handle
// The handle can be used to wait for this specific dispatch to complete
// and retrieve errors that occurred during this dispatch
//
// For memory driver: Wait() blocks until all handlers complete
// For Redis driver: Wait() only blocks for LOCAL handlers (not remote processes)
func (ge *GoEvent) Dispatch(event Event) *DispatchHandle {
	eventName := event.Name()

	// Create a dispatch handle for this specific dispatch
	handle := &DispatchHandle{
		id:      generateHandleID(),
		isLocal: ge.isMemoryDriver(),
		errors:  make([]*EventError, 0),
		done:    make(chan struct{}),
	}

	// Check if there are async listeners for this event
	ge.asyncListenersMu.RLock()
	asyncCount := ge.asyncListeners[eventName]
	ge.asyncListenersMu.RUnlock()

	// Increment WaitGroups before publishing (prevents race with Wait())
	if asyncCount > 0 {
		ge.wg.Add(asyncCount)     // Global wait group
		handle.wg.Add(asyncCount) // Handle-specific wait group
	}

	// Publish the event via driver
	if err := ge.driver.Publish(eventName, handle, event); err != nil {
		// If publish fails, record the error immediately
		publishError := &EventError{
			EventName:    eventName,
			ListenerType: "publisher",
			Err:          fmt.Errorf("failed to publish: %w", err),
		}
		handle.recordError(publishError)
		ge.recordError(publishError)

		// Mark handle as done immediately since publish failed
		handle.markDone()
		return handle
	}

	// Start a goroutine to mark the handle as done when complete
	go func() {
		handle.wg.Wait()
		handle.markDone()
	}()

	return handle
}

// Wait blocks until all asynchronous event handlers have completed
func (ge *GoEvent) Wait() {
	ge.wg.Wait()
}

// GetErrors returns all errors that occurred during event handling
// This method is thread-safe
func (ge *GoEvent) GetErrors() []*EventError {
	ge.errorsMu.Lock()
	defer ge.errorsMu.Unlock()

	// Return a copy to prevent external modification
	errorsCopy := make([]*EventError, len(ge.errors))
	copy(errorsCopy, ge.errors)
	return errorsCopy
}

// ClearErrors clears all recorded errors
func (ge *GoEvent) ClearErrors() {
	ge.errorsMu.Lock()
	defer ge.errorsMu.Unlock()
	ge.errors = make([]*EventError, 0)
}

// recordError stores an error in a thread-safe manner
func (ge *GoEvent) recordError(err *EventError) {
	ge.errorsMu.Lock()
	defer ge.errorsMu.Unlock()
	ge.errors = append(ge.errors, err)
}

// Close cleanly shuts down the event bus and releases resources
// This should be called when the event bus is no longer needed
// It waits for all pending handlers to complete before closing
func (ge *GoEvent) Close() error {
	ge.Wait()
	return ge.driver.Close()
}

// isMemoryDriver returns true if using the memory driver
func (ge *GoEvent) isMemoryDriver() bool {
	_, ok := ge.driver.(*memoryDriver)
	return ok
}
