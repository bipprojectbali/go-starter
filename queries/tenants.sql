-- name: CreateTenant :one
-- Buat tenant baru (dipanggil saat register/oauth user baru — 1 user = 1 tenant).
INSERT INTO tenants (name, slug)
VALUES ($1, $2)
RETURNING *;

-- name: GetTenant :one
SELECT * FROM tenants WHERE id = $1;

-- name: GetTenantBySlug :one
SELECT * FROM tenants WHERE slug = $1;

-- name: TenantSlugExists :one
-- Cek ketersediaan slug (untuk auto-suffix unik: acme -> acme-2 -> ...).
SELECT EXISTS (SELECT 1 FROM tenants WHERE slug = $1)::boolean;

-- name: UpdateTenant :exec
-- Ganti NAMA tampilan workspace (owner-only, di-guard di handler). Slug SENGAJA
-- tak diubah — immutable setelah dibuat (stabilitas URL; ganti display != ganti URL).
UPDATE tenants SET name = $2 WHERE id = $1;
