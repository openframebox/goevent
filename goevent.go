// Package goevent provides a type-safe, flexible event bus wrapper for Go.
//
// GoEvent wraps the EventBus library with enhanced features:
//   - Type-safe interfaces instead of reflection-based handlers
//   - Configurable sync/async execution per listener
//   - Per-event waiting with DispatchHandle
//   - Built-in error collection and reporting
//   - Thread-safe operations with proper synchronization
//
// Basic usage:
//
//	bus := goevent.New()
//	bus.RegisterListener(&MyListener{})
//	handle := bus.Dispatch(&MyEvent{})
//	handle.Wait()  // Wait for completion
package goevent

import (
	"fmt"
	"sync"

	"github.com/asaskevich/EventBus"
)

// GoEvent is a wrapper around EventBus with enhanced error handling and synchronization
type GoEvent struct {
	bus              EventBus.Bus
	wg               sync.WaitGroup
	errorsMu         sync.Mutex
	errors           []*EventError
	asyncListenersMu sync.RWMutex
	asyncListeners   map[string]int // tracks count of async listeners per event
}

// New creates a new GoEvent instance
func New() *GoEvent {
	return &GoEvent{
		bus:            EventBus.New(),
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

	// Create a wrapper function that matches EventBus signature
	// and handles error collection for both handle and global errors
	handler := func(args ...interface{}) {
		if len(args) < 2 {
			return
		}

		// Extract dispatch handle and event from args
		handle, okHandle := args[0].(*DispatchHandle)
		event, okEvent := args[1].(Event)

		if !okHandle || !okEvent {
			return
		}

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

	// Subscribe based on async flag
	if isAsync {
		// Track async listener count for this event
		ge.asyncListenersMu.Lock()
		ge.asyncListeners[eventName]++
		ge.asyncListenersMu.Unlock()

		// Wrap async handler with WaitGroup tracking
		asyncHandler := func(args ...interface{}) {
			// Extract handle to decrement its WaitGroup too
			if len(args) >= 1 {
				if handle, ok := args[0].(*DispatchHandle); ok {
					defer handle.wg.Done()
				}
			}
			defer ge.wg.Done() // Global WaitGroup was incremented during Dispatch
			handler(args...)
		}
		ge.bus.SubscribeAsync(eventName, asyncHandler, false)
	} else {
		// Synchronous subscription
		ge.bus.Subscribe(eventName, handler)
	}
}

// Dispatch publishes an event to all registered listeners and returns a handle
// The handle can be used to wait for this specific dispatch to complete
// and retrieve errors that occurred during this dispatch
func (ge *GoEvent) Dispatch(event Event) *DispatchHandle {
	eventName := event.Name()

	// Create a dispatch handle for this specific dispatch
	handle := &DispatchHandle{
		errors: make([]*EventError, 0),
		done:   make(chan struct{}),
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

	// Publish the event with the handle as first argument
	ge.bus.Publish(eventName, handle, event)

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
