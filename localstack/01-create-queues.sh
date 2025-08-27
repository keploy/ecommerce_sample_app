#!/usr/bin/env sh
set -eu

# Create SQS queue for order events (idempotent)
awslocal sqs create-queue --queue-name order-events >/dev/null 2>&1 || true
