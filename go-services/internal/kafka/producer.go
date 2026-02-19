package kafka

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/segmentio/kafka-go"
)

// Producer wraps the kafka-go writer for sending messages
type Producer struct {
	writer *kafka.Writer
	topic  string
}

// NewProducer creates a new Kafka producer
// brokers: list of Kafka broker addresses (e.g., ["kafka:9092"])
// topic: the Kafka topic to write to
func NewProducer(brokers []string, topic string) *Producer {
	writer := &kafka.Writer{
		Addr:         kafka.TCP(brokers...),
		Topic:        topic,
		Balancer:     &kafka.LeastBytes{},
		BatchTimeout: 0,                 // Disable batching for deterministic behavior
		WriteTimeout: 10 * time.Second,
		ReadTimeout:  10 * time.Second,
		RequiredAcks: kafka.RequireAll, // More deterministic than RequireOne
		Async:        false,             // Synchronous writes for reliability
		MaxAttempts:  1,                 // Disable retries to avoid connection ID mismatches
	}

	log.Printf("Kafka producer initialized for topic: %s, brokers: %v", topic, brokers)

	return &Producer{
		writer: writer,
		topic:  topic,
	}
}

// SendMessage sends a message to Kafka
// key: message key (used for partitioning)
// value: the message payload (will be JSON encoded)
func (p *Producer) SendMessage(ctx context.Context, key string, value interface{}) error {
	// Serialize the value to JSON
	jsonValue, err := json.Marshal(value)
	if err != nil {
		log.Printf("Kafka: failed to marshal message: %v", err)
		return err
	}

	msg := kafka.Message{
		Key:   []byte(key),
		Value: jsonValue,
		Time:  time.Now(),
	}

	err = p.writer.WriteMessages(ctx, msg)
	if err != nil {
		log.Printf("Kafka: failed to send message to topic %s: %v", p.topic, err)
		return err
	}

	log.Printf("Kafka: message sent to topic %s with key: %s", p.topic, key)
	return nil
}

// SendEvent is a convenience method for sending events with eventType
func (p *Producer) SendEvent(ctx context.Context, eventType string, payload map[string]interface{}) error {
	// Add eventType to payload
	payload["eventType"] = eventType

	// Use eventType as key for partitioning
	return p.SendMessage(ctx, eventType, payload)
}

// Close closes the Kafka producer connection
func (p *Producer) Close() error {
	if p.writer != nil {
		log.Println("Kafka: closing producer connection")
		return p.writer.Close()
	}
	return nil
}

// IsHealthy checks if the producer can connect to Kafka
func (p *Producer) IsHealthy(ctx context.Context) bool {
	// Try to get topic metadata to verify connection
	conn, err := kafka.DialContext(ctx, "tcp", p.writer.Addr.String())
	if err != nil {
		return false
	}
	defer conn.Close()
	return true
}
