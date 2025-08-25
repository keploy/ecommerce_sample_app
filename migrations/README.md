Manual DB migration notes:

- user_db

  - ALTER TABLE users ADD COLUMN phone VARCHAR(32) NULL; (safe if exists)
  - CREATE TABLE addresses (...)

- order_db
  - ALTER TABLE orders ADD COLUMN shipping_address JSON NULL; (safe if exists)

If using Docker Compose service names:

- Users DB container: microservices-mysql-users-1
- Orders DB container: microservices-mysql-orders-1

Run examples (optional):

- docker exec -i microservices-mysql-users-1 mysql -uroot -proot -e "ALTER TABLE user_db.users ADD COLUMN phone VARCHAR(32) NULL;"
- docker exec -i microservices-mysql-orders-1 mysql -uroot -proot -e "ALTER TABLE order_db.orders ADD COLUMN shipping_address JSON NULL;"
