-- name: ListCategories :many
SELECT id, name, sort_order, active
FROM menu_categories
WHERE active = 1
ORDER BY sort_order, name;

-- name: ListAdminCategories :many
SELECT id, name, sort_order, active
FROM menu_categories
ORDER BY active DESC, sort_order, name;

-- name: CreateCategory :one
INSERT INTO menu_categories (name, sort_order, active)
VALUES (?, ?, 1)
RETURNING id, name, sort_order, active;

-- name: ListMenuItems :many
SELECT
  id,
  category_id,
  name,
  description,
  price_cents,
  image_url,
  sku,
  item_type,
  cost_cents,
  sort_order,
  active,
  created_at
FROM menu_items
WHERE active = 1
ORDER BY category_id, sort_order, name;

-- name: ListAdminMenuItems :many
SELECT
  id,
  category_id,
  name,
  description,
  price_cents,
  image_url,
  sku,
  item_type,
  cost_cents,
  sort_order,
  active,
  created_at
FROM menu_items
ORDER BY active DESC, category_id, sort_order, name;

-- name: GetMenuItem :one
SELECT
  id,
  category_id,
  name,
  description,
  price_cents,
  image_url,
  sku,
  item_type,
  cost_cents,
  sort_order,
  active,
  created_at
FROM menu_items
WHERE id = ?
LIMIT 1;

-- name: CreateMenuItem :one
INSERT INTO menu_items (
  category_id,
  name,
  description,
  price_cents,
  image_url,
  sku,
  item_type,
  cost_cents,
  sort_order,
  active
)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
RETURNING
  id,
  category_id,
  name,
  description,
  price_cents,
  image_url,
  sku,
  item_type,
  cost_cents,
  sort_order,
  active,
  created_at;

-- name: UpdateMenuItem :one
UPDATE menu_items
SET
  category_id = ?,
  name = ?,
  description = ?,
  price_cents = ?,
  image_url = ?,
  sku = ?,
  item_type = ?,
  cost_cents = ?,
  sort_order = ?,
  active = ?
WHERE id = ?
RETURNING
  id,
  category_id,
  name,
  description,
  price_cents,
  image_url,
  sku,
  item_type,
  cost_cents,
  sort_order,
  active,
  created_at;

-- name: ArchiveMenuItem :exec
UPDATE menu_items
SET active = 0
WHERE id = ?;
