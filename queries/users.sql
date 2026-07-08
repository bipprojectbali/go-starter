-- name: CreateUser :one
INSERT INTO users (email, pass_hash)
VALUES ($1, $2)
RETURNING *;

-- name: CreateOAuthUser :one
-- User baru dari OAuth: tanpa password, email sudah diverifikasi provider.
INSERT INTO users (email, email_verified)
VALUES ($1, true)
RETURNING *;

-- name: GetUserByEmail :one
SELECT * FROM users WHERE email = $1;

-- name: GetUser :one
SELECT * FROM users WHERE id = $1;
