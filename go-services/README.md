# Go Services - E-commerce Sample App

This directory contains Go implementations of all the Python microservices.

## Services

| Service | Port | Description |
|---------|------|-------------|
| user-service | 8082 | User management, authentication, addresses |
| product-service | 8081 | Product catalog, stock management |
| order-service | 8080 | Order lifecycle, inter-service calls, SQS events |
| apigateway | 8083 | HTTP reverse proxy to all services |

## Quick Start

```bash
# Build and start all services
docker compose up -d --build

# Check health
docker compose ps

# View logs
docker compose logs -f

# Stop
docker compose down
```

## Running E2E Tests

```bash
# Start services first
docker compose up -d --build

# Wait for healthy (about 30 seconds)
sleep 30

# Run E2E tests
go test -v -tags=e2e ./tests/e2e/...
```

## Development

```bash
# Download dependencies
go mod tidy

# Run individual service locally
DB_HOST=localhost DB_USER=user DB_PASSWORD=password go run ./cmd/userservice
```

## Architecture

All services use:
- **Gin** for HTTP routing
- **sqlx** for MySQL access
- **JWT** for authentication
- **AWS SDK v2** for SQS (order-service only)

Database schemas are reused from the original Python service `db.sql` files.
