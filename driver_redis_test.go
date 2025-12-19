package goevent

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
)

// TestRedisEvent is a test event type for Redis testing
type TestRedisEvent struct {
	EventName string         `json:"event_name"`
	Data      map[string]any `json:"data"`
}

func (e *TestRedisEvent) Name() string {
	return e.EventName
}

func (e *TestRedisEvent) Payload() map[string]any {
	return e.Data
}

func init() {
	// Register test event types
	RegisterEventType(&TestRedisEvent{})
}

func TestRedisDriver_Connection(t *testing.T) {
	mr := miniredis.RunT(t)
	defer mr.Close()

	config := &RedisConfig{
		Addr: mr.Addr(),
	}

	driver, err := newRedisDriver(config)
	if err != nil {
		t.Fatalf("Failed to create Redis driver: %v", err)
	}
	defer driver.Close()

	if driver == nil {
		t.Fatal("Driver is nil")
	}
}

func TestRedisDriver_PublishSubscribe(t *testing.T) {
	mr := miniredis.RunT(t)
	defer mr.Close()

	config := &RedisConfig{
		Addr: mr.Addr(),
	}

	driver, err := newRedisDriver(config)
	if err != nil {
		t.Fatalf("Failed to create Redis driver: %v", err)
	}
	defer driver.Close()

	// Set up subscriber
	called := false
	handler := func(handle *DispatchHandle, event Event) {
		called = true
		if event.Name() != "test.redis" {
			t.Errorf("Expected event name 'test.redis', got '%s'", event.Name())
		}
		redisEvent, ok := event.(*TestRedisEvent)
		if !ok {
			t.Errorf("Expected *TestRedisEvent, got %T", event)
		}
		if redisEvent.Data["key"] != "value" {
			t.Errorf("Expected data key='value', got %v", redisEvent.Data["key"])
		}
	}

	// Subscribe
	if err := driver.Subscribe("test.redis", handler, false); err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}

	// Wait for subscription to be ready
	time.Sleep(100 * time.Millisecond)

	// Create and publish event
	testEvent := &TestRedisEvent{
		EventName: "test.redis",
		Data:      map[string]any{"key": "value"},
	}
	handle := &DispatchHandle{
		id:      generateHandleID(),
		isLocal: false,
		errors:  make([]*EventError, 0),
		done:    make(chan struct{}),
	}

	if err := driver.Publish("test.redis", handle, testEvent); err != nil {
		t.Fatalf("Publish failed: %v", err)
	}

	// Wait for message to be processed
	time.Sleep(100 * time.Millisecond)

	if !called {
		t.Error("Handler was not called")
	}
}

func TestRedisDriver_AsyncHandler(t *testing.T) {
	mr := miniredis.RunT(t)
	defer mr.Close()

	config := &RedisConfig{
		Addr: mr.Addr(),
	}

	driver, err := newRedisDriver(config)
	if err != nil {
		t.Fatalf("Failed to create Redis driver: %v", err)
	}
	defer driver.Close()

	called := false
	done := make(chan struct{})

	handler := func(handle *DispatchHandle, event Event) {
		defer close(done)
		time.Sleep(50 * time.Millisecond)
		called = true
	}

	// Subscribe async
	driver.Subscribe("test.async", handler, true)

	time.Sleep(100 * time.Millisecond)

	// Publish
	testEvent := &TestRedisEvent{
		EventName: "test.async",
		Data:      map[string]any{},
	}
	handle := &DispatchHandle{
		id:      generateHandleID(),
		isLocal: false,
		errors:  make([]*EventError, 0),
		done:    make(chan struct{}),
	}

	driver.Publish("test.async", handle, testEvent)

	// Wait for async handler
	select {
	case <-done:
		// Success
	case <-time.After(2 * time.Second):
		t.Fatal("Async handler timed out")
	}

	if !called {
		t.Error("Async handler was not called")
	}
}

func TestRedisDriver_MultipleHandlers(t *testing.T) {
	mr := miniredis.RunT(t)
	defer mr.Close()

	config := &RedisConfig{
		Addr: mr.Addr(),
	}

	driver, err := newRedisDriver(config)
	if err != nil {
		t.Fatalf("Failed to create Redis driver: %v", err)
	}
	defer driver.Close()

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

	time.Sleep(100 * time.Millisecond)

	// Publish
	testEvent := &TestRedisEvent{
		EventName: "multi.event",
		Data:      map[string]any{},
	}
	handle := &DispatchHandle{
		id:      generateHandleID(),
		isLocal: false,
		errors:  make([]*EventError, 0),
		done:    make(chan struct{}),
	}

	driver.Publish("multi.event", handle, testEvent)

	time.Sleep(100 * time.Millisecond)

	if !call1 || !call2 {
		t.Error("Not all handlers were called")
	}
}

func TestRedisDriver_SerializationError(t *testing.T) {
	mr := miniredis.RunT(t)
	defer mr.Close()

	config := &RedisConfig{
		Addr: mr.Addr(),
	}

	driver, err := newRedisDriver(config)
	if err != nil {
		t.Fatalf("Failed to create Redis driver: %v", err)
	}
	defer driver.Close()

	// Create event with invalid data (channels can't be JSON serialized)
	invalidEvent := &invalidEvent{}
	handle := &DispatchHandle{
		id:      generateHandleID(),
		isLocal: false,
		errors:  make([]*EventError, 0),
		done:    make(chan struct{}),
	}

	err = driver.Publish("test.invalid", handle, invalidEvent)
	if err == nil {
		t.Error("Expected error for invalid event, got nil")
	}
}

func TestRedisDriver_MaxEventSize(t *testing.T) {
	mr := miniredis.RunT(t)
	defer mr.Close()

	config := &RedisConfig{
		Addr:         mr.Addr(),
		MaxEventSize: 100, // Very small limit
	}

	driver, err := newRedisDriver(config)
	if err != nil {
		t.Fatalf("Failed to create Redis driver: %v", err)
	}
	defer driver.Close()

	// Create large event
	largeData := make(map[string]any)
	for i := 0; i < 100; i++ {
		largeData[string(rune('a'+i))] = "very long string value that will exceed the size limit"
	}

	testEvent := &TestRedisEvent{
		EventName: "test.large",
		Data:      largeData,
	}
	handle := &DispatchHandle{
		id:      generateHandleID(),
		isLocal: false,
		errors:  make([]*EventError, 0),
		done:    make(chan struct{}),
	}

	err = driver.Publish("test.large", handle, testEvent)
	if err == nil {
		t.Error("Expected error for oversized event, got nil")
	}
}

func TestRedisDriver_UnregisteredEventType(t *testing.T) {
	mr := miniredis.RunT(t)
	defer mr.Close()

	config := &RedisConfig{
		Addr: mr.Addr(),
	}

	driver, err := newRedisDriver(config)
	if err != nil {
		t.Fatalf("Failed to create Redis driver: %v", err)
	}
	defer driver.Close()

	// Subscribe to event
	errorLogged := false
	handler := func(handle *DispatchHandle, event Event) {
		// This should not be called
		t.Error("Handler should not be called for unregistered event type")
	}

	driver.Subscribe("test.unregistered", handler, false)
	time.Sleep(100 * time.Millisecond)

	// Manually publish a message with unregistered type
	redisDriver := driver.(*redisDriver)
	msg := redisMessage{
		EventName: "test.unregistered",
		EventType: "*goevent.UnregisteredEvent", // This type is not registered
		EventData: json.RawMessage(`{"name":"test"}`),
		HandleID:  generateHandleID(),
		Timestamp: time.Now().Unix(),
	}

	data, _ := json.Marshal(msg)
	channel := redisDriver.channelName("test.unregistered")
	redisDriver.pubClient.Publish(redisDriver.ctx, channel, data)

	// Wait for message processing
	time.Sleep(100 * time.Millisecond)

	// The error should be logged (we'd need to capture logs to verify)
	// For now, just verify handler wasn't called
	_ = errorLogged // Placeholder
}

func TestRedisDriver_Close(t *testing.T) {
	mr := miniredis.RunT(t)
	defer mr.Close()

	config := &RedisConfig{
		Addr: mr.Addr(),
	}

	driver, err := newRedisDriver(config)
	if err != nil {
		t.Fatalf("Failed to create Redis driver: %v", err)
	}

	// Subscribe to event
	driver.Subscribe("test.close", func(handle *DispatchHandle, event Event) {}, false)

	// Close driver
	if err := driver.Close(); err != nil {
		t.Errorf("Close returned error: %v", err)
	}

	// Verify we can't publish after close
	testEvent := &TestRedisEvent{
		EventName: "test.close",
		Data:      map[string]any{},
	}
	handle := &DispatchHandle{
		id:      generateHandleID(),
		isLocal: false,
		errors:  make([]*EventError, 0),
		done:    make(chan struct{}),
	}

	err = driver.Publish("test.close", handle, testEvent)
	if err == nil {
		t.Error("Expected error when publishing after close")
	}
}

// invalidEvent is an event type that cannot be JSON serialized
type invalidEvent struct {
	Ch chan struct{} // Channels cannot be JSON serialized
}

func (e *invalidEvent) Name() string {
	return "invalid"
}

func (e *invalidEvent) Payload() map[string]any {
	return map[string]any{"ch": e.Ch}
}
