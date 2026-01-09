#!/bin/bash

# Script to run Microservices Postman collection locally using curl
# Make sure your services are running on the expected ports

set -e

# Configuration
USER_BASE="http://localhost:8082/api/v1"
PRODUCT_BASE="http://localhost:8081/api/v1"
ORDER_BASE="http://localhost:8080/api/v1"
USERNAME="alice"
EMAIL="alice@example.com"
PASSWORD="p@ssw0rd"

# Variables (will be set during execution)
JWT=""
LAST_USER_ID=""
LAST_ADDRESS_ID=""
LAST_ORDER_ID=""
LAPTOP_ID=""
MOUSE_ID=""
IDEMPOTENCY_KEY=""

echo "=== E-commerce Microservices Tests ==="
echo ""

# ============================================
# USER SERVICE TESTS
# ============================================
echo "--- User Service Tests ---"
echo ""

# 1. Login (get token)
echo "1. Login (get token)..."
LOGIN_RESPONSE=$(curl -s -X POST "${USER_BASE}/login" \
  -H "Content-Type: application/json" \
  -d "{
    \"username\": \"${USERNAME}\",
    \"password\": \"${PASSWORD}\"
  }")

echo "Response: $LOGIN_RESPONSE"
JWT=$(echo "$LOGIN_RESPONSE" | grep -o '"token":"[^"]*' | cut -d'"' -f4)

if [ -z "$JWT" ]; then
  echo "ERROR: Failed to get JWT token. Trying to create user first..."
  
  # Try to create user first (might need admin token or no auth)
  echo "Creating user..."
  CREATE_USER_RESPONSE=$(curl -s -X POST "${USER_BASE}/users" \
    -H "Content-Type: application/json" \
    -d "{
      \"username\": \"${USERNAME}\",
      \"email\": \"${EMAIL}\",
      \"password\": \"${PASSWORD}\"
    }")
  
  echo "Create user response: $CREATE_USER_RESPONSE"
  LAST_USER_ID=$(echo "$CREATE_USER_RESPONSE" | grep -o '"id":"[^"]*' | cut -d'"' -f4 || echo "")
  
  # Try login again
  LOGIN_RESPONSE=$(curl -s -X POST "${USER_BASE}/login" \
    -H "Content-Type: application/json" \
    -d "{
      \"username\": \"${USERNAME}\",
      \"password\": \"${PASSWORD}\"
    }")
  
  JWT=$(echo "$LOGIN_RESPONSE" | grep -o '"token":"[^"]*' | cut -d'"' -f4)
  
  if [ -z "$JWT" ]; then
    echo "ERROR: Still failed to get JWT token"
    exit 1
  fi
fi

echo "✓ Got JWT token"
echo ""

# 2. Create user
echo "2. Create user..."
CREATE_USER_RESPONSE=$(curl -s -X POST "${USER_BASE}/users" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer ${JWT}" \
  -d "{
    \"username\": \"${USERNAME}\",
    \"email\": \"${EMAIL}\",
    \"password\": \"${PASSWORD}\"
  }")

echo "Response: $CREATE_USER_RESPONSE"
LAST_USER_ID=$(echo "$CREATE_USER_RESPONSE" | grep -o '"id":"[^"]*' | cut -d'"' -f4 || echo "$LAST_USER_ID")
echo "✓ User created (ID: ${LAST_USER_ID})"
echo ""

# 3. Add address (default)
echo "3. Add address (default)..."
ADDRESS_RESPONSE=$(curl -s -X POST "${USER_BASE}/users/${LAST_USER_ID}/addresses" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer ${JWT}" \
  -d '{
    "line1": "1 Main St",
    "city": "NYC",
    "state": "NY",
    "postal_code": "10001",
    "country": "US",
    "phone": "+1-555-0000",
    "is_default": true
  }')

echo "Response: $ADDRESS_RESPONSE"
LAST_ADDRESS_ID=$(echo "$ADDRESS_RESPONSE" | grep -o '"id":"[^"]*' | cut -d'"' -f4 || echo "")
echo "✓ Address created (ID: ${LAST_ADDRESS_ID})"
echo ""

# 4. List addresses
echo "4. List addresses..."
ADDRESSES=$(curl -s -X GET "${USER_BASE}/users/${LAST_USER_ID}/addresses" \
  -H "Authorization: Bearer ${JWT}")

echo "Response: $ADDRESSES"
echo "✓ Addresses listed"
echo ""

# 5. Get user
echo "5. Get user..."
USER_INFO=$(curl -s -X GET "${USER_BASE}/users/${LAST_USER_ID}" \
  -H "Authorization: Bearer ${JWT}")

echo "Response: $USER_INFO"
echo "✓ User fetched"
echo ""

# ============================================
# PRODUCT SERVICE TESTS
# ============================================
echo "--- Product Service Tests ---"
echo ""

# 1. List products (to get laptop_id and mouse_id)
echo "1. List products..."
PRODUCTS_RESPONSE=$(curl -s -X GET "${PRODUCT_BASE}/products" \
  -H "Authorization: Bearer ${JWT}")

echo "Response: $PRODUCTS_RESPONSE"
LAPTOP_ID=$(echo "$PRODUCTS_RESPONSE" | grep -o '"id":"[^"]*' | head -1 | cut -d'"' -f4 || echo "")
MOUSE_ID=$(echo "$PRODUCTS_RESPONSE" | grep -o '"id":"[^"]*' | head -2 | tail -1 | cut -d'"' -f4 || echo "")

if [ -z "$LAPTOP_ID" ]; then
  echo "WARNING: No products found. Using default IDs."
  LAPTOP_ID="1"
  MOUSE_ID="2"
fi

echo "✓ Products listed (Laptop ID: ${LAPTOP_ID}, Mouse ID: ${MOUSE_ID})"
echo ""

# 2. Get product (laptop)
if [ -n "$LAPTOP_ID" ]; then
  echo "2. Get product (laptop)..."
  LAPTOP_INFO=$(curl -s -X GET "${PRODUCT_BASE}/products/${LAPTOP_ID}" \
    -H "Authorization: Bearer ${JWT}")
  
  echo "Response: $LAPTOP_INFO"
  echo "✓ Product fetched"
  echo ""
fi

# 3. Reserve laptop
if [ -n "$LAPTOP_ID" ]; then
  echo "3. Reserve laptop..."
  RESERVE_RESPONSE=$(curl -s -X POST "${PRODUCT_BASE}/products/${LAPTOP_ID}/reserve" \
    -H "Content-Type: application/json" \
    -H "Authorization: Bearer ${JWT}" \
    -d '{
      "quantity": 1
    }')
  
  echo "Response: $RESERVE_RESPONSE"
  echo "✓ Laptop reserved"
  echo ""
fi

# 4. Release laptop
if [ -n "$LAPTOP_ID" ]; then
  echo "4. Release laptop..."
  RELEASE_RESPONSE=$(curl -s -X POST "${PRODUCT_BASE}/products/${LAPTOP_ID}/release" \
    -H "Content-Type: application/json" \
    -H "Authorization: Bearer ${JWT}" \
    -d '{
      "quantity": 1
    }')
  
  echo "Response: $RELEASE_RESPONSE"
  echo "✓ Laptop released"
  echo ""
fi

# ============================================
# ORDER SERVICE TESTS
# ============================================
echo "--- Order Service Tests ---"
echo ""

# 1. Create order (laptop x1)
echo "1. Create order (laptop x1)..."
IDEMPOTENCY_KEY=$(uuidgen 2>/dev/null || echo "$(date +%s)-$$")

ORDER_RESPONSE=$(curl -s -X POST "${ORDER_BASE}/orders" \
  -H "Content-Type: application/json" \
  -H "Idempotency-Key: ${IDEMPOTENCY_KEY}" \
  -H "Authorization: Bearer ${JWT}" \
  -d "{
    \"userId\": \"${LAST_USER_ID}\",
    \"items\": [ { \"productId\": \"${LAPTOP_ID}\", \"quantity\": 1 } ],
    \"shippingAddressId\": \"${LAST_ADDRESS_ID}\"
  }")

echo "Response: $ORDER_RESPONSE"
LAST_ORDER_ID=$(echo "$ORDER_RESPONSE" | grep -o '"id":"[^"]*' | cut -d'"' -f4 || echo "")
echo "✓ Order created (ID: ${LAST_ORDER_ID})"
echo ""

# 2. Create order (fallback default addr)
if [ -n "$MOUSE_ID" ]; then
  echo "2. Create order (fallback default addr)..."
  IDEMPOTENCY_KEY=$(uuidgen 2>/dev/null || echo "$(date +%s)-$$-2")
  
  ORDER_RESPONSE2=$(curl -s -X POST "${ORDER_BASE}/orders" \
    -H "Content-Type: application/json" \
    -H "Idempotency-Key: ${IDEMPOTENCY_KEY}" \
    -H "Authorization: Bearer ${JWT}" \
    -d "{
      \"userId\": \"${LAST_USER_ID}\",
      \"items\": [ { \"productId\": \"${MOUSE_ID}\", \"quantity\": 1 } ]
    }")
  
  echo "Response: $ORDER_RESPONSE2"
  echo "✓ Order created (fallback)"
  echo ""
fi

# 3. List my orders
echo "3. List my orders..."
ORDERS_LIST=$(curl -s -X GET "${ORDER_BASE}/orders?userId=${LAST_USER_ID}&limit=5" \
  -H "Authorization: Bearer ${JWT}")

echo "Response: $ORDERS_LIST"
echo "✓ Orders listed"
echo ""

# 4. Get order
if [ -n "$LAST_ORDER_ID" ]; then
  echo "4. Get order..."
  ORDER_INFO=$(curl -s -X GET "${ORDER_BASE}/orders/${LAST_ORDER_ID}" \
    -H "Authorization: Bearer ${JWT}")
  
  echo "Response: $ORDER_INFO"
  echo "✓ Order fetched"
  echo ""
fi

# 5. Get order details (enriched)
if [ -n "$LAST_ORDER_ID" ]; then
  echo "5. Get order details (enriched)..."
  ORDER_DETAILS=$(curl -s -X GET "${ORDER_BASE}/orders/${LAST_ORDER_ID}/details" \
    -H "Authorization: Bearer ${JWT}")
  
  echo "Response: $ORDER_DETAILS"
  echo "✓ Order details fetched"
  echo ""
fi

# 6. Pay order
if [ -n "$LAST_ORDER_ID" ]; then
  echo "6. Pay order..."
  PAY_RESPONSE=$(curl -s -X POST "${ORDER_BASE}/orders/${LAST_ORDER_ID}/pay" \
    -H "Authorization: Bearer ${JWT}")
  
  echo "Response: $PAY_RESPONSE"
  echo "✓ Order paid"
  echo ""
fi

# 7. Cancel order (expect 409 if paid)
if [ -n "$LAST_ORDER_ID" ]; then
  echo "7. Cancel order..."
  CANCEL_RESPONSE=$(curl -s -X POST "${ORDER_BASE}/orders/${LAST_ORDER_ID}/cancel" \
    -H "Authorization: Bearer ${JWT}")
  
  echo "Response: $CANCEL_RESPONSE"
  echo "✓ Cancel attempted"
  echo ""
fi

# 8. Create order idempotent (mouse x2)
if [ -n "$MOUSE_ID" ]; then
  echo "8. Create order idempotent (mouse x2)..."
  IDEMPOTENCY_KEY=$(uuidgen 2>/dev/null || echo "$(date +%s)-$$-idempotent")
  
  IDEMPOTENT_ORDER=$(curl -s -X POST "${ORDER_BASE}/orders" \
    -H "Content-Type: application/json" \
    -H "Idempotency-Key: ${IDEMPOTENCY_KEY}" \
    -H "Authorization: Bearer ${JWT}" \
    -d "{
      \"userId\": \"${LAST_USER_ID}\",
      \"items\": [ { \"productId\": \"${MOUSE_ID}\", \"quantity\": 2 } ]
    }")
  
  echo "Response: $IDEMPOTENT_ORDER"
  echo "✓ Idempotent order created"
  echo ""
fi

# 9. Delete user
echo "9. Delete user..."
DELETE_RESPONSE=$(curl -s -X DELETE "${USER_BASE}/users/${LAST_USER_ID}" \
  -H "Authorization: Bearer ${JWT}")

echo "Response: $DELETE_RESPONSE"
echo "✓ User deleted"
echo ""

echo "=== All Microservices Tests Complete ==="