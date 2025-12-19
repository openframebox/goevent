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
// Example:
//
//	func init() {
//	    goevent.RegisterEventType(&UserCreatedEvent{})
//	}
func RegisterEventType(event Event) {
	globalRegistry.Register(event)
}

// Register adds an event type to the registry
func (er *EventRegistry) Register(event Event) {
	er.mu.Lock()
	defer er.mu.Unlock()

	// Use the type name as the key
	typeName := fmt.Sprintf("%T", event)
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
