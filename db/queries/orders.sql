-- name: CreateOrder :one
INSERT INTO orders (table_id, customer_name, source, subtotal_cents, vat_cents, total_cents)
VALUES (?, ?, ?, ?, ?, ?)
RETURNING id, table_id, customer_name, status, subtotal_cents, vat_cents, total_cents, source, created_at, updated_at;

-- name: CreateOrderItem :one
INSERT INTO order_items (order_id, menu_item_id, quantity, unit_price_cents, notes)
VALUES (?, ?, ?, ?, ?)
RETURNING id, order_id, menu_item_id, quantity, unit_price_cents, notes, status;

-- name: ListOrders :many
SELECT id, table_id, customer_name, status, subtotal_cents, vat_cents, total_cents, source, created_at, updated_at
FROM orders
ORDER BY created_at DESC
LIMIT ?;

-- name: ListOrderItems :many
SELECT id, order_id, menu_item_id, quantity, unit_price_cents, notes, status FROM order_items WHERE order_id = ?;

-- name: UpdateOrderStatus :one
UPDATE orders SET status = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?
RETURNING id, table_id, customer_name, status, subtotal_cents, vat_cents, total_cents, source, created_at, updated_at;

-- name: GetMenuItem :one
SELECT id, category_id, name, description, price_cents, image_url, active, created_at FROM menu_items WHERE id = ?;

-- name: DeductIngredientStock :exec
UPDATE ingredients
SET stock_qty = stock_qty - (
  SELECT mii.qty * ? FROM menu_item_ingredients AS mii WHERE mii.menu_item_id = ? AND mii.ingredient_id = ingredients.id
)
WHERE ingredients.id IN (SELECT mii.ingredient_id FROM menu_item_ingredients AS mii WHERE mii.menu_item_id = ?);
