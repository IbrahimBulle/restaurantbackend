-- name: ListTables :many
SELECT id, number, seats, status, qr_token, created_at FROM restaurant_tables ORDER BY CAST(number AS INTEGER), number;

-- name: GetTableByToken :one
SELECT id, number, seats, status, qr_token, created_at FROM restaurant_tables WHERE qr_token = ? LIMIT 1;

-- name: CreateTable :one
INSERT INTO restaurant_tables (number, seats, qr_token) VALUES (?, ?, ?)
RETURNING id, number, seats, status, qr_token, created_at;

-- name: UpdateTableStatus :one
UPDATE restaurant_tables SET status = ? WHERE id = ?
RETURNING id, number, seats, status, qr_token, created_at;
