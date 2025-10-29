package goevent

import (
	"errors"
	"testing"
	"time"
)

// Test event types
type TestEvent struct {
	data string
}

func (e *TestEvent) Name() string {
	return "test.event"
}

func (e *TestEvent) Payload() map[string]any {
	return map[string]any{"data": e.data}
}

// Test listener types
type testSyncListener struct {
	called bool
	data   string
}

func (l *testSyncListener) EventName() string {
	return "test.event"
}

func (l *testSyncListener) OnEvent(event Event) error {
	l.called = true
	if e, ok := event.(*TestEvent); ok {
		l.data = e.data
	}
	return nil
}

type testAsyncListener struct {
	called bool
	data   string
}

func (l *testAsyncListener) EventName() string {
	return "test.event"
}

func (l *testAsyncListener) OnEvent(event Event) error {
	time.Sleep(10 * time.Millisecond) // Simulate work
	l.called = true
	if e, ok := event.(*TestEvent); ok {
		l.data = e.data
	}
	return nil
}

func (l *testAsyncListener) Options() ListenerOptions {
	return ListenerOptions{Async: true}
}

type testErrorListener struct {
	called bool
}

func (l *testErrorListener) EventName() string {
	return "test.event"
}

func (l *testErrorListener) OnEvent(event Event) error {
	l.called = true
	return errors.New("test error")
}

func TestNew(t *testing.T) {
	evt := New()
	if evt == nil {
		t.Fatal("New() returned nil")
	}
	if evt.bus == nil {
		t.Error("EventBus not initialized")
	}
	if evt.errors == nil {
		t.Error("Errors slice not initialized")
	}
	if evt.asyncListeners == nil {
		t.Error("Async listeners map not initialized")
	}
}

func TestSyncListener(t *testing.T) {
	evt := New()
	listener := &testSyncListener{}

	evt.RegisterListener(listener)
	handle := evt.Dispatch(&TestEvent{data: "sync test"})

	if !listener.called {
		t.Error("Sync listener was not called")
	}

	if listener.data != "sync test" {
		t.Errorf("Expected data 'sync test', got '%s'", listener.data)
	}

	handle.Wait() // Should return immediately for sync listeners
}

func TestAsyncListener(t *testing.T) {
	evt := New()
	listener := &testAsyncListener{}

	evt.RegisterListener(listener)
	handle := evt.Dispatch(&TestEvent{data: "async test"})

	// Listener may not be called immediately (async)
	handle.Wait()

	if !listener.called {
		t.Error("Async listener was not called")
	}

	if listener.data != "async test" {
		t.Errorf("Expected data 'async test', got '%s'", listener.data)
	}
}

func TestMultipleListeners(t *testing.T) {
	evt := New()
	syncListener := &testSyncListener{}
	asyncListener := &testAsyncListener{}

	evt.RegisterListener(syncListener, asyncListener)
	handle := evt.Dispatch(&TestEvent{data: "multi test"})
	handle.Wait()

	if !syncListener.called {
		t.Error("Sync listener was not called")
	}

	if !asyncListener.called {
		t.Error("Async listener was not called")
	}
}

func TestErrorCollection_PerDispatch(t *testing.T) {
	evt := New()
	errorListener := &testErrorListener{}

	evt.RegisterListener(errorListener)
	handle := evt.Dispatch(&TestEvent{data: "error test"})

	if !errorListener.called {
		t.Error("Error listener was not called")
	}

	errs := handle.GetErrors()
	if len(errs) != 1 {
		t.Fatalf("Expected 1 error, got %d", len(errs))
	}

	if errs[0].Err.Error() != "test error" {
		t.Errorf("Expected error 'test error', got '%s'", errs[0].Err.Error())
	}
}

func TestErrorCollection_Global(t *testing.T) {
	evt := New()
	errorListener := &testErrorListener{}

	evt.RegisterListener(errorListener)
	evt.Dispatch(&TestEvent{data: "error test 1"})
	evt.Dispatch(&TestEvent{data: "error test 2"})

	allErrs := evt.GetErrors()
	if len(allErrs) != 2 {
		t.Fatalf("Expected 2 errors, got %d", len(allErrs))
	}

	evt.ClearErrors()
	allErrs = evt.GetErrors()
	if len(allErrs) != 0 {
		t.Errorf("Expected 0 errors after ClearErrors(), got %d", len(allErrs))
	}
}

func TestDispatchHandle_Done(t *testing.T) {
	evt := New()
	asyncListener := &testAsyncListener{}

	evt.RegisterListener(asyncListener)
	handle := evt.Dispatch(&TestEvent{data: "done test"})

	select {
	case <-handle.Done():
		// Success
	case <-time.After(1 * time.Second):
		t.Error("Done() channel did not close within timeout")
	}

	if !asyncListener.called {
		t.Error("Async listener was not called")
	}
}

func TestGlobalWait(t *testing.T) {
	evt := New()
	listener1 := &testAsyncListener{}
	listener2 := &testAsyncListener{}

	evt.RegisterListener(listener1)
	evt.Dispatch(&TestEvent{data: "wait test 1"})

	evt.RegisterListener(listener2)
	evt.Dispatch(&TestEvent{data: "wait test 2"})

	evt.Wait()

	if !listener1.called {
		t.Error("First listener was not called")
	}

	if !listener2.called {
		t.Error("Second listener was not called")
	}
}

func TestEventPayload(t *testing.T) {
	evt := New()

	var receivedPayload map[string]any
	testListener := &testSyncListener{}

	evt.RegisterListener(testListener)
	event := &TestEvent{data: "payload test"}
	evt.Dispatch(event)

	receivedPayload = event.Payload()

	if receivedPayload["data"] != "payload test" {
		t.Errorf("Expected payload data 'payload test', got '%v'", receivedPayload["data"])
	}
}

func BenchmarkSyncDispatch(b *testing.B) {
	evt := New()
	listener := &testSyncListener{}
	evt.RegisterListener(listener)
	event := &TestEvent{data: "benchmark"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		evt.Dispatch(event)
	}
}

func BenchmarkAsyncDispatch(b *testing.B) {
	evt := New()
	listener := &testAsyncListener{}
	evt.RegisterListener(listener)
	event := &TestEvent{data: "benchmark"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		handle := evt.Dispatch(event)
		handle.Wait()
	}
}
