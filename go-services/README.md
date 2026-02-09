# Go E-commerce Microservices

A microservices-based e-commerce application built with Go, featuring Kafka for event-driven architecture.

## Architecture

| Service | Port | Description |
|---------|------|-------------|
| User Service | 8082 | User authentication and management |
| Product Service | 8081 | Product catalog management |
| Order Service | 8080 | Order processing with Kafka events |
| API Gateway | 8083 | Unified API entry point |
| Kafka | 29092 | Message broker for events |
| Zookeeper | 2181 | Kafka coordination |

## Prerequisites

- Docker and Docker Compose installed
- `curl` command available (for testing)

## Quick Start

### 1. Start All Services

```bash
cd go-services
docker compose up -d --build
```

Wait about 30 seconds for all services to be ready.

### 2. Verify Services Are Running

```bash
docker compose ps
```

All services should show "Up" status.

---

## Testing the Complete Flow (Copy-Paste Ready)

### Step 1: Login and Get Token

```bash
# Login as admin user
curl -s -X POST http://localhost:8082/api/v1/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"admin123"}'
```

**Expected Response:**
```json
{"email":"admin@example.com","id":"...","token":"eyJ...","username":"admin"}
```

Copy the `token` value for the next steps.

### Step 2: Set Token as Environment Variable

```bash
# Replace <YOUR_TOKEN> with the token from Step 1
export TOKEN="<YOUR_TOKEN>"
```

Or use this one-liner to automatically set the token:

```bash
export TOKEN=$(curl -s -X POST http://localhost:8082/api/v1/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"admin123"}' | \
  grep -o '"token":"[^"]*"' | cut -d'"' -f4)
echo "Token set: $TOKEN"
```

### Step 3: Create a Product

```bash
curl -s -X POST http://localhost:8081/api/v1/products \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"name":"Laptop","description":"Gaming Laptop","price":999.99,"stock":50}'
```

**Expected Response:**
```json
{"id":"<product-id>"}
```

Save the product ID:
```bash
export PRODUCT_ID="<product-id>"
```

Or use this one-liner:
```bash
export PRODUCT_ID=$(curl -s -X POST http://localhost:8081/api/v1/products \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"name":"Laptop","description":"Gaming Laptop","price":999.99,"stock":50}' | \
  grep -o '"id":"[^"]*"' | cut -d'"' -f4)
echo "Product ID: $PRODUCT_ID"
```

### Step 4: Get User ID

The user ID is returned in the login response. You can extract it from Step 1, or use this one-liner:

```bash
export USER_ID=$(curl -s -X POST http://localhost:8082/api/v1/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"admin123"}' | \
  grep -o '"id":"[^"]*"' | head -1 | cut -d'"' -f4)
echo "User ID: $USER_ID"
```

Or if you saved the login response, copy the `id` field:
```bash
export USER_ID="<user-id-from-login-response>"
```

### Step 5: Create an Order (Triggers `order_created` Kafka Event)

```bash
curl -s -X POST http://localhost:8080/api/v1/orders \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d "{\"userId\":\"$USER_ID\",\"items\":[{\"productId\":\"$PRODUCT_ID\",\"quantity\":2}]}"
```

**Expected Response:**
```json
{"id":"<order-id>","status":"PENDING"}
```

Save the order ID:
```bash
export ORDER_ID="<order-id>"
```

Or use this one-liner:
```bash
export ORDER_ID=$(curl -s -X POST http://localhost:8080/api/v1/orders \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d "{\"userId\":\"$USER_ID\",\"items\":[{\"productId\":\"$PRODUCT_ID\",\"quantity\":2}]}" | \
  grep -o '"id":"[^"]*"' | cut -d'"' -f4)
echo "Order ID: $ORDER_ID"
```

### Step 6: Pay the Order (Triggers `order_paid` Kafka Event)

```bash
curl -s -X POST "http://localhost:8080/api/v1/orders/$ORDER_ID/pay" \
  -H "Authorization: Bearer $TOKEN"
```

**Expected Response:**
```json
{"id":"<order-id>","status":"PAID"}
```

### Step 7: Create and Cancel Another Order (Triggers `order_cancelled` Event)

```bash
# Create a new order
NEW_ORDER=$(curl -s -X POST http://localhost:8080/api/v1/orders \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d "{\"userId\":\"$USER_ID\",\"items\":[{\"productId\":\"$PRODUCT_ID\",\"quantity\":1}]}" | \
  grep -o '"id":"[^"]*"' | cut -d'"' -f4)
echo "New Order ID: $NEW_ORDER"

# Cancel it
curl -s -X POST "http://localhost:8080/api/v1/orders/$NEW_ORDER/cancel" \
  -H "Authorization: Bearer $TOKEN"
```

**Expected Response:**
```json
{"id":"<order-id>","status":"CANCELLED"}
```

---

## Verify Kafka Events

### Check Order Service Logs

```bash
docker logs order_service 2>&1 | grep -E "KAFKA|order_"
```

**Expected Output:**
```
Kafka producer initialized for topic: order-events
Kafka consumer started for topic: order-events
Kafka: message sent to topic order-events with key: order_created
>>> KAFKA EVENT RECEIVED: [order_created] -> ...
Kafka: message sent to topic order-events with key: order_paid
>>> KAFKA EVENT RECEIVED: [order_paid] -> ...
Kafka: message sent to topic order-events with key: order_cancelled
>>> KAFKA EVENT RECEIVED: [order_cancelled] -> ...
```

### Read Messages Directly from Kafka Topic

```bash
docker exec kafka kafka-console-consumer \
  --bootstrap-server localhost:9092 \
  --topic order-events \
  --from-beginning \
  --max-messages 10
```

### List Kafka Topics

```bash
docker exec kafka kafka-topics --bootstrap-server localhost:9092 --list
```

---

## Complete One-Liner Test Script

Run this entire block to test everything at once:

```bash
# Login and set token + user ID
LOGIN_RESP=$(curl -s -X POST http://localhost:8082/api/v1/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"admin123"}')
export TOKEN=$(echo $LOGIN_RESP | grep -o '"token":"[^"]*"' | cut -d'"' -f4)
export USER_ID=$(echo $LOGIN_RESP | grep -o '"id":"[^"]*"' | head -1 | cut -d'"' -f4)
echo "✓ Logged in (User ID: $USER_ID)"

# Create product
export PRODUCT_ID=$(curl -s -X POST http://localhost:8081/api/v1/products \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"name":"Test Product","description":"Test","price":49.99,"stock":100}' | \
  grep -o '"id":"[^"]*"' | cut -d'"' -f4)
echo "✓ Created product: $PRODUCT_ID"

# Create order
export ORDER_ID=$(curl -s -X POST http://localhost:8080/api/v1/orders \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d "{\"userId\":\"$USER_ID\",\"items\":[{\"productId\":\"$PRODUCT_ID\",\"quantity\":2}]}" | \
  grep -o '"id":"[^"]*"' | cut -d'"' -f4)
echo "✓ Created order: $ORDER_ID"

# Pay order
curl -s -X POST "http://localhost:8080/api/v1/orders/$ORDER_ID/pay" \
  -H "Authorization: Bearer $TOKEN" > /dev/null
echo "✓ Paid order: $ORDER_ID"

# Create and cancel another order
export ORDER_ID2=$(curl -s -X POST http://localhost:8080/api/v1/orders \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d "{\"userId\":\"$USER_ID\",\"items\":[{\"productId\":\"$PRODUCT_ID\",\"quantity\":1}]}" | \
  grep -o '"id":"[^"]*"' | cut -d'"' -f4)
curl -s -X POST "http://localhost:8080/api/v1/orders/$ORDER_ID2/cancel" \
  -H "Authorization: Bearer $TOKEN" > /dev/null
echo "✓ Created and cancelled order: $ORDER_ID2"

echo ""
echo "=== Kafka Events ==="
docker logs order_service 2>&1 | grep ">>> KAFKA EVENT" | tail -5
```

---

## Kafka Event System

### Events Published

| Event Type | Trigger | Payload |
|------------|---------|---------|
| `order_created` | New order placed | `orderId`, `userId`, `totalAmount`, `items` |
| `order_paid` | Order payment confirmed | `orderId`, `userId`, `totalAmount` |
| `order_cancelled` | Order cancelled | `orderId` |

### Configuration

| Environment Variable | Default | Description |
|---------------------|---------|-------------|
| `KAFKA_BROKERS` | `kafka:9092` | Kafka broker addresses |
| `KAFKA_TOPIC` | `order-events` | Topic for order events |
| `KAFKA_GROUP_ID` | `order-service-group` | Consumer group ID |

---

## Stop Services

```bash
docker compose down -v
```

---

## Keploy Recording

To run Keploy record mode for order service:

```bash
keploy record -c "docker compose up" --container-name="order_service" --build-delay 40 --path="./order_service" --config-path="./order_service"
```

Wait for services to start, then run the test script:

```bash
python3 -m venv venv
source venv/bin/activate
pip install requests
python3 test_api_script.py
```

This will record all test cases in `order_service/keploy` folder.

