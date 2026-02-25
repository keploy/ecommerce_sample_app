# E-Commerce Microservices

[![Python 3.11](https://img.shields.io/badge/Python-3.11-3776AB?logo=python&logoColor=white)](https://python.org)
[![Flask](https://img.shields.io/badge/Flask-3.0-000000?logo=flask&logoColor=white)](https://flask.palletsprojects.com)
[![Docker](https://img.shields.io/badge/Docker-Compose-2496ED?logo=docker&logoColor=white)](https://docker.com)
[![Keploy](https://img.shields.io/badge/Tested_with-Keploy-7C3AED)](https://keploy.io)

A Python/Flask e-commerce backend using microservices, each with its own MySQL database, an API Gateway, and SQS event messaging via LocalStack - all wired together with Docker Compose.

---

## Services

| Service | Port | Responsibility |
|---|---|---|
| API Gateway | `8083` | Single entry point - proxies all client requests |
| Order Service | `8080` | Order lifecycle, stock reservation, SQS events |
| Product Service | `8081` | Product catalog, inventory management |
| User Service | `8082` | Auth, JWT issuance, user accounts & addresses |
| LocalStack (SQS) | `4566` | Local AWS SQS - `order-events` queue |

---

## Quick Start

**Requires:** Docker Desktop (includes Compose)

```bash
git clone https://github.com/keploy/ecommerce_sample_app.git
cd ecommerce_sample_app
docker compose up -d --build
```

That is it. On first boot the stack will:
- Run all DB migrations automatically
- Seed an admin user: `admin` / `admin123`
- Seed sample products (Laptop, Mouse)
- Create the `order-events` SQS queue

**Health check:**
```bash
curl http://localhost:8083/health
```

**Logs:**
```bash
docker compose logs -f
```

**Teardown:**
```bash
docker compose down -v   # -v removes data volumes
```

---

## API

All requests go through the gateway at `http://localhost:8083`. Every endpoint except `/api/v1/login` and `/health` requires `Authorization: Bearer <token>`.

**Get a token:**
```bash
curl -s -X POST http://localhost:8083/api/v1/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"admin123"}'
```

| Service | Endpoints |
|---|---|
| **User** | `POST /api/v1/login` , `POST /api/v1/users` , `GET /api/v1/users/{id}` , `GET/POST /api/v1/users/{id}/addresses` |
| **Product** | `GET/POST /api/v1/products` , `GET/PUT/DELETE /api/v1/products/{id}` , `GET /api/v1/products/search` , `POST /api/v1/products/{id}/reserve` , `POST /api/v1/products/{id}/release` |
| **Order** | `GET/POST /api/v1/orders` , `GET /api/v1/orders/{id}` , `GET /api/v1/orders/{id}/details` , `POST /api/v1/orders/{id}/pay` , `POST /api/v1/orders/{id}/cancel` |

Full OpenAPI specs: each service has an `openapi.yaml`. Import `postman/collection.gateway.json` + `postman/environment.local.json` for a ready-to-run Postman setup.

---

## Architecture

```
Client --> API Gateway (:8083)
            |-- /users/*    --> User Service (:8082)    --> mysql-users (:3307)
            |-- /products/* --> Product Service (:8081) --> mysql-products (:3308)
            |-- /orders/*   --> Order Service (:8080)   --> mysql-orders (:3309)
                                     |
                                     |-- calls User Service    (validate user)
                                     |-- calls Product Service (reserve / release stock)
                                     +-- publishes --> SQS order-events (LocalStack :4566)
```

**Key behaviours:**
- `POST /orders` validates the user, reserves product stock, persists the order, then emits an `ORDER_CREATED` event to SQS.
- Supports `Idempotency-Key` header on order creation to prevent duplicates.
- Cancelling an order releases reserved stock and emits `ORDER_CANCELLED`.
- JWT (`HS256`) is issued by the User Service and verified by all services via a shared `JWT_SECRET`.

---

## Testing

Tests live in `<service>/keploy/` - two suites per service:

| Suite | Description |
|---|---|
| `ai/` | AI-generated tests recorded from live traffic (e.g. 36 tests for `order_service`) |
| `manual/` | Hand-crafted test scenarios (e.g. 14 tests for `order_service`) |

**Run tests (requires [Keploy CLI](https://keploy.io/docs)):**
```bash
docker compose up -d --build
keploy test -c "docker compose up" --container-name order_service
```

Coverage reports are written to `coverage/` during test runs.

---

## Project Layout

```
ecommerce_sample_app/
|-- docker-compose.yml
|-- apigateway/           # Flask reverse proxy
|-- order_service/
|   |-- app.py
|   |-- migrations/       # Versioned SQL files - auto-applied at startup
|   +-- keploy/ai|manual/ # Keploy test suites
|-- product_service/      # Same structure as order_service
|-- user_service/         # Same structure as order_service
|-- localstack/           # Queue init scripts (runs on LocalStack ready)
+-- postman/              # Import-ready Postman collections
```

---

## Configuration

Set in `docker-compose.yml`. Key variables:

| Variable | Default | Note |
|---|---|---|
| `JWT_SECRET` | `dev-secret-change-me` | **Must change in production** |
| `DB_HOST / DB_USER / DB_PASSWORD / DB_NAME` | per-service | Each service owns its own DB |
| `AWS_ENDPOINT` | `http://localstack:4566` | Routes SQS traffic to LocalStack |
| `ADMIN_PASSWORD` | `admin123` | Seeded admin credentials |

---

## Contributing

1. Fork and create a branch (`feat/your-feature`, `fix/issue`, `docs/update`)
2. Make changes - keep services decoupled, update `openapi.yaml` for API changes, add new migration files for schema changes (never edit existing ones)
3. Test: `docker compose up -d --build` then run Keploy tests
4. Commit using [Conventional Commits](https://www.conventionalcommits.org) (`feat:`, `fix:`, `docs:`, `test:`, `refactor:`)
5. Open a Pull Request and reference the related issue
