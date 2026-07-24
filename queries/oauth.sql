-- name: GetOAuthAccount :one
SELECT * FROM oauth_accounts WHERE provider = $1 AND provider_uid = $2;

-- name: CreateOAuthAccount :one
INSERT INTO oauth_accounts (user_id, provider, provider_uid, tenant_id)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: ListOAuthAccountsByUser :many
SELECT * FROM oauth_accounts WHERE user_id = $1 ORDER BY created_at;
