-- +goose Up
PRAGMA foreign_keys = ON;

CREATE TABLE users (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  name TEXT NOT NULL,
  email TEXT NOT NULL UNIQUE,
  password_hash TEXT NOT NULL,
  role TEXT NOT NULL CHECK (role IN ('admin','manager','chef','cashier','waiter')),
  active INTEGER NOT NULL DEFAULT 1,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE restaurant_tables (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  number TEXT NOT NULL UNIQUE,
  seats INTEGER NOT NULL DEFAULT 4,
  status TEXT NOT NULL DEFAULT 'available' CHECK (status IN ('available','occupied','reserved','cleaning')),
  qr_token TEXT NOT NULL UNIQUE,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE menu_categories (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  name TEXT NOT NULL UNIQUE,
  sort_order INTEGER NOT NULL DEFAULT 0,
  active INTEGER NOT NULL DEFAULT 1
);

CREATE TABLE menu_items (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  category_id INTEGER NOT NULL REFERENCES menu_categories(id) ON DELETE CASCADE,
  name TEXT NOT NULL,
  description TEXT NOT NULL DEFAULT '',
  price_cents INTEGER NOT NULL CHECK (price_cents >= 0),
  image_url TEXT NOT NULL DEFAULT '',
  active INTEGER NOT NULL DEFAULT 1,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE ingredients (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  name TEXT NOT NULL UNIQUE,
  unit TEXT NOT NULL,
  stock_qty REAL NOT NULL DEFAULT 0,
  low_stock_qty REAL NOT NULL DEFAULT 0
);

CREATE TABLE menu_item_ingredients (
  menu_item_id INTEGER NOT NULL REFERENCES menu_items(id) ON DELETE CASCADE,
  ingredient_id INTEGER NOT NULL REFERENCES ingredients(id) ON DELETE CASCADE,
  qty REAL NOT NULL CHECK (qty > 0),
  PRIMARY KEY (menu_item_id, ingredient_id)
);

CREATE TABLE orders (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  table_id INTEGER REFERENCES restaurant_tables(id),
  customer_name TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL DEFAULT 'new' CHECK (status IN ('new','accepted','preparing','ready','served','cancelled','paid')),
  subtotal_cents INTEGER NOT NULL DEFAULT 0,
  vat_cents INTEGER NOT NULL DEFAULT 0,
  total_cents INTEGER NOT NULL DEFAULT 0,
  source TEXT NOT NULL DEFAULT 'qr' CHECK (source IN ('qr','staff')),
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE order_items (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  order_id INTEGER NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
  menu_item_id INTEGER NOT NULL REFERENCES menu_items(id),
  quantity INTEGER NOT NULL CHECK (quantity > 0),
  unit_price_cents INTEGER NOT NULL,
  notes TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL DEFAULT 'queued' CHECK (status IN ('queued','preparing','ready','served','cancelled'))
);

CREATE TABLE payments (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  order_id INTEGER NOT NULL REFERENCES orders(id),
  method TEXT NOT NULL CHECK (method IN ('mpesa','cash','card')),
  amount_cents INTEGER NOT NULL CHECK (amount_cents >= 0),
  status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','paid','failed','refunded')),
  reference TEXT NOT NULL DEFAULT '',
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_orders_status_created ON orders(status, created_at);
CREATE INDEX idx_orders_table ON orders(table_id);
CREATE INDEX idx_order_items_order ON order_items(order_id);
CREATE INDEX idx_menu_items_category ON menu_items(category_id);
CREATE INDEX idx_payments_order ON payments(order_id);

INSERT INTO users (name, email, password_hash, role) VALUES
('Admin User', 'admin@qrdine.local', '$2a$10$i5FMyRLM8BqOtXCC9QajmeNyiKSOYtp07W4jAtX3pPZnt6M7z5tMy', 'admin');

INSERT INTO restaurant_tables (number, seats, qr_token) VALUES
('1', 4, 'table-1-demo'), ('2', 2, 'table-2-demo'), ('3', 6, 'table-3-demo');

INSERT INTO menu_categories (name, sort_order) VALUES
('Breakfast', 1), ('Mains', 2), ('Drinks', 3);

INSERT INTO menu_items (category_id, name, description, price_cents, image_url) VALUES
(1, 'Swahili Omelette', 'Eggs, tomato, coriander, mild chili', 65000, ''),
(2, 'Grilled Chicken Pilau', 'Spiced rice, grilled chicken, kachumbari', 145000, ''),
(2, 'Vegetable Curry Bowl', 'Coconut curry, seasonal vegetables, rice', 110000, ''),
(3, 'Fresh Passion Juice', 'Pressed daily', 35000, '');

INSERT INTO ingredients (name, unit, stock_qty, low_stock_qty) VALUES
('Eggs', 'pcs', 90, 20), ('Chicken', 'kg', 18, 5), ('Rice', 'kg', 40, 8), ('Passion fruit', 'kg', 12, 3);

INSERT INTO menu_item_ingredients (menu_item_id, ingredient_id, qty) VALUES
(1, 1, 2), (2, 2, 0.25), (2, 3, 0.2), (3, 3, 0.2), (4, 4, 0.15);

-- +goose Down
DROP TABLE IF EXISTS payments;
DROP TABLE IF EXISTS order_items;
DROP TABLE IF EXISTS orders;
DROP TABLE IF EXISTS menu_item_ingredients;
DROP TABLE IF EXISTS ingredients;
DROP TABLE IF EXISTS menu_items;
DROP TABLE IF EXISTS menu_categories;
DROP TABLE IF EXISTS restaurant_tables;
DROP TABLE IF EXISTS users;
