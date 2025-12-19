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
🌐 **Distributed Support**: Redis driver for multi-process/multi-server event handling

## Installation

```bash
go get github.com/openframebox/goevent
```

## Drivers

GoEvent supports two drivers for different use cases:

### Memory Driver (Default)
The default in-memory driver uses EventBus for same-process communication:
- ✅ Zero configuration required
- ✅ High performance (no network overhead)
- ✅ Full DispatchHandle tracking
- ✅ Support for all Go types

### Redis Driver (Distributed)
For distributed event handling across multiple processes or servers:
- ✅ Multi-process/multi-server support
- ✅ Redis pub/sub for reliable delivery
- ✅ JSON serialization for cross-language compatibility
- ⚠️ Local-only DispatchHandle tracking
- ⚠️ Requires event type registration

**When to use each:**
- **Memory Driver**: Single-process applications, high-performance requirements, complex payloads
- **Redis Driver**: Microservices, distributed systems, worker pools, multi-server deployments

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

## Redis Driver Usage

### Basic Configuration

```go
package main

import (
    "fmt"
    "github.com/openframebox/goevent"
)

// 1. Register event types (required for Redis driver)
func init() {
    goevent.RegisterEventType(&UserRegisteredEvent{})
}

// 2. Define your event (must be JSON-serializable)
type UserRegisteredEvent struct {
    UserID string `json:"user_id"`
}

func (e *UserRegisteredEvent) Name() string {
    return "user.registered"
}

func (e *UserRegisteredEvent) Payload() map[string]any {
    return map[string]any{"user_id": e.UserID}
}

// 3. Configure Redis driver
func main() {
    evt := goevent.NewWithConfig(&goevent.Config{
        Driver: goevent.DriverRedis,
        Redis: &goevent.RedisConfig{
            Addr:     "localhost:6379",
            Password: "",  // Leave empty if no password
            DB:       0,
        },
    })
    defer evt.Close() // Always close to cleanup connections

    // Register listeners (same as memory driver)
    evt.RegisterListener(&EmailNotifier{})

    // Dispatch events (same API)
    handle := evt.Dispatch(&UserRegisteredEvent{UserID: "user123"})
    handle.Wait()

    fmt.Println("Done!")
}
```

### Multi-Process Example

**Process 1 (Publisher):**
```go
func main() {
    evt := goevent.NewWithConfig(&goevent.Config{
        Driver: goevent.DriverRedis,
        Redis:  &goevent.RedisConfig{Addr: "localhost:6379"},
    })
    defer evt.Close()

    // Publish events - will be received by all subscribers
    evt.Dispatch(&OrderCreatedEvent{OrderID: "123"})
}
```

**Process 2 (Subscriber):**
```go
func main() {
    evt := goevent.NewWithConfig(&goevent.Config{
        Driver: goevent.DriverRedis,
        Redis:  &goevent.RedisConfig{Addr: "localhost:6379"},
    })
    defer evt.Close()

    // Register listeners - will receive events from all publishers
    evt.RegisterListener(&OrderProcessor{})
    evt.RegisterListener(&EmailSender{})

    // Keep process running
    select {}
}
```

### Cross-Service Communication (Different Codebases)

The Redis driver supports **true cross-service communication** where different services can have their own event definitions:

**Service A (Order Service):**
```go
package main

import "github.com/openframebox/goevent"

type OrderCreatedEvent struct {
    OrderID string `json:"order_id"`
}

func (e *OrderCreatedEvent) Name() string {
    return "order.created"  // Key for cross-service compatibility
}

func init() {
    goevent.RegisterEventType(&OrderCreatedEvent{})
}

func main() {
    evt := goevent.NewWithConfig(&goevent.Config{
        Driver: goevent.DriverRedis,
        Redis:  &goevent.RedisConfig{Addr: "redis:6379"},
    })

    evt.Dispatch(&OrderCreatedEvent{OrderID: "123"})
}
```

**Service B (Email Service - completely different codebase):**
```go
package main

import "github.com/openframebox/goevent"

// Same logical event, different package, SAME Name()
type OrderCreatedEvent struct {
    OrderID string `json:"order_id"`
}

func (e *OrderCreatedEvent) Name() string {
    return "order.created"  // SAME name = cross-service compatible!
}

func init() {
    goevent.RegisterEventType(&OrderCreatedEvent{})
}

type EmailListener struct{}

func (l *EmailListener) EventName() string {
    return "order.created"
}

func (l *EmailListener) OnEvent(event goevent.Event) error {
    e := event.(*OrderCreatedEvent)  // ✅ Type-safe!
    sendEmail(e.OrderID)
    return nil
}
```

**How it works:** Both services register with `event.Name()` = `"order.created"`, so deserialization works across services even though they're different packages/codebases!

### Redis Configuration Options

```go
evt := goevent.NewWithConfig(&goevent.Config{
    Driver: goevent.DriverRedis,
    Redis: &goevent.RedisConfig{
        // Connection
        Addr:     "localhost:6379",
        Password: "your-password",
        DB:       0,

        // Pub/Sub
        ChannelPrefix: "myapp:",  // Prefix for Redis channels (default: "goevent:")

        // Performance
        MaxEventSize: 1024 * 1024,  // Max event size in bytes (default: 1MB)
        PoolSize:     10,           // Connection pool size (default: 10)
        MinIdleConns: 5,            // Min idle connections (default: 0)

        // Timeouts
        DialTimeout:  5 * time.Second,
        ReadTimeout:  3 * time.Second,
        WriteTimeout: 3 * time.Second,

        // TLS (optional)
        TLSConfig: &tls.Config{...},
    },
})
```

### Important: Event Registration

**All event types MUST be registered** when using the Redis driver:

```go
func init() {
    // Register all event types used in your application
    goevent.RegisterEventType(&UserCreatedEvent{})
    goevent.RegisterEventType(&OrderProcessedEvent{})
    goevent.RegisterEventType(&PaymentReceivedEvent{})
}
```

**How it works:** `RegisterEventType()` uses `event.Name()` as the registry key. This enables **cross-service communication** - different services can have the same logical event with different package names, as long as they return the same value from `Name()`.

**For custom type names or versioning:**
```go
func init() {
    // Use custom type name (e.g., for versioning)
    goevent.RegisterEventTypeAs("order.created.v1", &OrderCreatedEventV1{})
    goevent.RegisterEventTypeAs("order.created.v2", &OrderCreatedEventV2{})
}
```

Without registration, deserialization will fail with a clear error message.

### Redis Driver Limitations

1. **Local-Only DispatchHandle Tracking**
   - `handle.Wait()` only waits for LOCAL handlers in the current process
   - Remote handlers in other processes are not tracked
   - Use for synchronization within a process, not across processes

2. **Local-Only Error Collection**
   - `handle.GetErrors()` only returns errors from LOCAL handlers
   - Errors from remote processes are not collected
   - Use centralized logging for distributed error tracking

3. **JSON Serialization Requirements**
   - Event payloads must be JSON-serializable
   - Complex types (functions, channels, unexported fields) are not supported
   - Use struct tags for custom field names: `json:"field_name"`

4. **Network Latency**
   - Redis driver adds network overhead vs in-memory
   - Typical latency: 1-5ms depending on network
   - Use for distributed systems where latency is acceptable

### Switching Between Drivers

The API is identical for both drivers - just change the configuration:

```go
// Development: use memory driver
evt := goevent.New()

// Production: use Redis driver
evt := goevent.NewWithConfig(&goevent.Config{
    Driver: goevent.DriverRedis,
    Redis:  &goevent.RedisConfig{Addr: os.Getenv("REDIS_ADDR")},
})
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

### Unsubscribing Listeners

You can remove all listeners for a specific event at runtime using `UnregisterListenersForEvent()`:

```go
// Register listeners
evt.RegisterListener(&EmailListener{})
evt.RegisterListener(&SMSListener{})

// Dispatch events - both listeners handle them
evt.Dispatch(&UserCreatedEvent{})

// Later: disable all listeners for this event
evt.UnregisterListenersForEvent("user.created")

// Dispatch again - no listeners will handle it
evt.Dispatch(&UserCreatedEvent{})  // Nothing happens
```

**Common use cases:**
- **Feature Flags**: Dynamically enable/disable event-driven features
- **Maintenance Mode**: Temporarily disable certain handlers during maintenance
- **Testing**: Clean up listeners between test cases
- **Dynamic Configuration**: Enable/disable integrations at runtime

```go
// Example: Feature flag integration
func UpdateFeatureFlags(flags map[string]bool) {
    if !flags["email_notifications"] {
        evt.UnregisterListenersForEvent("user.created")
        evt.UnregisterListenersForEvent("order.created")
    }

    if !flags["analytics"] {
        evt.UnregisterListenersForEvent("page.viewed")
        evt.UnregisterListenersForEvent("button.clicked")
    }
}
```

**Important notes:**
- ✅ **Thread-safe**: Can be called while events are being dispatched
- ✅ **Idempotent**: Safe to call multiple times for the same event
- ✅ **Removes ALL listeners**: All listeners for the specified event are removed
- ✅ **Re-registerable**: You can register listeners again after unsubscribing

```go
// Idempotent - safe to call multiple times
evt.UnregisterListenersForEvent("user.created")
evt.UnregisterListenersForEvent("user.created")  // No error

// Non-existent events - no error
evt.UnregisterListenersForEvent("never.registered")  // No error

// Re-register after unsubscribe
evt.UnregisterListenersForEvent("user.created")
evt.RegisterListener(&NewEmailListener{})  // Works fine
evt.Dispatch(&UserCreatedEvent{})  // NewEmailListener handles it
```

**Works with both drivers:**
```go
// Memory driver
evt := goevent.New()
evt.RegisterListener(&Listener{})
evt.UnregisterListenersForEvent("my.event")

// Redis driver
evt := goevent.NewWithConfig(&goevent.Config{
    Driver: goevent.DriverRedis,
    Redis:  &goevent.RedisConfig{Addr: "localhost:6379"},
})
evt.RegisterListener(&Listener{})
evt.UnregisterListenersForEvent("my.event")  // Closes Redis subscription
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
// Constructors
func New() *GoEvent  // Creates with memory driver (backward compatible)
func NewWithConfig(cfg *Config) *GoEvent  // Creates with custom driver

// Core methods
func (ge *GoEvent) RegisterListener(listeners ...Listener)
func (ge *GoEvent) UnregisterListenersForEvent(eventName string) error  // Remove all listeners for an event
func (ge *GoEvent) Dispatch(event Event) *DispatchHandle
func (ge *GoEvent) Wait()
func (ge *GoEvent) GetErrors() []*EventError
func (ge *GoEvent) ClearErrors()
func (ge *GoEvent) Close() error  // Cleanup resources (important for Redis driver)
```

### Configuration Types

```go
type Config struct {
    Driver DriverType    // DriverMemory or DriverRedis
    Redis  *RedisConfig  // Required when Driver is DriverRedis
}

type DriverType string
const (
    DriverMemory DriverType = "memory"
    DriverRedis  DriverType = "redis"
)

type RedisConfig struct {
    // Connection
    Addr     string
    Password string
    DB       int

    // Pub/Sub
    ChannelPrefix string  // Default: "goevent:"

    // Performance
    MaxEventSize int      // Default: 1MB
    PoolSize     int      // Default: 10
    MinIdleConns int      // Default: 0

    // Timeouts
    DialTimeout  time.Duration  // Default: 5s
    ReadTimeout  time.Duration  // Default: 3s
    WriteTimeout time.Duration  // Default: 3s

    // TLS (optional)
    TLSConfig *tls.Config
}
```

### Event Registration (Redis Driver)

```go
// Register using event.Name() as key (recommended for cross-service)
func RegisterEventType(event Event)

// Register with custom type name (for versioning, custom names)
func RegisterEventTypeAs(typeName string, event Event)
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
