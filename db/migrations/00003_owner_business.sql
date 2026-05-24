-- +goose Up
ALTER TABLE menu_items ADD COLUMN sku TEXT NOT NULL DEFAULT '';
ALTER TABLE menu_items ADD COLUMN item_type TEXT NOT NULL DEFAULT 'food';
ALTER TABLE menu_items ADD COLUMN cost_cents INTEGER NOT NULL DEFAULT 0;
ALTER TABLE menu_items ADD COLUMN sort_order INTEGER NOT NULL DEFAULT 0;

CREATE TABLE IF NOT EXISTS inventory (
  product_id INTEGER PRIMARY KEY REFERENCES menu_items(id) ON DELETE CASCADE,
  stock_qty REAL NOT NULL DEFAULT 0,
  reorder_level REAL NOT NULL DEFAULT 0,
  unit TEXT NOT NULL DEFAULT 'pcs',
  track_stock INTEGER NOT NULL DEFAULT 1,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS inventory_movements (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  product_id INTEGER NOT NULL REFERENCES menu_items(id) ON DELETE CASCADE,
  change_qty REAL NOT NULL,
  reason TEXT NOT NULL DEFAULT 'manual_adjustment',
  reference TEXT NOT NULL DEFAULT '',
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS settings (
  id INTEGER PRIMARY KEY CHECK (id = 1),
  business_name TEXT NOT NULL DEFAULT 'MauzoHub',
  business_type TEXT NOT NULL DEFAULT 'Restaurant',
  phone TEXT NOT NULL DEFAULT '',
  currency_code TEXT NOT NULL DEFAULT 'KES',
  mpesa_till TEXT NOT NULL DEFAULT '',
  receipt_footer TEXT NOT NULL DEFAULT 'Thank you for your purchase.',
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

INSERT OR IGNORE INTO settings (id, business_name, business_type, receipt_footer)
VALUES (1, 'MauzoHub Demo', 'Cafe', 'Thank you for shopping with us.');

INSERT OR IGNORE INTO inventory (product_id, stock_qty, reorder_level, unit, track_stock)
SELECT id, 25, 5, 'pcs', 1
FROM menu_items;

UPDATE menu_items
SET
  item_type = CASE
    WHEN lower(name) LIKE '%juice%' OR lower(name) LIKE '%tea%' OR lower(name) LIKE '%coffee%' THEN 'drink'
    ELSE 'food'
  END,
  cost_cents = CASE
    WHEN cost_cents = 0 THEN CAST(price_cents * 0.45 AS INTEGER)
    ELSE cost_cents
  END,
  sort_order = CASE
    WHEN sort_order = 0 THEN id
    ELSE sort_order
  END;

INSERT OR IGNORE INTO users (name, email, password_hash, role)
VALUES ('Shop Owner', 'owner@mauzohub.local', '$2a$10$i5FMyRLM8BqOtXCC9QajmeNyiKSOYtp07W4jAtX3pPZnt6M7z5tMy', 'admin');

CREATE INDEX IF NOT EXISTS idx_inventory_track_stock ON inventory(track_stock, stock_qty, reorder_level);
CREATE INDEX IF NOT EXISTS idx_inventory_movements_product_created ON inventory_movements(product_id, created_at DESC);

-- +goose Down
DROP INDEX IF EXISTS idx_inventory_movements_product_created;
DROP INDEX IF EXISTS idx_inventory_track_stock;
DELETE FROM users WHERE email = 'owner@mauzohub.local';
DROP TABLE IF EXISTS settings;
DROP TABLE IF EXISTS inventory_movements;
DROP TABLE IF EXISTS inventory;
