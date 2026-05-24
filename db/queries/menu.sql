-- name: ListCategories :many
SELECT id, name, sort_order, active FROM menu_categories WHERE active = 1 ORDER BY sort_order, name;

-- name: ListMenuItems :many
SELECT id, category_id, name, description, price_cents, image_url, active, created_at
FROM menu_items
WHERE active = 1
ORDER BY category_id, name;

-- name: UpsertMenuItem :one
INSERT INTO menu_items (category_id, name, description, price_cents, image_url, active)
VALUES (?, ?, ?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET
  category_id = excluded.category_id,
  name = excluded.name,
  description = excluded.description,
  price_cents = excluded.price_cents,
  image_url = excluded.image_url,
  active = excluded.active
RETURNING id, category_id, name, description, price_cents, image_url, active, created_at;
