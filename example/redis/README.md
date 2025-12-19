# GoEvent Redis Example

This example demonstrates distributed event handling using the Redis driver.

## Prerequisites

- Redis server running on `localhost:6379` (or set `REDIS_ADDR` environment variable)
- Go 1.21 or higher

## Running the Example

### Option 1: Single Process (Publisher + Subscriber)

Run a single process that both publishes and subscribes:

```bash
go run main.go both
```

or simply:

```bash
go run main.go
```

### Option 2: Multiple Processes (Distributed)

This demonstrates the real power of the Redis driver - events published by one process are received by listeners in other processes.

**Terminal 1 (Subscriber):**
```bash
go run main.go subscriber
```

**Terminal 2 (Another Subscriber):**
```bash
go run main.go subscriber
```

**Terminal 3 (Publisher):**
```bash
go run main.go publisher
```

You'll see both subscriber terminals receive and process the events published by the publisher!

## Custom Redis Address

Set the `REDIS_ADDR` environment variable to use a different Redis server:

```bash
REDIS_ADDR="redis-server:6379" go run main.go
```

## What This Example Shows

1. **Event Registration**: How to register event types for Redis deserialization
2. **Multi-Process Communication**: Events published in one process are received by listeners in other processes
3. **Async Listeners**: EmailSender runs asynchronously using `ListenerOptions`
4. **Error Handling**: Demonstrates error collection
5. **Graceful Shutdown**: Proper cleanup with `evt.Close()` and `evt.Wait()`

## Expected Output

### Subscriber Terminal:
```
📥 Registering event listeners...
✅ Listeners registered

🚀 GoEvent Redis example running (mode: subscriber)
   Redis: localhost:6379
   Press Ctrl+C to exit

👂 Waiting for events...

[OrderProcessor] Processing order ORD-001 for customer CUST-123 (amount: $99.99)
[EmailSender] Sending confirmation email for order ORD-001
[EmailSender] Email sent for order ORD-001
[OrderProcessor] Order ORD-001 processed successfully

[PaymentNotifier] Payment processed for order ORD-001 (transaction: TXN-456)
```

### Publisher Terminal:
```
🚀 GoEvent Redis example running (mode: publisher)
   Redis: localhost:6379
   Press Ctrl+C to exit

📤 Publishing events...

Dispatching order.created event...
✅ Order event dispatch completed

Dispatching payment.processed event...
✅ Payment event dispatch completed

✅ All events processed successfully
```

## Architecture

```
┌─────────────┐         Redis Pub/Sub         ┌─────────────┐
│  Publisher  │  ──────────────────────────>  │ Subscriber1 │
│  Process    │                                │  Process    │
└─────────────┘                                └─────────────┘
                                                      ▲
                                                      │
                                               ┌──────┴──────┐
                                               │ Subscriber2 │
                                               │  Process    │
                                               └─────────────┘
```

All subscribers receive all events published by any publisher.

## Notes

- Each subscriber process runs its own set of listeners
- Events are JSON-serialized for transmission via Redis
- `DispatchHandle.Wait()` only waits for LOCAL handlers in the current process
- Use `evt.Close()` to properly cleanup Redis connections
