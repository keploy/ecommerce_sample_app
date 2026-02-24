make this perfect

# 📦 E-commerce Microservices

**Tech Stack:** Python • Flask • MySQL • Docker • LocalStack (SQS) • Keploy

This repository implements a **microservices-based e-commerce backend** built using **Flask**.  
Each service runs independently with its own database and communicates via **REST APIs** and **AWS SQS (LocalStack)** for event-driven workflows.

---

## 🏗️ Architecture Overview

### Services

| Service | Responsibility |
|---|---|
| API Gateway | Entry point & request routing |
| Order Service | Order lifecycle management |
| Product Service | Product catalog & inventory |
| User Service | User management |

### Infrastructure

- **MySQL** → Separate database per service  
- **LocalStack SQS** → Messaging queue simulation  
- **Docker Compose** → Service orchestration  

Architecture documentation:


docs/architecture.md


---

## 📁 Project Structure


apigateway/

├── app.py

├── entrypoint.sh

└── openapi.yaml


order_service/

├── app.py

├── db.sql

├── entrypoint.sh

├── keploy.yml

├── migrate.py

├── openapi.yaml

├── requirements.txt

├── migrations/

│ ├── 0001_init.sql

│ └── 0002_shipping_address_id.sql

└── keploy/

├── ai/tests/test1.yaml → test36.yaml

└── manual/tests/


product_service/

├── app.py

├── db.sql

├── entrypoint.sh

├── keploy.yml

├── migrate.py

├── openapi.yaml

├── requirements.txt

├── migrations/

│ ├── 0001_init.sql

│ └── 0002_shipping_address_id.sql

└── keploy/

├── ai/tests/test1.yaml → test36.yaml

└── manual/tests/

user_service/
├── app.py

├── db.sql

├── entrypoint.sh

├── keploy.yml

├── migrate.py

├── openapi.yaml

├── requirements.txt

├── migrations/

│ ├── 0001_init.sql

│ └── 0002_shipping_address_id.sql

└── keploy/

├── ai/tests/test1.yaml → test36.yaml

└── manual/tests/

localstack/

├── 01-create-queues.sh

└── 01-queues.sh

postman/

├── collection.gateway.json

├── collection.microservices.json

└── environment/local.json


coverage/

├── .coverage.apigateway.combined

├── .coverage.order_service.combined

├── .coverage.product_service.combined

└── .coverage.user_service.combined



---

## 🚀 Quick Start

### 1️⃣ Start Services
```bash
docker compose up -d --build
```
**2️⃣ View Logs**
```bash
docker compose logs -f apigateway order_service product_service user_service
```
**3️⃣ Stop Services**
```bash
docker compose down -v
```
**🔌 Services Description**

API Gateway

Location: apigateway/

**Responsibilities:**

- Routes external requests to services

- Aggregates responses when required

- Provides OpenAPI documentation

**Order Service**

Location: order_service/

**Responsibilities:**

- Create orders

- Validate users via user_service

- Validate products via product_service

- Reserve/release stock

**Persist orders**

- Publish SQS events

- Order Status Flow
- PENDING → PAID
- PENDING → CANCELLED (releases stock)
- Idempotency

**Uses header:**

- Idempotency-Key

- Prevents duplicate order creation.

- Product Service

**Location**: product_service/

Responsibilities:

- Product CRUD

- Stock management

- Stock reservation for orders

- User Service

- Location: user_service/

**Responsibilities:**
- User CRUD

- User validation for orders

**🗄️ Database**

Each service has:

- db.sql
- migrations/
- migrate.py

Migrations run automatically via container entrypoint.

**📨 Messaging (SQS via LocalStack)**

Setup scripts:

localstack/01-create-queues.sh
localstack/01-queues.sh

Used for:

- Order events

- Async processing

**🧪 Testing (Keploy)**

Each service includes:

keploy/
  ai/tests/
  manual/tests/
Test Cases

**AI-generated tests:**

- test1.yaml
- Coverage Includes
- Order Service Tests

- Create order success

- Duplicate order prevention

- Invalid user

- Invalid product

- Stock unavailable

- Order cancellation

- Order payment

- Idempotency validation

- Product Service Tests

- Create product

- Update product

- Delete product

- Stock updates

- Reserve stock

- Release stock

**User Service Tests**

- Create user

- Update user

- Delete user

- Fetch user

- Invalid user scenarios

**Gateway Tests**

- Routing validation

- Aggregated responses

- Error propagation

- Edge Cases

- Invalid payload

- Missing headers

- Service unavailable

- Database failures

**📬 API Documentation**

Each service exposes:

- openapi.yaml

**Postman collections:**

postman/
Environment:
environment/local.json
**📊 Coverage Reports**

Coverage stored in:
coverage/
- Contains combined coverage per service.

**🛠️ Development Workflow**
- Adding a New Endpoint

- Update app.py

- Update openapi.yaml

- Add DB migration if needed

- Add Keploy tests

- Update Postman collection

- Run services locally

- Submit PR

**🤝 Contributing**

- Fork repository

- Create feature branch

- Commit changes

- Add/update tests (test1 → test36 style)

- Ensure services run

- Create Pull Request

**⚡ Order Creation Flow**

Validate user

Validate products

Reserve stock

Save order

Publish SQS event

**📜 License**

Open source — follow repository license.
