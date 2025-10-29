package goevent

// Event represents an event that can be dispatched
type Event interface {
	Name() string
	Payload() map[string]any
}

// Listener represents a basic event listener
// By default, listeners execute synchronously
type Listener interface {
	EventName() string
	OnEvent(event Event) error
}

// ListenerOptions provides configuration for how a listener should execute
type ListenerOptions struct {
	// Async determines if the listener should execute asynchronously
	Async bool
}

// ListenerWithOptions represents a listener with custom execution options
type ListenerWithOptions interface {
	Listener
	Options() ListenerOptions
}
