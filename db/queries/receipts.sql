-- name: CreateReceipt :one
INSERT INTO receipts (order_id, payment_id, receipt_number, payload_json)
VALUES (?, ?, ?, ?)
RETURNING id, order_id, payment_id, receipt_number, payload_json, created_at;

-- name: GetReceiptByOrderID :one
SELECT
  id,
  order_id,
  payment_id,
  receipt_number,
  payload_json,
  created_at
FROM receipts
WHERE order_id = ?
LIMIT 1;
