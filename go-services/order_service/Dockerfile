FROM golang:1.21-alpine AS builder

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source
COPY . .

# Download the time freeze agent for amd64
ADD https://keploy-enterprise.s3.us-west-2.amazonaws.com/releases/latest/assets/go_freeze_time_amd64 /lib/keploy/go_freeze_time_amd64
# Set suitable permissions
RUN chmod +x /lib/keploy/go_freeze_time_amd64
# Run the binary
RUN /lib/keploy/go_freeze_time_amd64
# # Build with fake time (during test mode)
RUN go build -tags=faketime -o /order-service ./order_service
# RUN go build -o /order-service ./order_service

# RUN go build -cover -covermode=atomic -coverpkg=./... -o /order-service ./order_service

# Runtime
FROM alpine:3.19

RUN apk --no-cache add ca-certificates bash

# Copy Go toolchain from the builder stage (Required for Keploy coverage)
COPY --from=builder /usr/local/go /usr/local/go

# Set Go environment variables so the app can use internal go tools
ENV GOROOT=/usr/local/go
ENV PATH=/usr/local/go/bin:${PATH}

# Add Keploy CA certificate setup (like Python service)
ADD https://raw.githubusercontent.com/keploy/keploy/refs/heads/main/pkg/core/proxy/tls/asset/ca.crt /app/ca.crt
ADD https://raw.githubusercontent.com/keploy/keploy/refs/heads/main/pkg/core/proxy/tls/asset/setup_ca.sh /app/setup_ca.sh

WORKDIR /app

# Copy go.mod and go.sum (Required for dependency resolution during coverage)
COPY --from=builder /app/go.mod /app/go.sum /app/

# Set the GOMOD environment variable
ENV GOMOD=/app/go.mod

COPY --from=builder /order-service .

# Create entrypoint that sets up CA before running
COPY ./order_service/entrypoint.sh /app/entrypoint.sh
RUN chmod +x /app/entrypoint.sh

EXPOSE 8080

ENTRYPOINT ["/bin/bash", "/app/entrypoint.sh"]
