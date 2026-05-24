-- name: CreateOrder :one
INSERT INTO orders (table_id, session_id, customer_name, status, payment_status, source, subtotal_cents, vat_cents, total_cents)
VALUES (?, ?, ?, 'new', 'unpaid', ?, ?, ?, ?)
RETURNING
  id,
  table_id,
  '' AS table_number,
  session_id,
  customer_name,
  status,
  payment_status,
  subtotal_cents,
  vat_cents,
  total_cents,
  source,
  created_at,
  updated_at;

-- name: CreateOrderItem :one
INSERT INTO order_items (order_id, menu_item_id, quantity, unit_price_cents, notes)
VALUES (?, ?, ?, ?, ?)
RETURNING id, order_id, menu_item_id, quantity, unit_price_cents, notes, status;

-- name: ListOrders :many
SELECT
  o.id,
  o.table_id,
  COALESCE(t.number, '') AS table_number,
  o.session_id,
  o.customer_name,
  o.status,
  COALESCE(o.payment_status, 'unpaid') AS payment_status,
  o.subtotal_cents,
  o.vat_cents,
  o.total_cents,
  o.source,
  o.created_at,
  o.updated_at
FROM orders o
LEFT JOIN restaurant_tables t ON t.id = o.table_id
ORDER BY created_at DESC
LIMIT ?;

-- name: ListOrderItems :many
SELECT
  oi.id,
  oi.order_id,
  oi.menu_item_id,
  COALESCE(mi.name, '') AS menu_item_name,
  oi.quantity,
  oi.unit_price_cents,
  oi.notes,
  oi.status
FROM order_items oi
JOIN menu_items mi ON mi.id = oi.menu_item_id
WHERE oi.order_id = ?;

-- name: UpdateOrderStatus :one
UPDATE orders
SET status = ?, updated_at = CURRENT_TIMESTAMP
WHERE id = ?
RETURNING
  id,
  table_id,
  '' AS table_number,
  session_id,
  customer_name,
  status,
  payment_status,
  subtotal_cents,
  vat_cents,
  total_cents,
  source,
  created_at,
  updated_at;

-- name: GetOrder :one
SELECT
  o.id,
  o.table_id,
  COALESCE(t.number, '') AS table_number,
  o.session_id,
  o.customer_name,
  o.status,
  COALESCE(o.payment_status, 'unpaid') AS payment_status,
  o.subtotal_cents,
  o.vat_cents,
  o.total_cents,
  o.source,
  o.created_at,
  o.updated_at
FROM orders o
LEFT JOIN restaurant_tables t ON t.id = o.table_id
WHERE o.id = ?
LIMIT 1;

-- name: GetLatestOrderForSession :one
SELECT
  o.id,
  o.table_id,
  COALESCE(t.number, '') AS table_number,
  o.session_id,
  o.customer_name,
  o.status,
  COALESCE(o.payment_status, 'unpaid') AS payment_status,
  o.subtotal_cents,
  o.vat_cents,
  o.total_cents,
  o.source,
  o.created_at,
  o.updated_at
FROM orders o
LEFT JOIN restaurant_tables t ON t.id = o.table_id
WHERE o.session_id = ?
ORDER BY o.created_at DESC
LIMIT 1;

-- name: UpdateOrderPaymentStatus :one
UPDATE orders
SET payment_status = ?, status = CASE WHEN ? = 'paid' THEN 'paid' ELSE status END, updated_at = CURRENT_TIMESTAMP
WHERE id = ?
RETURNING
  id,
  table_id,
  '' AS table_number,
  session_id,
  customer_name,
  status,
  payment_status,
  subtotal_cents,
  vat_cents,
  total_cents,
  source,
  created_at,
  updated_at;

-- name: SessionHasOpenOrders :one
SELECT COUNT(*)
FROM orders
WHERE session_id = ?
  AND status NOT IN ('paid', 'cancelled');
