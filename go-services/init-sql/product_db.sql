CREATE DATABASE IF NOT EXISTS product_db;
USE product_db;

CREATE TABLE IF NOT EXISTS products (
    id VARCHAR(36) PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    price DECIMAL(10, 2) NOT NULL,
    stock INT NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    INDEX idx_products_name (name),
    CONSTRAINT chk_price_nonneg CHECK (price >= 0),
    CONSTRAINT chk_stock_nonneg CHECK (stock >= 0)
);

INSERT INTO products (id, name, description, price, stock) VALUES
(UUID(), 'Laptop', 'A powerful and portable laptop.', 1200.00, 50),
(UUID(), 'Mouse', 'An ergonomic wireless mouse.', 25.50, 200);
