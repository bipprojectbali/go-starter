-- name: CreateUser :one
INSERT INTO users (email, pass_hash)
VALUES ($1, $2)
RETURNING *;

-- name: CreateOAuthUser :one
-- User baru dari OAuth: tanpa password, email terverifikasi provider, + avatar.
INSERT INTO users (email, email_verified, avatar_url)
VALUES ($1, true, $2)
RETURNING *;

-- name: GetUserByEmail :one
-- Soft-delete gotcha: user terhapus tak boleh login.
SELECT * FROM users WHERE email = $1 AND deleted_at IS NULL;

-- name: GetUser :one
SELECT * FROM users WHERE id = $1 AND deleted_at IS NULL;

-- name: UpdateUserAvatar :exec
-- URL avatar Google berubah saat user ganti foto → update tiap login.
UPDATE users SET avatar_url = $2 WHERE id = $1;

-- name: UpdateUserRole :exec
UPDATE users SET role = $2 WHERE id = $1 AND deleted_at IS NULL;

-- name: PromoteSuperAdmins :exec
-- Reconcile boot: naikkan email root (env) ke super_admin bila belum. Promote-
-- ONLY — tak pernah menurunkan siapa pun, jadi super-admin DB tier-2 (diangkat
-- lewat panel) tetap aman, dan email yang dicabut dari env tak otomatis turun.
UPDATE users SET role = 'super_admin'
WHERE email = ANY(@emails::text[]) AND role <> 'super_admin' AND deleted_at IS NULL;

-- name: UpdateUserStatus :exec
UPDATE users SET status = $2 WHERE id = $1 AND deleted_at IS NULL;

-- name: SoftDeleteUser :exec
UPDATE users SET deleted_at = now() WHERE id = $1 AND deleted_at IS NULL;

-- name: ListUsers :many
-- Panel /dev: keyset pagination, hanya user aktif (belum soft-delete).
SELECT * FROM users
WHERE deleted_at IS NULL
  AND (created_at, id) < (sqlc.arg(cursor_created_at)::timestamptz, sqlc.arg(cursor_id)::bigint)
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg(page_size);
