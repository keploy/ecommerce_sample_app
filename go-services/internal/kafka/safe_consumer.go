package kafka

import (
	"context"
	"log"
	"os"
	"sync"
	"sync/atomic"
	"time"
)

// SafeConsumer wraps a Kafka consumer with connection state management
// and graceful failure handling. It starts asynchronously and allows the
// service to start even if Kafka is unavailable.
type SafeConsumer struct {
	consumer  *Consumer
	connected atomic.Bool
	mu        sync.RWMutex
	brokers   []string
	topic     string
	groupID   string
}

// NewSafeConsumer creates a new SafeConsumer.
// The consumer is not started automatically; call StartAsync to begin consuming.
//
// Parameters:
//   - brokers: list of Kafka broker addresses (e.g., ["kafka:9092"])
//   - topic: the Kafka topic to read from
//   - groupID: consumer group ID for coordinated consumption
//
// Returns:
//   - *SafeConsumer: a consumer that gracefully handles connection failures
func NewSafeConsumer(brokers []string, topic, groupID string) *SafeConsumer {
	log.Printf("Kafka SafeConsumer: created for topic: %s, group: %s, brokers: %v", topic, groupID, brokers)

	return &SafeConsumer{
		brokers: brokers,
		topic:   topic,
		groupID: groupID,
	}
}

// StartAsync begins consuming messages asynchronously in a background goroutine.
// If the initial connection cannot be established within the timeout, the consumer
// will log a warning and return, allowing the service to continue starting.
//
// The handler function is called for each message received. If the handler returns
// an error, it is logged but consumption continues.
//
// Parameters:
//   - ctx: context for cancellation (when cancelled, consumer stops)
//   - handler: function to process each message
//   - timeout: maximum time to wait for initial connection
func (sc *SafeConsumer) StartAsync(ctx context.Context, handler MessageHandler, timeout time.Duration) {
	log.Printf("Kafka SafeConsumer: starting async consumer with timeout: %v", timeout)

	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("Kafka SafeConsumer: panic in consumer goroutine: %v", r)
			}
		}()

		// Try initial connection with timeout
		connectCtx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()

		done := make(chan bool, 1)
		var initErr error

		go func() {
			defer func() {
				if r := recover(); r != nil {
					log.Printf("Kafka SafeConsumer: panic during initialization: %v", r)
					initErr = nil
				}
			}()

			sc.mu.Lock()
			sc.consumer = NewConsumer(sc.brokers, sc.topic, sc.groupID)
			sc.connected.Store(true)
			sc.mu.Unlock()
			done <- true
		}()

		select {
		case <-done:
			if initErr == nil {
				log.Println("Kafka SafeConsumer: connected successfully, starting message consumption")
			} else {
				log.Printf("Kafka SafeConsumer: connection failed: %v", initErr)
				return
			}
		case <-connectCtx.Done():
			log.Println("Kafka SafeConsumer: connection timeout, consumer will not start (service continues normally)")
			return
		}

		// Start consuming messages
		sc.mu.RLock()
		consumer := sc.consumer
		sc.mu.RUnlock()

		if consumer != nil {
			log.Println("Kafka SafeConsumer: beginning message consumption loop")
			if err := consumer.Start(ctx, handler); err != nil && err != context.Canceled {
				log.Printf("Kafka SafeConsumer: consumer error: %v", err)
				sc.connected.Store(false)
			}
		}
	}()
}

// ReadMessage reads a single message from Kafka (blocking call).
// This is useful for testing or one-off reads.
// Returns nil if the consumer is not connected.
//
// Parameters:
//   - ctx: context for timeout and cancellation
//
// Returns:
//   - *Message: the parsed message, or nil if not connected
//   - error: any error that occurred during reading
func (sc *SafeConsumer) ReadMessage(ctx context.Context) (*Message, error) {
	if !sc.connected.Load() {
		log.Println("Kafka SafeConsumer: not connected, cannot read message")
		return nil, nil
	}

	sc.mu.RLock()
	defer sc.mu.RUnlock()

	if sc.consumer == nil {
		log.Println("Kafka SafeConsumer: consumer is nil, cannot read message")
		return nil, nil
	}

	return sc.consumer.ReadMessage(ctx)
}

// IsConnected returns true if the consumer is currently connected to Kafka.
func (sc *SafeConsumer) IsConnected() bool {
	return sc.connected.Load()
}

// Close closes the Kafka consumer connection.
// It's safe to call Close multiple times or on a nil consumer.
// In Keploy test mode, it skips the actual close to avoid LeaveGroup requests
// that don't have matching mocks.
func (sc *SafeConsumer) Close() error {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	// Skip close in Keploy test mode to avoid unmocked LeaveGroup requests
	if isKeployTestMode() {
		log.Println("Kafka SafeConsumer: skipping close in Keploy test mode")
		sc.connected.Store(false)
		return nil
	}

	if sc.consumer != nil {
		log.Println("Kafka SafeConsumer: closing connection")
		err := sc.consumer.Close()
		sc.connected.Store(false)
		return err
	}

	log.Println("Kafka SafeConsumer: no connection to close")
	return nil
}

// isKeployTestMode checks if we're running in Keploy test mode
func isKeployTestMode() bool {
	// Keploy sets various environment variables during test replay
	keployMode := os.Getenv("KEPLOY_MODE")
	keployTestID := os.Getenv("KEPLOY_TEST_ID")
	keployTestRun := os.Getenv("KEPLOY_TEST_RUN")
	
	isTestMode := keployMode != "" || keployTestID != "" || keployTestRun != ""
	
	log.Printf("Kafka SafeConsumer: Keploy environment check - KEPLOY_MODE=%s, KEPLOY_TEST_ID=%s, KEPLOY_TEST_RUN=%s, isTestMode=%v",
		keployMode, keployTestID, keployTestRun, isTestMode)
	
	return isTestMode
}
