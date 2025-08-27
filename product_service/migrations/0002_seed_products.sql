USE product_db;

-- Seed initial products if they don't already exist (idempotent)
INSERT INTO products (id, name, description, price, stock)
SELECT '11111111-1111-4111-8111-111111111111', 'Laptop', 'A powerful and portable laptop.', 1200.00, 50
FROM DUAL
WHERE NOT EXISTS (SELECT 1 FROM products WHERE name = 'Laptop')
LIMIT 1;

INSERT INTO products (id, name, description, price, stock)
SELECT '22222222-2222-4222-8222-222222222222', 'Mouse', 'An ergonomic wireless mouse.', 25.50, 200
FROM DUAL
WHERE NOT EXISTS (SELECT 1 FROM products WHERE name = 'Mouse')
LIMIT 1;

INSERT INTO products (id, name, description, price, stock)
SELECT '33333333-3333-4333-8333-333333333333', 'Keyboard', 'A compact mechanical keyboard.', 75.00, 120
FROM DUAL
WHERE NOT EXISTS (SELECT 1 FROM products WHERE name = 'Keyboard')
LIMIT 1;
