package kafka

import (
	"context"
	"log"
	"os"
	"sync"
	"sync/atomic"
	"time"
)

// SafeProducer wraps a Kafka producer with connection state management
// and graceful failure handling. It allows the service to start even if
// Kafka is unavailable and skips event emission when not connected.
type SafeProducer struct {
	producer  *Producer
	connected atomic.Bool
	mu        sync.RWMutex
	brokers   []string
	topic     string
}

// NewSafeProducer creates a new SafeProducer with timeout-based initialization.
// If the connection cannot be established within the timeout, the producer
// will operate in degraded mode (events will be logged but not sent).
// In Keploy test mode, the producer is not initialized at all.
//
// Parameters:
//   - brokers: list of Kafka broker addresses (e.g., ["kafka:9092"])
//   - topic: the Kafka topic to write to
//   - timeout: maximum time to wait for initial connection
//
// Returns:
//   - *SafeProducer: a producer that gracefully handles connection failures
func NewSafeProducer(brokers []string, topic string, timeout time.Duration) *SafeProducer {
	sp := &SafeProducer{
		brokers: brokers,
		topic:   topic,
	}

	// Check if we're in Keploy test mode (replay)
	// During test mode, we skip Kafka initialization since mocks will be replayed
	// During record mode, we need to connect to Kafka to capture the traffic
	keployMode := os.Getenv("KEPLOY_MODE")
	
	isTestMode := keployMode == "test"
	
	if isTestMode {
		log.Printf("Kafka SafeProducer: Keploy test mode detected (KEPLOY_MODE=%s), skipping producer initialization", keployMode)
		log.Println("Kafka SafeProducer: operating in test mode - all events will be logged but not sent to Kafka")
		return sp
	}

	log.Printf("Kafka SafeProducer: attempting to connect to brokers: %v, topic: %s (timeout: %v)", brokers, topic, timeout)

	// Try to connect with timeout
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	done := make(chan bool, 1)
	var initErr error

	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("Kafka SafeProducer: panic during initialization: %v", r)
				initErr = nil
			}
		}()

		sp.producer = NewProducer(brokers, topic)
		sp.connected.Store(true)
		done <- true
	}()

	select {
	case <-done:
		if initErr == nil {
			log.Println("Kafka SafeProducer: connected successfully")
		} else {
			log.Printf("Kafka SafeProducer: connection failed: %v, operating in degraded mode", initErr)
		}
	case <-ctx.Done():
		log.Println("Kafka SafeProducer: connection timeout, operating in degraded mode (events will be logged but not sent)")
	}

	return sp
}

// SendEvent sends an event to Kafka with the specified eventType and payload.
// If the producer is not connected, the event is logged but not sent (graceful skip).
//
// Parameters:
//   - ctx: context for timeout and cancellation
//   - eventType: the type of event (used as message key for partitioning)
//   - payload: the event data (will be JSON encoded)
//
// Returns:
//   - error: nil if successful or if gracefully skipped, error otherwise
func (sp *SafeProducer) SendEvent(ctx context.Context, eventType string, payload map[string]interface{}) error {
	if !sp.connected.Load() {
		log.Printf("Kafka SafeProducer: not connected, skipping event: %s (payload: %v)", eventType, payload)
		return nil // Gracefully skip
	}

	sp.mu.RLock()
	defer sp.mu.RUnlock()

	if sp.producer == nil {
		log.Printf("Kafka SafeProducer: producer is nil, skipping event: %s", eventType)
		return nil
	}

	return sp.producer.SendEvent(ctx, eventType, payload)
}

// SendMessage sends a raw message to Kafka with the specified key and value.
// If the producer is not connected, the message is logged but not sent (graceful skip).
//
// Parameters:
//   - ctx: context for timeout and cancellation
//   - key: message key (used for partitioning)
//   - value: the message payload (will be JSON encoded)
//
// Returns:
//   - error: nil if successful or if gracefully skipped, error otherwise
func (sp *SafeProducer) SendMessage(ctx context.Context, key string, value interface{}) error {
	if !sp.connected.Load() {
		log.Printf("Kafka SafeProducer: not connected, skipping message with key: %s", key)
		return nil // Gracefully skip
	}

	sp.mu.RLock()
	defer sp.mu.RUnlock()

	if sp.producer == nil {
		log.Printf("Kafka SafeProducer: producer is nil, skipping message with key: %s", key)
		return nil
	}

	return sp.producer.SendMessage(ctx, key, value)
}

// IsConnected returns true if the producer is currently connected to Kafka.
func (sp *SafeProducer) IsConnected() bool {
	return sp.connected.Load()
}

// Close closes the Kafka producer connection.
// It's safe to call Close multiple times or on a nil producer.
// In Keploy test mode, it skips the actual close to avoid unmocked requests.
func (sp *SafeProducer) Close() error {
	sp.mu.Lock()
	defer sp.mu.Unlock()

	// Skip close in Keploy test mode to avoid unmocked requests
	keployMode := os.Getenv("KEPLOY_MODE")
	
	isTestMode := keployMode == "test"
	
	log.Printf("Kafka SafeProducer: Close() called - KEPLOY_MODE=%s, isTestMode=%v",
		keployMode, isTestMode)
	
	if isTestMode {
		log.Println("Kafka SafeProducer: skipping close in Keploy test mode")
		sp.connected.Store(false)
		return nil
	}

	if sp.producer != nil {
		log.Println("Kafka SafeProducer: closing connection")
		err := sp.producer.Close()
		sp.connected.Store(false)
		return err
	}

	log.Println("Kafka SafeProducer: no connection to close")
	return nil
}
