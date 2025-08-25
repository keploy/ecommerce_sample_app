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

## Component view

```mermaid
flowchart LR
    subgraph Clients
      client[Client / Postman]
    end

    subgraph Services
      order[Order Service\n:8080]
      product[Product Service\n:8081]
      user[User Service\n:8082]
    end

    subgraph Datastores
      dborders[(MySQL\norder_db)]
      dbproducts[(MySQL\nproduct_db)]
      dbusers[(MySQL\nuser_db)]
      sqs[(LocalStack SQS\nqueue: order-events)]
    end

    client -->|HTTP| order
    client -->|HTTP| product
    client -->|HTTP| user

  order -->|GET /users/:id| user
  order -->|GET /products/:id| product
  order -->|POST /products/:id/reserve| product
  order -->|POST /products/:id/release| product

    order -->|MySQL| dborders
    product -->|MySQL| dbproducts
    user -->|MySQL| dbusers

    order -->|SQS send\norder_created| sqs
    order -->|SQS send\norder_paid| sqs
    order -->|SQS send\norder_cancelled| sqs
```

Key behaviors

- Order creation validates user and products, reserves stock, persists order + items, emits SQS event.
- Idempotency: POST /orders supports Idempotency-Key to avoid duplicate orders.
- Status transitions: PENDING → PAID or CANCELLED (cancel releases stock).

## Sequence: Place Order

```mermaid
sequenceDiagram
    autonumber
    participant C as Client
    participant O as Order Service
    participant U as User Service
    participant P as Product Service
    participant D as Orders DB
    participant Q as SQS

    C->>O: POST /orders (Idempotency-Key)
    O->>D: SELECT by idempotency_key
    alt Existing order
      O-->>C: 200 (id, status)
    else New order
      O->>U: GET /users/:userId
      O->>P: GET /products/:id (each item)
      O->>P: POST /products/:id/reserve (each item)
      O->>D: INSERT orders, order_items (with total_amount)
      O->>Q: Send message (eventType: order_created, ...)
      O-->>C: 201 (id, status: PENDING)
    end
```

## Sequence: Pay / Cancel Order

```mermaid
sequenceDiagram
    autonumber
    participant C as Client
    participant O as Order Service
    participant P as Product Service
    participant D as Orders DB
    participant Q as SQS

    rect rgb(235, 255, 235)
  C->>O: POST /orders/:id/pay
    O->>D: SELECT status FOR UPDATE
    alt Already PAID
      O-->>C: 200 PAID
    else Cancelled
      O-->>C: 409 Cannot pay cancelled
    else Pending
      O->>D: UPDATE status=PAID
      O->>Q: Send order_paid
      O-->>C: 200 PAID
    end
    end

    rect rgb(255, 240, 240)
  C->>O: POST /orders/:id/cancel
    O->>D: SELECT status FOR UPDATE
    alt Already CANCELLED
      O-->>C: 200 CANCELLED
    else PAID
      O-->>C: 409 Cannot cancel paid
    else Pending
      O->>D: SELECT items
  O->>P: POST /products/:id/release (each item)
      O->>D: UPDATE status=CANCELLED
      O->>Q: Send order_cancelled
      O-->>C: 200 CANCELLED
    end
    end
```

## Data model (simplified)

- users: id, username, email, password_hash, created_at, updated_at
- products: id, name, description, price, stock, created_at, updated_at
- orders: id, user_id, status(PENDING|PAID|CANCELLED), idempotency_key(uniq), total_amount, created_at, updated_at
- order_items: id, order_id, product_id, quantity, price

## Ports and endpoints

- User Service (8082): POST /api/v1/users, GET /api/v1/users/{id}, POST /api/v1/login, GET /health
- Product Service (8081): CRUD /api/v1/products, reserve/release, search, GET /health
- Order Service (8080): POST/GET /api/v1/orders, GET /api/v1/orders/{id}, POST /api/v1/orders/{id}/pay|cancel, GET /health

## Messaging

- LocalStack SQS (4566) with queue order-events.
- Events: order_created, order_paid, order_cancelled (JSON payloads).

## Notes

- Docker Compose brings up three MySQL instances, three services, and LocalStack SQS.
- Idempotency check is performed before external calls to avoid double reservation on retries.

