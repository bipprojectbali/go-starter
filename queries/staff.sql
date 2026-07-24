-- name: IsPlatformStaff :one
-- Cek apakah email = operator platform (staff). Dipakai RefreshIdentity untuk
-- menentukan bypass RLS (is_super) + role platform. super_admin TIDAK di sini
-- (env-only via SUPER_ADMIN_EMAILS).
SELECT EXISTS (SELECT 1 FROM platform_staff WHERE email = $1)::boolean;

-- name: ListPlatformStaff :many
SELECT * FROM platform_staff ORDER BY created_at DESC;

-- name: AddPlatformStaff :one
INSERT INTO platform_staff (email) VALUES ($1)
ON CONFLICT (email) DO NOTHING
RETURNING *;

-- name: RemovePlatformStaff :exec
DELETE FROM platform_staff WHERE email = $1;
