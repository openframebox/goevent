package goevent

import (
	"errors"
	"testing"
	"time"
)

func TestMemoryDriver_BasicPublishSubscribe(t *testing.T) {
	driver := newMemoryDriver()

	called := false
	handler := func(handle *DispatchHandle, event Event) {
		called = true
		if event.Name() != "test.event" {
			t.Errorf("Expected event name 'test.event', got '%s'", event.Name())
		}
	}

	// Subscribe
	if err := driver.Subscribe("test.event", handler, false); err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}

	// Create test event
	testEvent := &testEvent{name: "test.event", payload: map[string]any{"key": "value"}}
	handle := &DispatchHandle{
		id:      generateHandleID(),
		isLocal: true,
		errors:  make([]*EventError, 0),
		done:    make(chan struct{}),
	}

	// Publish
	if err := driver.Publish("test.event", handle, testEvent); err != nil {
		t.Fatalf("Publish failed: %v", err)
	}

	// Allow event to propagate (EventBus is sync for non-async handlers)
	time.Sleep(10 * time.Millisecond)

	if !called {
		t.Error("Handler was not called")
	}
}

func TestMemoryDriver_AsyncHandler(t *testing.T) {
	driver := newMemoryDriver()

	called := false
	var wg mockWaitGroup

	handler := func(handle *DispatchHandle, event Event) {
		defer wg.Done()
		time.Sleep(50 * time.Millisecond) // Simulate async work
		called = true
	}

	// Subscribe async
	if err := driver.Subscribe("test.async", handler, true); err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}

	// Create test event
	testEvent := &testEvent{name: "test.async", payload: map[string]any{}}
	handle := &DispatchHandle{
		id:      generateHandleID(),
		isLocal: true,
		errors:  make([]*EventError, 0),
		done:    make(chan struct{}),
	}

	// Add to wait group before publishing
	wg.Add(1)

	// Publish
	if err := driver.Publish("test.async", handle, testEvent); err != nil {
		t.Fatalf("Publish failed: %v", err)
	}

	// Wait for async handler
	wg.Wait()

	if !called {
		t.Error("Async handler was not called")
	}
}

func TestMemoryDriver_MultipleHandlers(t *testing.T) {
	driver := newMemoryDriver()

	call1 := false
	call2 := false

	handler1 := func(handle *DispatchHandle, event Event) {
		call1 = true
	}

	handler2 := func(handle *DispatchHandle, event Event) {
		call2 = true
	}

	// Subscribe both handlers
	driver.Subscribe("multi.event", handler1, false)
	driver.Subscribe("multi.event", handler2, false)

	// Create test event
	testEvent := &testEvent{name: "multi.event", payload: map[string]any{}}
	handle := &DispatchHandle{
		id:      generateHandleID(),
		isLocal: true,
		errors:  make([]*EventError, 0),
		done:    make(chan struct{}),
	}

	// Publish
	driver.Publish("multi.event", handle, testEvent)

	time.Sleep(10 * time.Millisecond)

	if !call1 || !call2 {
		t.Error("Not all handlers were called")
	}
}

func TestMemoryDriver_Close(t *testing.T) {
	driver := newMemoryDriver()

	if err := driver.Close(); err != nil {
		t.Errorf("Close returned error: %v", err)
	}
}

// Mock wait group for testing
type mockWaitGroup struct {
	count int
	done  chan struct{}
}

func (m *mockWaitGroup) Add(delta int) {
	m.count += delta
	if m.done == nil {
		m.done = make(chan struct{})
	}
}

func (m *mockWaitGroup) Done() {
	m.count--
	if m.count == 0 && m.done != nil {
		close(m.done)
	}
}

func (m *mockWaitGroup) Wait() {
	if m.done != nil {
		<-m.done
	}
}

// Test event type
type testEvent struct {
	name    string
	payload map[string]any
}

func (e *testEvent) Name() string {
	return e.name
}

func (e *testEvent) Payload() map[string]any {
	return e.payload
}

// Test error handler
func TestMemoryDriver_ErrorInHandler(t *testing.T) {
	driver := newMemoryDriver()

	testErr := errors.New("test error")
	handler := func(handle *DispatchHandle, event Event) {
		// Simulate error by recording it to the handle
		handle.recordError(&EventError{
			EventName:    event.Name(),
			ListenerType: "test",
			Err:          testErr,
		})
	}

	driver.Subscribe("error.event", handler, false)

	testEvent := &testEvent{name: "error.event", payload: map[string]any{}}
	handle := &DispatchHandle{
		id:      generateHandleID(),
		isLocal: true,
		errors:  make([]*EventError, 0),
		done:    make(chan struct{}),
	}

	driver.Publish("error.event", handle, testEvent)

	time.Sleep(10 * time.Millisecond)

	errs := handle.GetErrors()
	if len(errs) != 1 {
		t.Fatalf("Expected 1 error, got %d", len(errs))
	}

	if errs[0].Err != testErr {
		t.Errorf("Expected error '%v', got '%v'", testErr, errs[0].Err)
	}
}

func TestMemoryDriver_Unsubscribe(t *testing.T) {
	driver := newMemoryDriver()

	called := false
	handler := func(handle *DispatchHandle, event Event) {
		called = true
	}

	// Subscribe
	if err := driver.Subscribe("test.event", handler, false); err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}

	// Create test event
	testEvent := &testEvent{name: "test.event", payload: map[string]any{}}
	handle := &DispatchHandle{
		id:      generateHandleID(),
		isLocal: true,
		errors:  make([]*EventError, 0),
		done:    make(chan struct{}),
	}

	// Publish - should be called
	if err := driver.Publish("test.event", handle, testEvent); err != nil {
		t.Fatalf("Publish failed: %v", err)
	}
	time.Sleep(10 * time.Millisecond)

	if !called {
		t.Error("Handler was not called before unsubscribe")
	}

	// Unsubscribe
	if err := driver.Unsubscribe("test.event"); err != nil {
		t.Fatalf("Unsubscribe failed: %v", err)
	}

	// Publish again - should NOT be called
	called = false
	handle2 := &DispatchHandle{
		id:      generateHandleID(),
		isLocal: true,
		errors:  make([]*EventError, 0),
		done:    make(chan struct{}),
	}
	if err := driver.Publish("test.event", handle2, testEvent); err != nil {
		t.Fatalf("Publish failed: %v", err)
	}
	time.Sleep(10 * time.Millisecond)

	if called {
		t.Error("Handler was called after unsubscribe")
	}
}

func TestMemoryDriver_UnsubscribeNonExistent(t *testing.T) {
	driver := newMemoryDriver()

	// Unsubscribe event that was never subscribed - should be idempotent
	if err := driver.Unsubscribe("never.subscribed"); err != nil {
		t.Errorf("Unsubscribe of non-existent event returned error: %v", err)
	}
}

func TestMemoryDriver_UnsubscribeMultipleHandlers(t *testing.T) {
	driver := newMemoryDriver()

	call1 := false
	call2 := false

	handler1 := func(handle *DispatchHandle, event Event) {
		call1 = true
	}

	handler2 := func(handle *DispatchHandle, event Event) {
		call2 = true
	}

	// Subscribe both handlers
	driver.Subscribe("multi.event", handler1, false)
	driver.Subscribe("multi.event", handler2, false)

	// Create test event
	testEvent := &testEvent{name: "multi.event", payload: map[string]any{}}
	handle := &DispatchHandle{
		id:      generateHandleID(),
		isLocal: true,
		errors:  make([]*EventError, 0),
		done:    make(chan struct{}),
	}

	// Publish - both should be called
	driver.Publish("multi.event", handle, testEvent)
	time.Sleep(10 * time.Millisecond)

	if !call1 || !call2 {
		t.Error("Not all handlers were called before unsubscribe")
	}

	// Unsubscribe
	if err := driver.Unsubscribe("multi.event"); err != nil {
		t.Fatalf("Unsubscribe failed: %v", err)
	}

	// Publish again - neither should be called
	call1 = false
	call2 = false
	handle2 := &DispatchHandle{
		id:      generateHandleID(),
		isLocal: true,
		errors:  make([]*EventError, 0),
		done:    make(chan struct{}),
	}
	driver.Publish("multi.event", handle2, testEvent)
	time.Sleep(10 * time.Millisecond)

	if call1 || call2 {
		t.Error("Handlers were called after unsubscribe")
	}
}

func TestMemoryDriver_UnsubscribeAsync(t *testing.T) {
	driver := newMemoryDriver()

	called := false
	var wg mockWaitGroup

	handler := func(handle *DispatchHandle, event Event) {
		defer wg.Done()
		time.Sleep(20 * time.Millisecond)
		called = true
	}

	// Subscribe async
	if err := driver.Subscribe("test.async", handler, true); err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}

	// Create test event
	testEvent := &testEvent{name: "test.async", payload: map[string]any{}}
	handle := &DispatchHandle{
		id:      generateHandleID(),
		isLocal: true,
		errors:  make([]*EventError, 0),
		done:    make(chan struct{}),
	}

	// Publish - should be called
	wg.Add(1)
	if err := driver.Publish("test.async", handle, testEvent); err != nil {
		t.Fatalf("Publish failed: %v", err)
	}
	wg.Wait()

	if !called {
		t.Error("Async handler was not called before unsubscribe")
	}

	// Unsubscribe
	if err := driver.Unsubscribe("test.async"); err != nil {
		t.Fatalf("Unsubscribe failed: %v", err)
	}

	// Publish again - should NOT be called
	called = false
	handle2 := &DispatchHandle{
		id:      generateHandleID(),
		isLocal: true,
		errors:  make([]*EventError, 0),
		done:    make(chan struct{}),
	}
	if err := driver.Publish("test.async", handle2, testEvent); err != nil {
		t.Fatalf("Publish failed: %v", err)
	}
	time.Sleep(50 * time.Millisecond)

	if called {
		t.Error("Async handler was called after unsubscribe")
	}
}
