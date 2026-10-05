#!/bin/bash

# Local Kafka Testing Script for Ecommerce Sample App
# Uses locally built keploy-enterprise binary

set -e

echo "🚀 Testing Kafka Integration with Local Binaries"
echo "================================================"

# Check if keploy-enterprise is available
if ! command -v keploy-enterprise &> /dev/null; then
    echo "❌ keploy-enterprise not found in PATH"
    echo "Please run: cd ../enterprise && ./build-and-install.sh && sudo mv keploy-enterprise /usr/local/bin/"
    exit 1
fi

echo "✅ Using keploy-enterprise: $(which keploy-enterprise)"

# Change to go-services directory
cd go-services

# Cleanup function
cleanup() {
    echo "🧹 Cleaning up..."
    docker compose down -v --remove-orphans 2>/dev/null || true
    docker ps -a --filter "name=go-services" -q | xargs -r docker rm -f 2>/dev/null || true
}

trap cleanup EXIT

# Record mode
echo ""
echo "📦 Starting RECORD mode..."
echo "================================================"

sudo -E keploy-enterprise record \
    -c "docker compose up" \
    --container-name="order_service" \
    --build-delay 90 \
    --path="./order_service" \
    --generateGithubActions=false

echo ""
echo "✅ Recording complete!"
echo "Generated mocks:"
ls -la order_service/keploy/

# Check if Kafka mocks were generated
if grep -q "kind: Kafka" order_service/keploy/*/mocks.yaml 2>/dev/null; then
    echo "✅ SUCCESS: Kafka mocks generated!"
else
    echo "⚠️  WARNING: No Kafka mocks found (check if Generic mocks were created instead)"
fi

echo ""
echo "📦 Starting TEST mode..."
echo "================================================"

sudo -E keploy-enterprise test \
    -c "docker compose up" \
    --container-name="order_service" \
    --delay 90 \
    --path="./order_service" \
    --generateGithubActions=false \
    --disableMockUpload

echo ""
echo "🎉 Test complete!"
