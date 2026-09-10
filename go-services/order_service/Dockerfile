FROM golang:1.24

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
# Build with fake time (during test mode)
RUN go build -tags=faketime -o /order-service ./order_service

# Install ca-certificates and bash (using apt for Debian-based image)
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates sudo bash && rm -rf /var/lib/apt/lists/*

# Set Go environment variables so the app can use internal go tools
ENV GOROOT=/usr/local/go
ENV PATH=/usr/local/go/bin:${PATH}
ENV GOMOD=/app/go.mod

# Add Keploy CA certificate setup
ADD https://raw.githubusercontent.com/keploy/keploy/refs/heads/main/pkg/core/proxy/tls/asset/ca.crt /app/ca.crt
ADD https://raw.githubusercontent.com/keploy/keploy/refs/heads/main/pkg/core/proxy/tls/asset/setup_ca.sh /app/setup_ca.sh

# Copy entrypoint script
COPY ./order_service/entrypoint.sh /app/entrypoint.sh
RUN chmod +x /app/entrypoint.sh

EXPOSE 8080

ENTRYPOINT ["/bin/bash", "/app/entrypoint.sh"]
