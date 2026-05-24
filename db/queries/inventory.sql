-- name: ListProducts :many
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
LEFT JOIN menu_categories mc ON mc.id = mi.category_id
LEFT JOIN inventory inv ON inv.product_id = mi.id
ORDER BY mi.active DESC, mi.sort_order, mi.name;

-- name: GetProduct :one
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
LEFT JOIN menu_categories mc ON mc.id = mi.category_id
LEFT JOIN inventory inv ON inv.product_id = mi.id
WHERE mi.id = ?
LIMIT 1;

-- name: UpsertInventory :one
INSERT INTO inventory (product_id, stock_qty, reorder_level, unit, track_stock, updated_at)
VALUES (?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
ON CONFLICT(product_id) DO UPDATE SET
  stock_qty = excluded.stock_qty,
  reorder_level = excluded.reorder_level,
  unit = excluded.unit,
  track_stock = excluded.track_stock,
  updated_at = CURRENT_TIMESTAMP
RETURNING product_id, stock_qty, reorder_level, unit, track_stock, updated_at;

-- name: AdjustInventoryStock :one
UPDATE inventory
SET
  stock_qty = stock_qty + ?,
  updated_at = CURRENT_TIMESTAMP
WHERE product_id = ?
RETURNING product_id, stock_qty, reorder_level, unit, track_stock, updated_at;

-- name: CreateInventoryMovement :one
INSERT INTO inventory_movements (product_id, change_qty, reason, reference)
VALUES (?, ?, ?, ?)
RETURNING id, product_id, change_qty, reason, reference, created_at;

-- name: ListInventoryMovements :many
SELECT
  im.id,
  im.product_id,
  COALESCE(mi.name, '') AS product_name,
  im.change_qty,
  im.reason,
  im.reference,
  im.created_at
FROM inventory_movements im
JOIN menu_items mi ON mi.id = im.product_id
ORDER BY im.created_at DESC
LIMIT ?;
