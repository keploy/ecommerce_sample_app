#!/bin/bash
# Test script that triggers order_service to make calls (which Keploy will record)

USER_BASE="http://localhost:8082/api/v1"
PRODUCT_BASE="http://localhost:8081/api/v1"
ORDER_BASE="http://localhost:8080/api/v1"

echo "=== Setup: Login and get token ==="
RESPONSE=$(curl -s -X POST "${USER_BASE}/login" \
  -H "Content-Type: application/json" \
  -d '{"username": "admin", "password": "admin123"}')
JWT=$(echo $RESPONSE | grep -o '"token":"[^"]*"' | cut -d'"' -f4)
echo "Got JWT: ${JWT:0:20}..."

# Create unique user to avoid conflicts
TIMESTAMP=$(date +%s)
echo -e "\n=== Setup: Create user alice_${TIMESTAMP} ==="
RESPONSE=$(curl -s -X POST "${USER_BASE}/users" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $JWT" \
  -d "{\"username\": \"alice_${TIMESTAMP}\", \"email\": \"alice_${TIMESTAMP}@example.com\", \"password\": \"p@ssw0rd\"}")
echo $RESPONSE | jq '.'

echo -e "\n=== Setup: Login as alice_${TIMESTAMP} ==="
RESPONSE=$(curl -s -X POST "${USER_BASE}/login" \
  -H "Content-Type: application/json" \
  -d "{\"username\": \"alice_${TIMESTAMP}\", \"password\": \"p@ssw0rd\"}")
JWT=$(echo $RESPONSE | grep -o '"token":"[^"]*"' | cut -d'"' -f4)
USER_ID=$(echo $RESPONSE | grep -o '"id":"[^"]*"' | cut -d'"' -f4)
echo "USER_ID: $USER_ID"

echo -e "\n=== Setup: Add address ==="
RESPONSE=$(curl -s -X POST "${USER_BASE}/users/${USER_ID}/addresses" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $JWT" \
  -d '{
    "line1": "1 Main St",
    "city": "NYC",
    "state": "NY",
    "postal_code": "10001",
    "country": "US",
    "phone": "+1-555-0000",
    "is_default": true
  }')
ADDRESS_ID=$(echo $RESPONSE | grep -o '"id":"[^"]*"' | cut -d'"' -f4)
echo "ADDRESS_ID: $ADDRESS_ID"

echo -e "\n=== Setup: Fetch real product IDs ==="
RESPONSE=$(curl -s -X GET "${PRODUCT_BASE}/products" \
  -H "Authorization: Bearer $JWT")
echo $RESPONSE | jq '.'
LAPTOP_ID=$(echo $RESPONSE | jq -r '.[0].id')
MOUSE_ID=$(echo $RESPONSE | jq -r '.[1].id')
echo "LAPTOP_ID: $LAPTOP_ID"
echo "MOUSE_ID: $MOUSE_ID"

echo -e "\n============================================================"
echo "=== KEPLOY SHOULD RECORD THE FOLLOWING CALLS ==="
echo "============================================================"

# These calls to order_service will trigger it to call user_service and product_service
# Keploy will record these outbound calls from order_service

echo -e "\n=== 1. CREATE ORDER (Keploy records order→user + order→product calls) ==="
RESPONSE=$(curl -s -X POST "${ORDER_BASE}/orders" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $JWT" \
  -H "Idempotency-Key: $(uuidgen 2>/dev/null || cat /proc/sys/kernel/random/uuid)" \
  -d "{
    \"userId\": \"${USER_ID}\",
    \"items\": [{\"productId\": \"${LAPTOP_ID}\", \"quantity\": 1}],
    \"shippingAddressId\": \"${ADDRESS_ID}\"
  }")
echo $RESPONSE | jq '.'
ORDER_ID=$(echo $RESPONSE | grep -o '"id":"[^"]*"' | cut -d'"' -f4)
echo "ORDER_ID: $ORDER_ID"

echo -e "\n=== 2. GET ORDER (Get single order by ID) ==="
curl -s -X GET "${ORDER_BASE}/orders/${ORDER_ID}" \
  -H "Authorization: Bearer $JWT" | jq '.'

echo -e "\n=== 2.1. GET ORDER (DUPLICATE - for dedup testing) ==="
curl -s -X GET "${ORDER_BASE}/orders/${ORDER_ID}" \
  -H "Authorization: Bearer $JWT" | jq '.'

echo -e "\n=== 2.2. GET ORDER (DUPLICATE - for dedup testing) ==="
curl -s -X GET "${ORDER_BASE}/orders/${ORDER_ID}" \
  -H "Authorization: Bearer $JWT" | jq '.'

echo -e "\n=== 3. GET ORDER DETAILS (Keploy records enrichment calls) ==="
curl -s -X GET "${ORDER_BASE}/orders/${ORDER_ID}/details" \
  -H "Authorization: Bearer $JWT" | jq '.'

echo -e "\n=== 3.1. GET ORDER DETAILS (DUPLICATE - for dedup testing) ==="
curl -s -X GET "${ORDER_BASE}/orders/${ORDER_ID}/details" \
  -H "Authorization: Bearer $JWT" | jq '.'

echo -e "\n=== 4. LIST ORDERS (Keploy records list operation) ==="
curl -s -X GET "${ORDER_BASE}/orders?userId=${USER_ID}&limit=5" \
  -H "Authorization: Bearer $JWT" | jq '.'

echo -e "\n=== 4.1. LIST ORDERS (DUPLICATE - for dedup testing) ==="
curl -s -X GET "${ORDER_BASE}/orders?userId=${USER_ID}&limit=5" \
  -H "Authorization: Bearer $JWT" | jq '.'

echo -e "\n=== 5. CREATE ANOTHER ORDER (Mouse) ==="
RESPONSE=$(curl -s -X POST "${ORDER_BASE}/orders" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $JWT" \
  -H "Idempotency-Key: $(uuidgen 2>/dev/null || cat /proc/sys/kernel/random/uuid)" \
  -d "{
    \"userId\": \"${USER_ID}\",
    \"items\": [{\"productId\": \"${MOUSE_ID}\", \"quantity\": 2}]
  }")
echo $RESPONSE | jq '.'
ORDER_ID_2=$(echo $RESPONSE | grep -o '"id":"[^"]*"' | cut -d'"' -f4)
echo "ORDER_ID_2: $ORDER_ID_2"

echo -e "\n=== 6. GET ORDER (Get second order by ID) ==="
curl -s -X GET "${ORDER_BASE}/orders/${ORDER_ID_2}" \
  -H "Authorization: Bearer $JWT" | jq '.'

echo -e "\n=== 6.1. GET ORDER (DUPLICATE - for dedup testing) ==="
curl -s -X GET "${ORDER_BASE}/orders/${ORDER_ID_2}" \
  -H "Authorization: Bearer $JWT" | jq '.'

echo -e "\n=== 7. CANCEL ORDER (Cancel the second order) ==="
curl -s -X POST "${ORDER_BASE}/orders/${ORDER_ID_2}/cancel" \
  -H "Authorization: Bearer $JWT" | jq '.'

echo -e "\n=== 7.1. CANCEL ORDER (DUPLICATE - idempotent, returns 200 if already cancelled) ==="
curl -s -X POST "${ORDER_BASE}/orders/${ORDER_ID_2}/cancel" \
  -H "Authorization: Bearer $JWT" | jq '.'

echo -e "\n=== 7.2. CANCEL ORDER (DUPLICATE - idempotent, returns 200 if already cancelled) ==="
curl -s -X POST "${ORDER_BASE}/orders/${ORDER_ID_2}/cancel" \
  -H "Authorization: Bearer $JWT" | jq '.'

echo -e "\n=== 8. PAY ORDER (Keploy records payment validation calls) ==="
curl -s -X POST "${ORDER_BASE}/orders/${ORDER_ID}/pay" \
  -H "Authorization: Bearer $JWT" | jq '.'

echo -e "\n=== 8.1. PAY ORDER (DUPLICATE - idempotent, returns 200 if already paid) ==="
curl -s -X POST "${ORDER_BASE}/orders/${ORDER_ID}/pay" \
  -H "Authorization: Bearer $JWT" | jq '.'

echo -e "\n=== 8.2. PAY ORDER (DUPLICATE - idempotent, returns 200 if already paid) ==="
curl -s -X POST "${ORDER_BASE}/orders/${ORDER_ID}/pay" \
  -H "Authorization: Bearer $JWT" | jq '.'

echo -e "\n=== 9. GET HEALTH (Health check endpoint) ==="
curl -s -X GET "${ORDER_BASE}/health" \
  -H "Authorization: Bearer $JWT" | jq '.'

echo -e "\n=== 9.1. GET HEALTH (DUPLICATE - for dedup testing) ==="
curl -s -X GET "${ORDER_BASE}/health" \
  -H "Authorization: Bearer $JWT" | jq '.'

echo -e "\n=== 9.2. GET HEALTH (DUPLICATE - for dedup testing) ==="
curl -s -X GET "${ORDER_BASE}/health" \
  -H "Authorization: Bearer $JWT" | jq '.'

echo -e "\n=== 10. GET STATS (Stats endpoint) ==="
curl -s -X GET "${ORDER_BASE}/stats" \
  -H "Authorization: Bearer $JWT" | jq '.'

echo -e "\n=== 10.1. GET STATS (DUPLICATE - for dedup testing) ==="
curl -s -X GET "${ORDER_BASE}/stats" \
  -H "Authorization: Bearer $JWT" | jq '.'

echo -e "\n=== 10.2. GET STATS (DUPLICATE - for dedup testing) ==="
curl -s -X GET "${ORDER_BASE}/stats" \
  -H "Authorization: Bearer $JWT" | jq '.'

echo -e "\n============================================================"
echo "Done! Check ./order_service/keploy/ for recorded test cases"
echo "============================================================"

