#!/bin/bash
export DB_HOST=localhost
export DB_PORT=3309
export DB_USER=user
export DB_PASSWORD=password
export DB_NAME=order_db
export PORT=8085
export USER_SERVICE_URL=http://localhost:8082/api/v1
export PRODUCT_SERVICE_URL=http://localhost:8081/api/v1
export AWS_REGION=us-east-1
export AWS_ACCESS_KEY_ID=test
export AWS_SECRET_ACCESS_KEY=test
export AWS_ENDPOINT=http://localhost:4566
export SQS_QUEUE_URL=http://localhost:4566/000000000000/order-events

# Run the service
go run order_service/main.go
