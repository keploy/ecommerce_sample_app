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
    CONSTRAINT chk_price_nonneg CHECK (price >= 0),
    CONSTRAINT chk_stock_nonneg CHECK (stock >= 0)
);

-- Conditionally create index if missing (MySQL-safe)
SET @idx_exists = (
    SELECT COUNT(1) FROM INFORMATION_SCHEMA.STATISTICS
    WHERE TABLE_SCHEMA = DATABASE()
        AND TABLE_NAME = 'products'
        AND INDEX_NAME = 'idx_products_name'
);
SET @create_idx = IF(@idx_exists > 0, 'SELECT 1', 'CREATE INDEX idx_products_name ON products(name)');
PREPARE stmt FROM @create_idx;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;
