package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/openframebox/goevent"
)

// OrderCreatedEvent represents an order being created
type OrderCreatedEvent struct {
	OrderID    string  `json:"order_id"`
	CustomerID string  `json:"customer_id"`
	Amount     float64 `json:"amount"`
}

func (e *OrderCreatedEvent) Name() string {
	return "order.created"
}

func (e *OrderCreatedEvent) Payload() map[string]any {
	return map[string]any{
		"order_id":    e.OrderID,
		"customer_id": e.CustomerID,
		"amount":      e.Amount,
	}
}

// PaymentProcessedEvent represents a payment being processed
type PaymentProcessedEvent struct {
	OrderID       string  `json:"order_id"`
	TransactionID string  `json:"transaction_id"`
	Amount        float64 `json:"amount"`
}

func (e *PaymentProcessedEvent) Name() string {
	return "payment.processed"
}

func (e *PaymentProcessedEvent) Payload() map[string]any {
	return map[string]any{
		"order_id":       e.OrderID,
		"transaction_id": e.TransactionID,
		"amount":         e.Amount,
	}
}

// Register event types (required for Redis driver)
func init() {
	goevent.RegisterEventType(&OrderCreatedEvent{})
	goevent.RegisterEventType(&PaymentProcessedEvent{})
}

// OrderProcessor handles order processing
type OrderProcessor struct{}

func (l *OrderProcessor) EventName() string {
	return "order.created"
}

func (l *OrderProcessor) OnEvent(event goevent.Event) error {
	e := event.(*OrderCreatedEvent)
	fmt.Printf("[OrderProcessor] Processing order %s for customer %s (amount: $%.2f)\n",
		e.OrderID, e.CustomerID, e.Amount)

	// Simulate processing time
	time.Sleep(500 * time.Millisecond)

	fmt.Printf("[OrderProcessor] Order %s processed successfully\n", e.OrderID)
	return nil
}

// EmailSender sends email notifications
type EmailSender struct{}

func (l *EmailSender) EventName() string {
	return "order.created"
}

func (l *EmailSender) OnEvent(event goevent.Event) error {
	e := event.(*OrderCreatedEvent)
	fmt.Printf("[EmailSender] Sending confirmation email for order %s\n", e.OrderID)

	// Simulate email sending
	time.Sleep(200 * time.Millisecond)

	fmt.Printf("[EmailSender] Email sent for order %s\n", e.OrderID)
	return nil
}

// This listener runs asynchronously
func (l *EmailSender) Options() goevent.ListenerOptions {
	return goevent.ListenerOptions{Async: true}
}

// PaymentNotifier sends payment notifications
type PaymentNotifier struct{}

func (l *PaymentNotifier) EventName() string {
	return "payment.processed"
}

func (l *PaymentNotifier) OnEvent(event goevent.Event) error {
	e := event.(*PaymentProcessedEvent)
	fmt.Printf("[PaymentNotifier] Payment processed for order %s (transaction: %s)\n",
		e.OrderID, e.TransactionID)
	return nil
}

func main() {
	// Get Redis address from environment or use default
	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "localhost:6379"
	}

	// Get process mode from arguments
	mode := "both" // default: both publisher and subscriber
	if len(os.Args) > 1 {
		mode = os.Args[1]
	}

	// Create event bus with Redis driver
	evt := goevent.NewWithConfig(&goevent.Config{
		Driver: goevent.DriverRedis,
		Redis: &goevent.RedisConfig{
			Addr:          redisAddr,
			ChannelPrefix: "example:",
		},
	})
	defer evt.Close()

	// Register listeners if in subscriber or both mode
	if mode == "subscriber" || mode == "both" {
		fmt.Println("📥 Registering event listeners...")
		evt.RegisterListener(
			&OrderProcessor{},
			&EmailSender{},
			&PaymentNotifier{},
		)
		fmt.Println("✅ Listeners registered")
	}

	// Publish events if in publisher or both mode
	if mode == "publisher" || mode == "both" {
		go func() {
			time.Sleep(1 * time.Second) // Wait for subscribers to be ready

			fmt.Println("\n📤 Publishing events...")

			// Dispatch order created event
			fmt.Println("\nDispatching order.created event...")
			orderHandle := evt.Dispatch(&OrderCreatedEvent{
				OrderID:    "ORD-001",
				CustomerID: "CUST-123",
				Amount:     99.99,
			})

			// Wait for local handlers (if any)
			orderHandle.Wait()
			fmt.Println("✅ Order event dispatch completed")

			time.Sleep(1 * time.Second)

			// Dispatch payment processed event
			fmt.Println("\nDispatching payment.processed event...")
			paymentHandle := evt.Dispatch(&PaymentProcessedEvent{
				OrderID:       "ORD-001",
				TransactionID: "TXN-456",
				Amount:        99.99,
			})

			// Wait for local handlers
			paymentHandle.Wait()
			fmt.Println("✅ Payment event dispatch completed")

			// Check for errors
			if errs := evt.GetErrors(); len(errs) > 0 {
				fmt.Printf("\n⚠️  Errors occurred: %d\n", len(errs))
				for _, err := range errs {
					fmt.Printf("   - %s\n", err)
				}
			} else {
				fmt.Println("\n✅ All events processed successfully")
			}
		}()
	}

	// Graceful shutdown
	fmt.Printf("\n🚀 GoEvent Redis example running (mode: %s)\n", mode)
	fmt.Println("   Redis:", redisAddr)
	fmt.Println("   Press Ctrl+C to exit")

	if mode == "subscriber" {
		fmt.Println("👂 Waiting for events...")
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	<-sigChan
	fmt.Println("\n\n🛑 Shutting down gracefully...")
	evt.Wait() // Wait for all pending handlers
	fmt.Println("✅ Shutdown complete")
}
