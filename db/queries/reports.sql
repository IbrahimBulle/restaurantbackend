-- name: DailySales :many
SELECT date(created_at) AS day, COUNT(*) AS order_count, SUM(total_cents) AS revenue_cents
FROM orders
WHERE status IN ('paid','served','ready') OR payment_status = 'paid'
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

-- name: PaymentMethodStats :many
SELECT method, COUNT(*) AS count, COALESCE(SUM(amount_cents), 0) AS revenue_cents
FROM payments
WHERE status = 'paid'
GROUP BY method
ORDER BY count DESC, method ASC;

-- name: RevenueSnapshot :one
SELECT
  COALESCE(SUM(CASE WHEN date(created_at) = date('now', 'localtime') AND status = 'paid' THEN amount_cents END), 0) AS daily_total_cents,
  COALESCE(SUM(CASE WHEN date(created_at) >= date('now', '-6 day', 'localtime') AND status = 'paid' THEN amount_cents END), 0) AS weekly_total_cents,
  COALESCE(SUM(CASE WHEN strftime('%Y-%m', created_at) = strftime('%Y-%m', 'now', 'localtime') AND status = 'paid' THEN amount_cents END), 0) AS monthly_total_cents
FROM payments;

-- name: OpenOrderCount :one
SELECT COUNT(*) FROM orders WHERE status NOT IN ('paid', 'cancelled');

-- name: ActiveSessionCount :one
SELECT COUNT(*) FROM sessions WHERE status = 'active';
