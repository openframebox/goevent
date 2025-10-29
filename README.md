# GoEvent

[![Go Version](https://img.shields.io/badge/go-%3E%3D1.21-blue.svg)](https://golang.org/doc/devel/release.html)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

A **type-safe, flexible event bus** for Go, providing an elegant wrapper around [EventBus](https://github.com/asaskevich/EventBus) with enhanced error handling, synchronization, and per-event waiting capabilities.

## Features

🔒 **Type-Safe**: Interface-based design eliminates reflection in your code
⚡ **Sync/Async Flexibility**: Choose execution mode per listener
🎯 **Per-Event Waiting**: Fine-grained control with `DispatchHandle`
🚨 **Error Collection**: Built-in error tracking and reporting
🔧 **Simple API**: Minimal boilerplate, maximum flexibility
✅ **Production Ready**: Thread-safe with proper synchronization

## Installation

```bash
go get github.com/openframebox/goevent
```

## Quick Start

```go
package main

import (
    "fmt"
    "github.com/openframebox/goevent"
)

// 1. Define your event
type UserRegisteredEvent struct {
    UserID string
}

func (e *UserRegisteredEvent) Name() string {
    return "user.registered"
}

func (e *UserRegisteredEvent) Payload() map[string]any {
    return map[string]any{"user_id": e.UserID}
}

// 2. Define a listener
type EmailNotifier struct{}

func (l *EmailNotifier) EventName() string {
    return "user.registered"
}

func (l *EmailNotifier) OnEvent(event goevent.Event) error {
    e := event.(*UserRegisteredEvent)
    fmt.Printf("Sending email to user: %s\n", e.UserID)
    return nil
}

// 3. Initialize and use
func main() {
    evt := goevent.New()
    evt.RegisterListener(&EmailNotifier{})

    handle := evt.Dispatch(&UserRegisteredEvent{UserID: "user123"})
    handle.Wait() // Wait for completion

    fmt.Println("Done!")
}
```

## Usage Examples

### Synchronous vs Asynchronous Listeners

By default, listeners execute **synchronously**. To make a listener async, implement the `ListenerWithOptions` interface:

```go
// Synchronous listener (default)
type SyncListener struct{}

func (l *SyncListener) EventName() string {
    return "my.event"
}

func (l *SyncListener) OnEvent(event goevent.Event) error {
    // Executes immediately in the same goroutine
    return nil
}

// Asynchronous listener
type AsyncListener struct{}

func (l *AsyncListener) EventName() string {
    return "my.event"
}

func (l *AsyncListener) OnEvent(event goevent.Event) error {
    // Executes in a separate goroutine
    return nil
}

// This makes it async!
func (l *AsyncListener) Options() goevent.ListenerOptions {
    return goevent.ListenerOptions{Async: true}
}
```

### Per-Event Waiting with DispatchHandle

Each `Dispatch()` returns a handle for fine-grained control:

```go
// Wait for a specific event
handle := evt.Dispatch(&CriticalEvent{})
handle.Wait() // Blocks until this event's handlers complete

// Check errors for this specific dispatch
if errs := handle.GetErrors(); len(errs) > 0 {
    log.Printf("Errors occurred: %v", errs)
}

// Non-blocking check with Done() channel
handle := evt.Dispatch(&Event{})
select {
case <-handle.Done():
    fmt.Println("Completed!")
case <-time.After(timeout):
    fmt.Println("Timeout!")
}
```

### Fire-and-Forget Pattern

For non-critical events, simply discard the handle:

```go
// Dispatch and continue immediately
evt.Dispatch(&AnalyticsEvent{})
evt.Dispatch(&LogEvent{})
// Handlers run in background, no waiting

// At shutdown, wait for all remaining handlers
defer evt.Wait()
```

### Error Handling

```go
// Per-dispatch errors
handle := evt.Dispatch(&Event{})
handle.Wait()
for _, err := range handle.GetErrors() {
    log.Printf("Handler error: %s", err)
}

// Global error collection
evt.Dispatch(&Event1{})
evt.Dispatch(&Event2{})
evt.Wait()

// Get all errors across all dispatches
allErrors := evt.GetErrors()
fmt.Printf("Total errors: %d\n", len(allErrors))

// Clear errors
evt.ClearErrors()
```

### Hybrid Pattern (Recommended)

Combine per-event and global waiting for maximum flexibility:

```go
func ProcessOrder(orderID string) error {
    // Critical: Must complete before continuing
    handle := evt.Dispatch(&ProcessPaymentEvent{OrderID: orderID})
    handle.Wait()

    if errs := handle.GetErrors(); len(errs) > 0 {
        return fmt.Errorf("payment failed: %v", errs[0])
    }

    // Non-critical: Fire and forget
    evt.Dispatch(&SendReceiptEmail{OrderID: orderID})
    evt.Dispatch(&UpdateAnalytics{OrderID: orderID})

    return nil
}

func main() {
    defer evt.Wait() // Catch any remaining async handlers at shutdown

    // Your application logic...
}
```

### Using Event Payloads

Access event data through the `Payload()` method:

```go
type OrderCreatedEvent struct {
    OrderID string
    Amount  float64
}

func (e *OrderCreatedEvent) Name() string {
    return "order.created"
}

func (e *OrderCreatedEvent) Payload() map[string]any {
    return map[string]any{
        "order_id": e.OrderID,
        "amount":   e.Amount,
    }
}

// In your listener
func (l *Listener) OnEvent(event goevent.Event) error {
    payload := event.Payload()
    orderID := payload["order_id"].(string)
    amount := payload["amount"].(float64)

    // Or use type assertion
    if e, ok := event.(*OrderCreatedEvent); ok {
        fmt.Printf("Order %s: $%.2f\n", e.OrderID, e.Amount)
    }

    return nil
}
```

## API Reference

### Core Types

```go
type Event interface {
    Name() string
    Payload() map[string]any
}

type Listener interface {
    EventName() string
    OnEvent(event Event) error
}

type ListenerWithOptions interface {
    Listener
    Options() ListenerOptions
}

type ListenerOptions struct {
    Async bool  // Execute asynchronously if true
}
```

### GoEvent Methods

```go
func New() *GoEvent
func (ge *GoEvent) RegisterListener(listeners ...Listener)
func (ge *GoEvent) Dispatch(event Event) *DispatchHandle
func (ge *GoEvent) Wait()
func (ge *GoEvent) GetErrors() []*EventError
func (ge *GoEvent) ClearErrors()
```

### DispatchHandle Methods

```go
func (dh *DispatchHandle) Wait()
func (dh *DispatchHandle) Done() <-chan struct{}
func (dh *DispatchHandle) GetErrors() []*EventError
```

## Real-World Example

```go
package main

import (
    "log"
    "os"
    "os/signal"
    "syscall"

    "github.com/openframebox/goevent"
)

// Global event bus
var Evt = goevent.New()

func init() {
    // Register all listeners at startup
    Evt.RegisterListener(
        &PaymentProcessor{},
        &EmailSender{},
        &AnalyticsTracker{},
    )
}

func main() {
    // Graceful shutdown
    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

    go func() {
        <-sigChan
        log.Println("Shutting down gracefully...")
        Evt.Wait() // Wait for all pending event handlers
        os.Exit(0)
    }()

    // Your application logic...
    ProcessOrder("order-123")

    // Keep running
    select {}
}

func ProcessOrder(orderID string) error {
    // Critical event - must wait
    handle := Evt.Dispatch(&PaymentEvent{OrderID: orderID})
    handle.Wait()

    if errs := handle.GetErrors(); len(errs) > 0 {
        return errs[0].Err
    }

    // Non-critical events - fire and forget
    Evt.Dispatch(&EmailEvent{OrderID: orderID})
    Evt.Dispatch(&AnalyticsEvent{OrderID: orderID})

    return nil
}
```

## Why GoEvent?

### vs. EventBus (underlying library)
- ✅ Type-safe interfaces instead of reflection
- ✅ Built-in error collection and reporting
- ✅ Per-event waiting and tracking
- ✅ Simplified async/sync configuration

### vs. Channels
- ✅ Multiple listeners per event automatically
- ✅ No manual goroutine management
- ✅ Built-in error handling
- ✅ More declarative code

## Contributing

Contributions are welcome! Please read [CONTRIBUTING.md](CONTRIBUTING.md) for details on our code of conduct and the process for submitting pull requests.

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Acknowledgments

Built on top of [asaskevich/EventBus](https://github.com/asaskevich/EventBus)
