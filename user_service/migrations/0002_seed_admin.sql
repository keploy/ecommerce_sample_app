-- Seed default admin user (admin / admin123)
INSERT INTO users (id, username, email, password_hash)
VALUES (
  UUID(),
  'admin',
  'admin@example.com',
  'pbkdf2:sha256:600000$nBh786TvaYCbJPcQ$b1429e79b521a37444b162e4526d576a3acd87525c7f7413e5f94108c775815c'
)
ON DUPLICATE KEY UPDATE username = username;
