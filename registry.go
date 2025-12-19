package goevent

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sync"
)

// EventRegistry manages event type registration for deserialization
// Required for Redis driver to reconstruct typed events from JSON
type EventRegistry struct {
	types map[string]reflect.Type
	mu    sync.RWMutex
}

// globalRegistry is the default registry used by RegisterEventType
var globalRegistry = &EventRegistry{
	types: make(map[string]reflect.Type),
}

// RegisterEventType registers an event type for deserialization
// This must be called for all event types when using the Redis driver
// Typically called in init() functions
//
// The event.Name() is used as the registry key, enabling cross-service communication
// where different services can have the same logical event with different package names.
//
// Example:
//
//	func init() {
//	    goevent.RegisterEventType(&UserCreatedEvent{})
//	    // Registers with key from event.Name(), e.g., "user.created"
//	}
func RegisterEventType(event Event) {
	globalRegistry.RegisterAs(event.Name(), event)
}

// RegisterEventTypeAs registers an event type with a custom type name
// This allows explicit control over the registry key, useful for:
// - Event versioning (e.g., "order.created.v1")
// - Custom naming conventions
// - Migrating between naming schemes
//
// Example:
//
//	func init() {
//	    goevent.RegisterEventTypeAs("order.created.v1", &OrderCreatedEvent{})
//	}
func RegisterEventTypeAs(typeName string, event Event) {
	globalRegistry.RegisterAs(typeName, event)
}

// Register adds an event type to the registry using event.Name() as the key
// Deprecated: Use RegisterEventType() instead
func (er *EventRegistry) Register(event Event) {
	er.RegisterAs(event.Name(), event)
}

// RegisterAs adds an event type to the registry with a custom type name
func (er *EventRegistry) RegisterAs(typeName string, event Event) {
	er.mu.Lock()
	defer er.mu.Unlock()

	eventType := reflect.TypeOf(event)

	// If it's a pointer, get the element type
	if eventType.Kind() == reflect.Ptr {
		eventType = eventType.Elem()
	}

	er.types[typeName] = eventType
}

// Create creates a new event instance from a type name and payload
// Returns an error if the type is not registered
func (er *EventRegistry) Create(typeName string, payloadJSON json.RawMessage) (Event, error) {
	er.mu.RLock()
	eventType, exists := er.types[typeName]
	er.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf(
			"event type not registered: %s. "+
				"Did you forget to call goevent.RegisterEventType(&%s{}) in an init() function?",
			typeName, typeName,
		)
	}

	// Create a new instance (as pointer)
	eventValue := reflect.New(eventType)
	event := eventValue.Interface()

	// Try to unmarshal the entire event from JSON
	if err := json.Unmarshal(payloadJSON, event); err != nil {
		return nil, fmt.Errorf("failed to unmarshal event: %w", err)
	}

	// Verify it implements Event interface
	eventInterface, ok := event.(Event)
	if !ok {
		return nil, fmt.Errorf("type %s does not implement Event interface", typeName)
	}

	return eventInterface, nil
}

// IsRegistered checks if an event type is registered
func (er *EventRegistry) IsRegistered(typeName string) bool {
	er.mu.RLock()
	defer er.mu.RUnlock()
	_, exists := er.types[typeName]
	return exists
}
