-- name: CreateSession :one
INSERT INTO sessions (table_id, token, customer_name)
VALUES (?, ?, ?)
RETURNING id, table_id, '' AS table_number, token, customer_name, status, started_at, last_seen_at, ended_at;

-- name: TouchSession :one
UPDATE sessions
SET
  customer_name = CASE WHEN ? = '' THEN customer_name ELSE ? END,
  last_seen_at = CURRENT_TIMESTAMP
WHERE token = ?
RETURNING id, table_id, '' AS table_number, token, customer_name, status, started_at, last_seen_at, ended_at;

-- name: GetSessionByToken :one
SELECT
  s.id,
  s.table_id,
  COALESCE(t.number, '') AS table_number,
  s.token,
  s.customer_name,
  s.status,
  s.started_at,
  s.last_seen_at,
  s.ended_at
FROM sessions s
JOIN restaurant_tables t ON t.id = s.table_id
WHERE s.token = ?
LIMIT 1;

-- name: CloseSession :exec
UPDATE sessions
SET
  status = 'closed',
  ended_at = CURRENT_TIMESTAMP,
  last_seen_at = CURRENT_TIMESTAMP
WHERE id = ?;
