-- name: CreatePayment :one
INSERT INTO payments (order_id, method, amount_cents, status, reference, phone_number, provider, metadata_json, confirmed_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, CASE WHEN ? = 'paid' THEN CURRENT_TIMESTAMP ELSE NULL END)
RETURNING id, order_id, method, amount_cents, status, reference, phone_number, provider, metadata_json, confirmed_at, created_at;

-- name: ListPayments :many
SELECT
  id,
  order_id,
  method,
  amount_cents,
  status,
  reference,
  phone_number,
  provider,
  metadata_json,
  confirmed_at,
  created_at
FROM payments
ORDER BY created_at DESC
LIMIT ?;

-- name: ConfirmPayment :one
UPDATE payments
SET
  status = 'paid',
  reference = CASE WHEN ? = '' THEN reference ELSE ? END,
  confirmed_at = CURRENT_TIMESTAMP
WHERE id = ?
RETURNING id, order_id, method, amount_cents, status, reference, phone_number, provider, metadata_json, confirmed_at, created_at;

-- name: GetPaymentsForOrder :many
SELECT
  id,
  order_id,
  method,
  amount_cents,
  status,
  reference,
  phone_number,
  provider,
  metadata_json,
  confirmed_at,
  created_at
FROM payments
WHERE order_id = ?
ORDER BY created_at DESC;
