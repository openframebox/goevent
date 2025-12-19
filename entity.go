package goevent

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
)

// EventError wraps errors that occur during event handling
type EventError struct {
	EventName    string
	ListenerType string
	Err          error
}

func (e *EventError) Error() string {
	return fmt.Sprintf("event '%s' listener '%s': %v", e.EventName, e.ListenerType, e.Err)
}

// DispatchHandle represents a handle to a specific event dispatch
// It allows waiting for and collecting errors from that specific dispatch
type DispatchHandle struct {
	id       string // Unique identifier for this dispatch
	isLocal  bool   // true for memory driver, false for Redis (affects Wait behavior)
	wg       sync.WaitGroup
	errorsMu sync.Mutex
	errors   []*EventError
	done     chan struct{}
}

// Wait blocks until all async handlers for this specific dispatch complete
func (dh *DispatchHandle) Wait() {
	dh.wg.Wait()
}

// Done returns a channel that closes when all handlers complete
// Useful for select statements
func (dh *DispatchHandle) Done() <-chan struct{} {
	return dh.done
}

// GetErrors returns errors that occurred during this specific dispatch
func (dh *DispatchHandle) GetErrors() []*EventError {
	dh.errorsMu.Lock()
	defer dh.errorsMu.Unlock()

	errorsCopy := make([]*EventError, len(dh.errors))
	copy(errorsCopy, dh.errors)
	return errorsCopy
}

// recordError stores an error for this specific dispatch
func (dh *DispatchHandle) recordError(err *EventError) {
	dh.errorsMu.Lock()
	defer dh.errorsMu.Unlock()
	dh.errors = append(dh.errors, err)
}

// markDone signals that all handlers have completed
func (dh *DispatchHandle) markDone() {
	close(dh.done)
}

// generateHandleID creates a unique identifier for a dispatch handle
func generateHandleID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		// Fallback to timestamp-based ID if random fails
		return fmt.Sprintf("%d", new(int))
	}
	return hex.EncodeToString(b)
}
