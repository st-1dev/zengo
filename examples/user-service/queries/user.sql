-- name: GetUser :one
SELECT id, email, name FROM users WHERE id = $1;

-- name: ListUsers :many
SELECT id, email, name FROM users ORDER BY created_at DESC;

-- name: CreateUser :one
INSERT INTO users (id, email, name)
VALUES ($1, $2, $3)
RETURNING id, email, name;
