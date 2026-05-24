-- name: DailySales :many
SELECT date(created_at) AS day, COUNT(*) AS order_count, SUM(total_cents) AS revenue_cents
FROM orders
WHERE status IN ('paid','served','ready')
GROUP BY date(created_at)
ORDER BY day DESC
LIMIT ?;

-- name: BestSellingItems :many
SELECT mi.name, SUM(oi.quantity) AS quantity, SUM(oi.quantity * oi.unit_price_cents) AS revenue_cents
FROM order_items oi
JOIN menu_items mi ON mi.id = oi.menu_item_id
GROUP BY mi.id, mi.name
ORDER BY quantity DESC
LIMIT ?;

-- name: LowStockIngredients :many
SELECT id, name, unit, stock_qty, low_stock_qty FROM ingredients WHERE stock_qty <= low_stock_qty ORDER BY stock_qty ASC;
