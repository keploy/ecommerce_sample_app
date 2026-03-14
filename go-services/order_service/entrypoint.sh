#!/bin/bash
set -eu

# Setup Keploy CA once (non-fatal if repeated)
if [ -f ./setup_ca.sh ]; then
  source ./setup_ca.sh || true
fi

# Detect Keploy test mode by checking for Keploy agent environment variables
# The Keploy agent sets these when running in test mode
if [ ! -z "${KEPLOY_TEST_ID:-}" ] || [ ! -z "${KEPLOY_TEST_RUN:-}" ]; then
  export KEPLOY_MODE="test"
  echo "🧪 Keploy test mode detected (KEPLOY_TEST_ID or KEPLOY_TEST_RUN set)"
  echo "   Setting KEPLOY_MODE=test"
elif [ ! -z "${KEPLOY_RECORD:-}" ]; then
  export KEPLOY_MODE="record"
  echo "📹 Keploy record mode detected"
  echo "   Setting KEPLOY_MODE=record"
else
  # Additional check: if Keploy agent is intercepting our process, we're in test mode
  # This is a fallback for Keploy v3 which uses eBPF and may not set env vars
  if pgrep -f "keploy.*test" > /dev/null 2>&1; then
    export KEPLOY_MODE="test"
    echo "🧪 Keploy test mode detected (keploy test process found)"
    echo "   Setting KEPLOY_MODE=test"
  fi
fi

# Print environment for debugging
echo "Environment: KEPLOY_MODE=${KEPLOY_MODE:-not set}, KEPLOY_TEST_ID=${KEPLOY_TEST_ID:-not set}, KEPLOY_TEST_RUN=${KEPLOY_TEST_RUN:-not set}"

for candidate in "${ORDER_SERVICE_BIN:-}" "/order-service" "/app/order-service" "./order-service"; do
  if [ -n "$candidate" ] && [ -x "$candidate" ]; then
    exec "$candidate"
  fi
done

echo "❌ order_service binary not found. Checked: ${ORDER_SERVICE_BIN:-<unset>}, /order-service, /app/order-service, ./order-service"
exit 127
