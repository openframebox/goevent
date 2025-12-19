package goevent

// Driver abstracts the underlying event dispatch mechanism
// Implementations include in-memory (memoryDriver) and distributed (redisDriver)
type Driver interface {
	// Publish sends an event to all registered listeners
	// Returns an error if the event cannot be published
	Publish(eventName string, handle *DispatchHandle, event Event) error

	// Subscribe registers a handler for an event
	// isAsync indicates if the handler should execute asynchronously
	Subscribe(eventName string, handler EventHandler, isAsync bool) error

	// Unsubscribe removes all handlers for a specific event
	// This is idempotent - calling it multiple times for the same event is safe
	// Returns nil if the event was not subscribed or if unsubscribe succeeded
	Unsubscribe(eventName string) error

	// Wait blocks until all pending async operations complete
	// For memory driver: waits for all async handlers
	// For Redis driver: waits for local async handlers only
	Wait()

	// Close cleanly shuts down the driver and releases resources
	Close() error
}

// EventHandler is the normalized handler signature used by drivers
// The handler receives the dispatch handle and event to process
type EventHandler func(handle *DispatchHandle, event Event)
