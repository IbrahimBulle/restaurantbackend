-- name: CreatePayment :one
INSERT INTO payments (order_id, method, amount_cents, status, reference)
VALUES (?, ?, ?, ?, ?)
RETURNING id, order_id, method, amount_cents, status, reference, created_at;

-- name: ListPayments :many
SELECT id, order_id, method, amount_cents, status, reference, created_at FROM payments ORDER BY created_at DESC LIMIT ?;
