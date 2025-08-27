CREATE DATABASE IF NOT EXISTS order_db;
USE order_db;

CREATE TABLE IF NOT EXISTS orders (
    id VARCHAR(36) PRIMARY KEY,
    user_id VARCHAR(36) NOT NULL,
    status ENUM('PENDING','PAID','CANCELLED') NOT NULL DEFAULT 'PENDING',
    idempotency_key VARCHAR(64) NULL,
    shipping_address_id VARCHAR(36) NULL,
    total_amount DECIMAL(12, 2) NOT NULL DEFAULT 0,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    UNIQUE KEY uq_orders_idmp (idempotency_key)
);

CREATE TABLE IF NOT EXISTS order_items (
    id INT AUTO_INCREMENT PRIMARY KEY,
    order_id VARCHAR(36) NOT NULL,
    product_id VARCHAR(36) NOT NULL,
    quantity INT NOT NULL,
    price DECIMAL(10, 2) NOT NULL,
    FOREIGN KEY (order_id) REFERENCES orders(id),
    CONSTRAINT chk_qty_pos CHECK (quantity > 0),
    CONSTRAINT chk_price_nonneg CHECK (price >= 0)
);

-- Conditionally create indexes (MySQL-safe)
-- idx_orders_user
SET @idx_exists = (
    SELECT COUNT(1) FROM INFORMATION_SCHEMA.STATISTICS
    WHERE TABLE_SCHEMA = DATABASE()
        AND TABLE_NAME = 'orders'
        AND INDEX_NAME = 'idx_orders_user'
);
SET @stmt = IF(@idx_exists > 0, 'SELECT 1', 'CREATE INDEX idx_orders_user ON orders(user_id)');
PREPARE s FROM @stmt; EXECUTE s; DEALLOCATE PREPARE s;

-- idx_orders_status
SET @idx_exists = (
    SELECT COUNT(1) FROM INFORMATION_SCHEMA.STATISTICS
    WHERE TABLE_SCHEMA = DATABASE()
        AND TABLE_NAME = 'orders'
        AND INDEX_NAME = 'idx_orders_status'
);
SET @stmt = IF(@idx_exists > 0, 'SELECT 1', 'CREATE INDEX idx_orders_status ON orders(status)');
PREPARE s FROM @stmt; EXECUTE s; DEALLOCATE PREPARE s;

-- idx_orders_created
SET @idx_exists = (
    SELECT COUNT(1) FROM INFORMATION_SCHEMA.STATISTICS
    WHERE TABLE_SCHEMA = DATABASE()
        AND TABLE_NAME = 'orders'
        AND INDEX_NAME = 'idx_orders_created'
);
SET @stmt = IF(@idx_exists > 0, 'SELECT 1', 'CREATE INDEX idx_orders_created ON orders(created_at)');
PREPARE s FROM @stmt; EXECUTE s; DEALLOCATE PREPARE s;

-- idx_orders_shipaddr
SET @idx_exists = (
    SELECT COUNT(1) FROM INFORMATION_SCHEMA.STATISTICS
    WHERE TABLE_SCHEMA = DATABASE()
        AND TABLE_NAME = 'orders'
        AND INDEX_NAME = 'idx_orders_shipaddr'
);
SET @stmt = IF(@idx_exists > 0, 'SELECT 1', 'CREATE INDEX idx_orders_shipaddr ON orders(shipping_address_id)');
PREPARE s FROM @stmt; EXECUTE s; DEALLOCATE PREPARE s;

-- idx_order_items_order
SET @idx_exists = (
    SELECT COUNT(1) FROM INFORMATION_SCHEMA.STATISTICS
    WHERE TABLE_SCHEMA = DATABASE()
        AND TABLE_NAME = 'order_items'
        AND INDEX_NAME = 'idx_order_items_order'
);
SET @stmt = IF(@idx_exists > 0, 'SELECT 1', 'CREATE INDEX idx_order_items_order ON order_items(order_id)');
PREPARE s FROM @stmt; EXECUTE s; DEALLOCATE PREPARE s;
