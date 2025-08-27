-- Add shipping_address_id column and (optionally) drop legacy JSON column
USE order_db;

-- Add column if not exists
SET @col_exists = (
  SELECT COUNT(1) FROM INFORMATION_SCHEMA.COLUMNS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'orders' AND COLUMN_NAME = 'shipping_address_id'
);
SET @stmt = IF(@col_exists > 0, 'SELECT 1', 'ALTER TABLE orders ADD COLUMN shipping_address_id VARCHAR(36) NULL AFTER idempotency_key');
PREPARE s FROM @stmt; EXECUTE s; DEALLOCATE PREPARE s;

-- Drop old shipping_address JSON column if present
SET @old_exists = (
  SELECT COUNT(1) FROM INFORMATION_SCHEMA.COLUMNS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'orders' AND COLUMN_NAME = 'shipping_address'
);
SET @stmt = IF(@old_exists = 0, 'SELECT 1', 'ALTER TABLE orders DROP COLUMN shipping_address');
PREPARE s FROM @stmt; EXECUTE s; DEALLOCATE PREPARE s;

-- Optional index to query by shipping address
SET @idx_exists = (
  SELECT COUNT(1) FROM INFORMATION_SCHEMA.STATISTICS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'orders' AND INDEX_NAME = 'idx_orders_shipaddr'
);
SET @stmt = IF(@idx_exists > 0, 'SELECT 1', 'CREATE INDEX idx_orders_shipaddr ON orders(shipping_address_id)');
PREPARE s FROM @stmt; EXECUTE s; DEALLOCATE PREPARE s;
