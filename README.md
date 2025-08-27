# E-commerce Microservices (Python, Flask, MySQL, SQS)

Services

- order_service (A): REST + MySQL + publishes SQS order events
- product_service (B): REST + MySQL (catalog)
- user_service (C): REST + MySQL (users)

Infra

- 3x MySQL containers
- LocalStack (SQS)
- Docker Compose to orchestrate

Quick start

- docker compose up -d --build
- docker compose logs -f order_service product_service user_service
- docker compose down -v

APIs

- OpenAPI specs in each service under openapi.yaml

# E-commerce Microservices Architecture

This repo contains three Python (Flask) microservices orchestrated with Docker Compose, each with its own MySQL database, plus LocalStack SQS for messaging.

## Architecture (single diagram)

```mermaid
flowchart LR
  client([Client / Postman])

  subgraph "Order Service :8080"
    order_svc([Order Service])
    o1["POST /api/v1/orders"]
    o2["GET /api/v1/orders"]
    o3["GET /api/v1/orders/{id}"]
    o4["GET /api/v1/orders/{id}/details"]
    o5["POST /api/v1/orders/{id}/pay"]
    o6["POST /api/v1/orders/{id}/cancel"]
    o7["GET /health"]
    order_svc --- o1
    order_svc --- o2
    order_svc --- o3
    order_svc --- o4
    order_svc --- o5
    order_svc --- o6
    order_svc --- o7
  end

  subgraph "Product Service :8081"
    product_svc([Product Service])
    p1["CRUD /api/v1/products"]
    p2["POST /api/v1/products/{id}/reserve"]
    p3["POST /api/v1/products/{id}/release"]
    p4["GET /api/v1/products/search"]
    p5["GET /health"]
    product_svc --- p1
    product_svc --- p2
    product_svc --- p3
    product_svc --- p4
    product_svc --- p5
  end

  subgraph "User Service :8082"
    user_svc([User Service])
    u1["POST /api/v1/users"]
    u2["GET /api/v1/users/{id}"]
    u3["POST /api/v1/login"]
    u4["GET /health"]
    user_svc --- u1
    user_svc --- u2
    user_svc --- u3
    user_svc --- u4
  end

  subgraph Datastores
    dborders[(MySQL order_db)]
    dbproducts[(MySQL product_db)]
    dbusers[(MySQL user_db)]
  end

  sqs[(LocalStack SQS: order-events)]

  client --> order_svc
  client --> product_svc
  client --> user_svc

  order_svc --> dborders
  product_svc --> dbproducts
  user_svc --> dbusers

  order_svc --> user_svc
  order_svc --> product_svc

  order_svc --> sqs
```

Key behaviors

- Order creation validates user and products, reserves stock, persists order + items, emits SQS event.
- Idempotency: POST /orders supports Idempotency-Key to avoid duplicate orders.
- Status transitions: PENDING → PAID or CANCELLED (cancel releases stock).
