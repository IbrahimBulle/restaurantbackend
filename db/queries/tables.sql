-- name: ListTables :many
SELECT
  t.id,
  t.number,
  t.seats,
  t.status,
  COALESCE(t.slug, '') AS slug,
  t.qr_token,
  COALESCE(q.public_url, '') AS qr_url,
  COALESCE(q.image_data, '') AS qr_image_data,
  COALESCE(t.active, 1) AS active,
  t.created_at
FROM restaurant_tables t
LEFT JOIN qr_codes q ON q.table_id = t.id
WHERE COALESCE(t.active, 1) = 1
ORDER BY CAST(t.number AS INTEGER), t.number;

-- name: GetTableByID :one
SELECT
  t.id,
  t.number,
  t.seats,
  t.status,
  COALESCE(t.slug, '') AS slug,
  t.qr_token,
  COALESCE(q.public_url, '') AS qr_url,
  COALESCE(q.image_data, '') AS qr_image_data,
  COALESCE(t.active, 1) AS active,
  t.created_at
FROM restaurant_tables t
LEFT JOIN qr_codes q ON q.table_id = t.id
WHERE t.id = ?
LIMIT 1;

-- name: GetTableByIdentifier :one
SELECT
  t.id,
  t.number,
  t.seats,
  t.status,
  COALESCE(t.slug, '') AS slug,
  t.qr_token,
  COALESCE(q.public_url, '') AS qr_url,
  COALESCE(q.image_data, '') AS qr_image_data,
  COALESCE(t.active, 1) AS active,
  t.created_at
FROM restaurant_tables t
LEFT JOIN qr_codes q ON q.table_id = t.id
WHERE t.slug = ? OR t.qr_token = ?
LIMIT 1;

-- name: CreateTable :one
INSERT INTO restaurant_tables (number, seats, qr_token, slug)
VALUES (?, ?, ?, ?)
RETURNING
  id,
  number,
  seats,
  status,
  COALESCE(slug, '') AS slug,
  qr_token,
  '' AS qr_url,
  '' AS qr_image_data,
  COALESCE(active, 1) AS active,
  created_at;

-- name: UpdateTableStatus :one
UPDATE restaurant_tables
SET status = ?
WHERE id = ?
RETURNING
  id,
  number,
  seats,
  status,
  COALESCE(slug, '') AS slug,
  qr_token,
  '' AS qr_url,
  '' AS qr_image_data,
  COALESCE(active, 1) AS active,
  created_at;

-- name: UpsertQRCode :exec
INSERT INTO qr_codes (table_id, public_url, image_data)
VALUES (?, ?, ?)
ON CONFLICT(table_id) DO UPDATE SET
  public_url = excluded.public_url,
  image_data = CASE
    WHEN excluded.image_data = '' THEN qr_codes.image_data
    ELSE excluded.image_data
  END,
  updated_at = CURRENT_TIMESTAMP;
