-- +goose Up
ALTER TABLE restaurant_tables ADD COLUMN slug TEXT;
ALTER TABLE restaurant_tables ADD COLUMN active INTEGER NOT NULL DEFAULT 1;

ALTER TABLE orders ADD COLUMN session_id INTEGER;
ALTER TABLE orders ADD COLUMN payment_status TEXT NOT NULL DEFAULT 'unpaid';

ALTER TABLE payments ADD COLUMN phone_number TEXT NOT NULL DEFAULT '';
ALTER TABLE payments ADD COLUMN provider TEXT NOT NULL DEFAULT '';
ALTER TABLE payments ADD COLUMN metadata_json TEXT NOT NULL DEFAULT '{}';
ALTER TABLE payments ADD COLUMN confirmed_at DATETIME;

CREATE TABLE IF NOT EXISTS qr_codes (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  table_id INTEGER NOT NULL UNIQUE REFERENCES restaurant_tables(id) ON DELETE CASCADE,
  public_url TEXT NOT NULL DEFAULT '',
  image_data TEXT NOT NULL DEFAULT '',
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS sessions (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  table_id INTEGER NOT NULL REFERENCES restaurant_tables(id) ON DELETE CASCADE,
  token TEXT NOT NULL UNIQUE,
  customer_name TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active','closed')),
  started_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  last_seen_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  ended_at DATETIME
);

CREATE TABLE IF NOT EXISTS receipts (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  order_id INTEGER NOT NULL UNIQUE REFERENCES orders(id) ON DELETE CASCADE,
  payment_id INTEGER REFERENCES payments(id) ON DELETE SET NULL,
  receipt_number TEXT NOT NULL UNIQUE,
  payload_json TEXT NOT NULL DEFAULT '{}',
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

UPDATE restaurant_tables
SET slug = CASE
  WHEN number GLOB '[0-9]*' THEN 'table-' || number
  ELSE 'table-' || lower(replace(trim(number), ' ', '-'))
END
WHERE COALESCE(slug, '') = '';

INSERT OR IGNORE INTO qr_codes (table_id)
SELECT id FROM restaurant_tables;

UPDATE orders
SET payment_status = CASE
  WHEN status = 'paid' THEN 'paid'
  ELSE 'unpaid'
END
WHERE COALESCE(payment_status, '') = '';

INSERT OR IGNORE INTO users (name, email, password_hash, role) VALUES
('Chef User', 'chef@qrdine.local', '$2a$10$i5FMyRLM8BqOtXCC9QajmeNyiKSOYtp07W4jAtX3pPZnt6M7z5tMy', 'chef'),
('Waiter User', 'waiter@qrdine.local', '$2a$10$i5FMyRLM8BqOtXCC9QajmeNyiKSOYtp07W4jAtX3pPZnt6M7z5tMy', 'waiter'),
('Cashier User', 'cashier@qrdine.local', '$2a$10$i5FMyRLM8BqOtXCC9QajmeNyiKSOYtp07W4jAtX3pPZnt6M7z5tMy', 'cashier');

CREATE INDEX IF NOT EXISTS idx_sessions_table_status ON sessions(table_id, status, started_at DESC);
CREATE INDEX IF NOT EXISTS idx_orders_session ON orders(session_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_payments_status_created ON payments(status, created_at DESC);

-- +goose Down
DROP INDEX IF EXISTS idx_payments_status_created;
DROP INDEX IF EXISTS idx_orders_session;
DROP INDEX IF EXISTS idx_sessions_table_status;

DELETE FROM users WHERE email IN ('chef@qrdine.local', 'waiter@qrdine.local', 'cashier@qrdine.local');

DROP TABLE IF EXISTS receipts;
DROP TABLE IF EXISTS sessions;
DROP TABLE IF EXISTS qr_codes;
