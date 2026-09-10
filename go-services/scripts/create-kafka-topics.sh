#!/bin/bash
# Wait for Kafka to be ready
echo "Waiting for Kafka to be ready..."
until kafka-topics --bootstrap-server localhost:9092 --list >/dev/null 2>&1; do
  sleep 1
done

echo "Kafka is ready. Creating topics..."

# Create order-events topic (idempotent - will not fail if exists)
kafka-topics --bootstrap-server localhost:9092 \
  --create \
  --topic order-events \
  --partitions 3 \
  --replication-factor 1 \
  --if-not-exists

echo "Topic 'order-events' created successfully!"
kafka-topics --bootstrap-server localhost:9092 --describe --topic order-events
