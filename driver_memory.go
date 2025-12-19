package goevent

import (
	"sync"

	"github.com/asaskevich/EventBus"
)

// memoryDriver implements the Driver interface using in-memory EventBus
// This is the default driver and provides the same functionality as the original implementation
type memoryDriver struct {
	bus EventBus.Bus

	// Subscription tracking for unsubscribe support
	subsMu sync.RWMutex
	subs   map[string][]interface{} // eventName -> wrapper functions
}

// newMemoryDriver creates a new in-memory driver
func newMemoryDriver() Driver {
	return &memoryDriver{
		bus:  EventBus.New(),
		subs: make(map[string][]interface{}),
	}
}

// Publish publishes an event to all registered listeners via the in-memory EventBus
func (md *memoryDriver) Publish(eventName string, handle *DispatchHandle, event Event) error {
	// EventBus.Publish doesn't return errors, it always succeeds for in-memory
	md.bus.Publish(eventName, handle, event)
	return nil
}

// Subscribe registers a handler for an event
// If isAsync is true, the handler executes in a separate goroutine
func (md *memoryDriver) Subscribe(eventName string, handler EventHandler, isAsync bool) error {
	// Wrap the EventHandler to match EventBus's signature (variadic any)
	wrapper := func(args ...any) {
		if len(args) < 2 {
			return
		}

		// Extract handle and event from args
		handle, okHandle := args[0].(*DispatchHandle)
		event, okEvent := args[1].(Event)

		if !okHandle || !okEvent {
			return
		}

		// Call the actual handler
		handler(handle, event)
	}

	// Store wrapper reference for unsubscribe support
	md.subsMu.Lock()
	md.subs[eventName] = append(md.subs[eventName], wrapper)
	md.subsMu.Unlock()

	// Subscribe based on async flag
	if isAsync {
		md.bus.SubscribeAsync(eventName, wrapper, false)
	} else {
		md.bus.Subscribe(eventName, wrapper)
	}

	return nil
}

// Unsubscribe removes all handlers for a specific event
// This is idempotent - calling it multiple times for the same event is safe
func (md *memoryDriver) Unsubscribe(eventName string) error {
	md.subsMu.Lock()
	wrappers, exists := md.subs[eventName]
	if !exists {
		md.subsMu.Unlock()
		return nil // Already unsubscribed or never subscribed
	}

	// Remove from tracking
	delete(md.subs, eventName)
	md.subsMu.Unlock()

	// Unsubscribe each wrapper from EventBus
	for _, wrapper := range wrappers {
		if err := md.bus.Unsubscribe(eventName, wrapper); err != nil {
			// Log error but continue unsubscribing others
			// EventBus.Unsubscribe returns error if handler not found, which is fine
		}
	}

	return nil
}

// Wait blocks until all asynchronous handlers complete
// For memory driver, this is handled by GoEvent's WaitGroup, so this is a no-op
func (md *memoryDriver) Wait() {
	// No-op: waiting is handled by GoEvent.wg
}

// Close cleanly shuts down the memory driver
func (md *memoryDriver) Close() error {
	// EventBus doesn't require cleanup
	return nil
}
