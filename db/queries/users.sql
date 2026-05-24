-- name: GetUserByEmail :one
SELECT id, name, email, password_hash, role, active, created_at FROM users WHERE email = ? LIMIT 1;

-- name: ListUsers :many
SELECT id, name, email, role, active, created_at FROM users ORDER BY created_at DESC;

-- name: CreateUser :one
INSERT INTO users (name, email, password_hash, role) VALUES (?, ?, ?, ?)
RETURNING id, name, email, role, active, created_at;
