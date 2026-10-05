package kafka

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"time"

	"github.com/segmentio/kafka-go"
)

// MessageHandler is a function that processes a Kafka message
type MessageHandler func(ctx context.Context, eventType string, payload map[string]interface{}) error

// Consumer wraps the kafka-go reader for consuming messages
type Consumer struct {
	reader  *kafka.Reader
	topic   string
	groupID string
}

// NewConsumer creates a new Kafka consumer
// brokers: list of Kafka broker addresses (e.g., ["kafka:9092"])
// topic: the Kafka topic to read from
// groupID: consumer group ID for coordinated consumption
func NewConsumer(brokers []string, topic, groupID string) *Consumer {
	// Check if we're in Keploy test mode
	isKeployTest := os.Getenv("KEPLOY_MODE") != "" ||
		os.Getenv("KEPLOY_TEST_ID") != "" ||
		os.Getenv("KEPLOY_TEST_RUN") != ""
	
	log.Printf("Kafka consumer: initializing for topic: %s, group: %s, brokers: %v, keployTestMode: %v",
		topic, groupID, brokers, isKeployTest)
	
	config := kafka.ReaderConfig{
		Brokers:        brokers,
		Topic:          topic,
		GroupID:        groupID,
		MinBytes:       10e3,             // 10KB
		MaxBytes:       10e6,             // 10MB
		MaxWait:        1 * time.Second,  // Max time to wait for new data
		CommitInterval: 1 * time.Second,  // Commit offsets every second
		StartOffset:    kafka.FirstOffset, // Start from the beginning if no offset
	}
	
	// In Keploy test mode, use a very short session timeout to minimize
	// the chance of LeaveGroup being sent
	if isKeployTest {
		log.Println("Kafka consumer: Keploy test mode detected, configuring for test replay")
		// Note: We can't completely prevent LeaveGroup, but we can minimize it
		// The real solution is to ensure LeaveGroup is mocked during recording
	}
	
	reader := kafka.NewReader(config)

	log.Printf("Kafka consumer initialized for topic: %s, group: %s, brokers: %v", topic, groupID, brokers)

	return &Consumer{
		reader:  reader,
		topic:   topic,
		groupID: groupID,
	}
}

// Start begins consuming messages and calls the handler for each message
// This is a blocking call that runs until the context is cancelled
func (c *Consumer) Start(ctx context.Context, handler MessageHandler) error {
	log.Printf("Kafka consumer started for topic: %s", c.topic)

	for {
		select {
		case <-ctx.Done():
			log.Println("Kafka consumer stopping due to context cancellation")
			return ctx.Err()
		default:
			msg, err := c.reader.ReadMessage(ctx)
			if err != nil {
				if ctx.Err() != nil {
					// Context was cancelled, this is expected
					return nil
				}
				log.Printf("Kafka: error reading message: %v", err)
				continue
			}

			// Parse the message
			var payload map[string]interface{}
			if err := json.Unmarshal(msg.Value, &payload); err != nil {
				log.Printf("Kafka: failed to unmarshal message: %v", err)
				continue
			}

			// Extract eventType from payload
			eventType := ""
			if et, ok := payload["eventType"].(string); ok {
				eventType = et
			}

			log.Printf("Kafka: received message - topic: %s, partition: %d, offset: %d, eventType: %s",
				msg.Topic, msg.Partition, msg.Offset, eventType)

			// Call the handler
			if err := handler(ctx, eventType, payload); err != nil {
				log.Printf("Kafka: handler error for eventType %s: %v", eventType, err)
				// Continue processing other messages even if handler fails
			}
		}
	}
}

// ReadMessage reads a single message (for testing or one-off reads)
func (c *Consumer) ReadMessage(ctx context.Context) (*Message, error) {
	msg, err := c.reader.ReadMessage(ctx)
	if err != nil {
		return nil, err
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(msg.Value, &payload); err != nil {
		return nil, err
	}

	eventType := ""
	if et, ok := payload["eventType"].(string); ok {
		eventType = et
	}

	return &Message{
		Topic:     msg.Topic,
		Partition: msg.Partition,
		Offset:    msg.Offset,
		Key:       string(msg.Key),
		EventType: eventType,
		Payload:   payload,
		Timestamp: msg.Time,
	}, nil
}

// Message represents a parsed Kafka message
type Message struct {
	Topic     string
	Partition int
	Offset    int64
	Key       string
	EventType string
	Payload   map[string]interface{}
	Timestamp time.Time
}

// Close closes the Kafka consumer connection
func (c *Consumer) Close() error {
	if c.reader != nil {
		log.Println("Kafka: closing consumer connection")
		return c.reader.Close()
	}
	return nil
}

// GetStats returns consumer statistics
func (c *Consumer) GetStats() kafka.ReaderStats {
	return c.reader.Stats()
}
