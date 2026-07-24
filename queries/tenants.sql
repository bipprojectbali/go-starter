-- name: CreateTenant :one
-- Buat tenant baru (dipanggil saat register/oauth user baru — 1 user = 1 tenant).
INSERT INTO tenants (name, slug)
VALUES ($1, $2)
RETURNING *;

-- name: GetTenant :one
SELECT * FROM tenants WHERE id = $1;

-- name: GetTenantBySlug :one
SELECT * FROM tenants WHERE slug = $1;
