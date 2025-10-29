package main

import (
	"errors"
	"fmt"
	"time"

	"github.com/openframebox/goevent"
)

// --- Synchronous Listener Example ---

// SyncListener executes synchronously (default behavior)
type SyncListener struct{}

func (sl *SyncListener) EventName() string {
	return "user.created"
}

func (sl *SyncListener) OnEvent(event goevent.Event) error {
	payload := event.Payload()
	userID := payload["userId"]
	fmt.Printf("[SYNC] Processing event: %s (UserID: %v)\n", event.Name(), userID)
	return nil
}

// --- Asynchronous Listener Example ---

// AsyncListener executes asynchronously
type AsyncListener struct{}

func (al *AsyncListener) EventName() string {
	return "user.created"
}

func (al *AsyncListener) OnEvent(event goevent.Event) error {
	payload := event.Payload()
	userID := payload["userId"]
	fmt.Printf("[ASYNC] Processing event: %s (UserID: %v)\n", event.Name(), userID)
	// Simulate some async work
	time.Sleep(100 * time.Millisecond)
	fmt.Printf("[ASYNC] Finished processing: %s\n", event.Name())
	return nil
}

// Options makes AsyncListener execute asynchronously
func (al *AsyncListener) Options() goevent.ListenerOptions {
	return goevent.ListenerOptions{Async: true}
}

// --- Error Handling Example ---

// ErrorListener demonstrates error collection
type ErrorListener struct{}

func (el *ErrorListener) EventName() string {
	return "user.deleted"
}

func (el *ErrorListener) OnEvent(event goevent.Event) error {
	payload := event.Payload()
	userID := payload["userId"]
	fmt.Printf("[ERROR-LISTENER] Processing event: %s (UserID: %v)\n", event.Name(), userID)
	return errors.New("simulated error during event processing")
}

// --- Event Types ---

type UserCreatedEvent struct{}

func (e *UserCreatedEvent) Name() string {
	return "user.created"
}

func (e *UserCreatedEvent) Payload() map[string]any {
	return map[string]any{
		"userId": 123,
	}
}

type UserDeletedEvent struct{}

func (e *UserDeletedEvent) Name() string {
	return "user.deleted"
}

func (e *UserDeletedEvent) Payload() map[string]any {
	return map[string]any{
		"userId": 456,
	}
}

// --- Main Example ---

func main() {
	fmt.Println("=== GoEvent Wrapper Example ===")

	// Create new event bus
	evt := goevent.New()

	// Register listeners
	// - SyncListener will execute synchronously
	// - AsyncListener will execute asynchronously (implements ListenerWithOptions)
	// - ErrorListener will produce an error that we can collect
	evt.RegisterListener(
		&SyncListener{},
		&AsyncListener{},
		&ErrorListener{},
	)

	// Pattern 1: Per-event waiting
	fmt.Println("\n--- Pattern 1: Per-Event Waiting ---")
	fmt.Println("Dispatching UserCreatedEvent (has sync + async listeners):")
	handle1 := evt.Dispatch(&UserCreatedEvent{})

	fmt.Println("Waiting for this specific dispatch to complete...")
	handle1.Wait()
	fmt.Println("✓ UserCreatedEvent handlers completed")

	// Check errors for this specific dispatch
	if errs := handle1.GetErrors(); len(errs) > 0 {
		fmt.Printf("Errors from this dispatch: %d\n", len(errs))
	}

	// Pattern 2: Multiple dispatches with selective waiting
	fmt.Println("\n--- Pattern 2: Multiple Dispatches ---")
	fmt.Println("Dispatching multiple events...")
	h2 := evt.Dispatch(&UserCreatedEvent{})
	h3 := evt.Dispatch(&UserDeletedEvent{})

	fmt.Println("Waiting for h2...")
	h2.Wait()
	fmt.Println("✓ h2 completed")

	fmt.Println("Waiting for h3...")
	h3.Wait()
	fmt.Println("✓ h3 completed")

	// Check h3 errors (should have the simulated error)
	if errs := h3.GetErrors(); len(errs) > 0 {
		fmt.Printf("h3 had %d error(s):\n", len(errs))
		for _, err := range errs {
			fmt.Printf("  - %s\n", err)
		}
	}

	// Pattern 3: Fire and forget (handle discarded)
	fmt.Println("\n--- Pattern 3: Fire-and-Forget ---")
	fmt.Println("Dispatching without waiting (handle discarded):")
	evt.Dispatch(&UserCreatedEvent{})
	fmt.Println("Event dispatched, continuing without waiting...")

	// Pattern 4: Using Done() channel for non-blocking checks
	fmt.Println("\n--- Pattern 4: Non-Blocking Check with Done() ---")
	h4 := evt.Dispatch(&UserCreatedEvent{})

	select {
	case <-h4.Done():
		fmt.Println("✓ h4 completed immediately")
	case <-time.After(50 * time.Millisecond):
		fmt.Println("h4 still processing after 50ms...")
		<-h4.Done() // Wait for completion
		fmt.Println("✓ h4 completed")
	}

	// Pattern 5: Global wait (waits for all pending async handlers)
	fmt.Println("\n--- Pattern 5: Global Wait ---")
	evt.Dispatch(&UserCreatedEvent{})
	evt.Dispatch(&UserCreatedEvent{})
	fmt.Println("Dispatched 2 events, using global Wait()...")
	evt.Wait()
	fmt.Println("✓ All pending handlers completed")

	// Final error summary
	fmt.Println("\n--- Error Summary ---")
	allErrors := evt.GetErrors()
	fmt.Printf("Total errors across all dispatches: %d\n", len(allErrors))

	fmt.Println("\n=== Example Complete ===")
}
