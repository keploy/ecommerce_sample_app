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
