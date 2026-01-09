FROM golang:1.22-alpine AS builder

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source
COPY . .

# Build
RUN go build -o /order-service ./order_service

# Runtime
FROM alpine:3.19

RUN apk --no-cache add ca-certificates bash

# Add Keploy CA certificate setup (like Python service)
ADD https://raw.githubusercontent.com/keploy/keploy/refs/heads/main/pkg/core/proxy/tls/asset/ca.crt /app/ca.crt
ADD https://raw.githubusercontent.com/keploy/keploy/refs/heads/main/pkg/core/proxy/tls/asset/setup_ca.sh /app/setup_ca.sh

WORKDIR /app
COPY --from=builder /order-service .

# Create entrypoint that sets up CA before running
COPY ./order_service/entrypoint.sh /app/entrypoint.sh
RUN chmod +x /app/entrypoint.sh

EXPOSE 8080

ENTRYPOINT ["/bin/bash", "/app/entrypoint.sh"]
