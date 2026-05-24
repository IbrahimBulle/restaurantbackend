-- name: DailySales :many
SELECT date(created_at) AS day, COUNT(*) AS order_count, SUM(total_cents) AS revenue_cents
FROM orders
WHERE status IN ('paid', 'served', 'ready') OR payment_status = 'paid'
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

-- name: LowStockProducts :many
SELECT
  mi.id,
  mi.category_id,
  COALESCE(mc.name, '') AS category_name,
  mi.name,
  mi.description,
  mi.price_cents,
  mi.cost_cents,
  mi.image_url,
  mi.sku,
  mi.item_type,
  mi.sort_order,
  mi.active,
  COALESCE(inv.stock_qty, 0) AS stock_qty,
  COALESCE(inv.reorder_level, 0) AS reorder_level,
  COALESCE(inv.unit, 'pcs') AS unit,
  COALESCE(inv.track_stock, 1) AS track_stock,
  CASE
    WHEN COALESCE(inv.track_stock, 1) = 1 AND COALESCE(inv.stock_qty, 0) <= 0 THEN 1
    ELSE 0
  END AS out_of_stock,
  mi.created_at
FROM menu_items mi
JOIN inventory inv ON inv.product_id = mi.id
LEFT JOIN menu_categories mc ON mc.id = mi.category_id
WHERE COALESCE(inv.track_stock, 1) = 1
  AND COALESCE(inv.stock_qty, 0) <= COALESCE(inv.reorder_level, 0)
ORDER BY inv.stock_qty ASC, mi.name ASC;

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
