-- name: GetSettings :one
SELECT
  id,
  business_name,
  business_type,
  phone,
  currency_code,
  mpesa_till,
  receipt_footer,
  created_at,
  updated_at
FROM settings
WHERE id = 1
LIMIT 1;

-- name: SaveSettings :one
INSERT INTO settings (
  id,
  business_name,
  business_type,
  phone,
  currency_code,
  mpesa_till,
  receipt_footer,
  created_at,
  updated_at
)
VALUES (1, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
ON CONFLICT(id) DO UPDATE SET
  business_name = excluded.business_name,
  business_type = excluded.business_type,
  phone = excluded.phone,
  currency_code = excluded.currency_code,
  mpesa_till = excluded.mpesa_till,
  receipt_footer = excluded.receipt_footer,
  updated_at = CURRENT_TIMESTAMP
RETURNING
  id,
  business_name,
  business_type,
  phone,
  currency_code,
  mpesa_till,
  receipt_footer,
  created_at,
  updated_at;
