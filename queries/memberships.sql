-- name: CreateMembership :one
-- Jadikan user anggota workspace dgn role tertentu. Dipakai: register/OAuth (owner
-- workspace pertama), buat workspace baru (owner), terima invite (admin/member).
INSERT INTO memberships (user_id, tenant_id, role)
VALUES ($1, $2, $3)
ON CONFLICT (user_id, tenant_id) DO NOTHING
RETURNING *;

-- name: GetMembership :one
-- Validasi keanggotaan — dipakai middleware Scope SEBELUM membuka tx ber-tenant
-- (memastikan tenant aktif di session memang milik user; anti tenant-forcing).
SELECT * FROM memberships WHERE user_id = $1 AND tenant_id = $2;

-- name: ListMembershipsByUser :many
-- Daftar workspace milik user (untuk switcher sidebar). Urut terlama dulu agar
-- workspace pertama (dari register) jadi default stabil.
SELECT m.tenant_id, m.role, t.name, t.slug
FROM memberships m
JOIN tenants t ON t.id = m.tenant_id
WHERE m.user_id = $1
ORDER BY m.created_at, m.id;

-- name: ListMembersByTenant :many
-- Daftar anggota SATU workspace (panel /admin/members). JOIN users untuk email
-- tampilan — users kini tabel global (tanpa RLS), jadi filter tenant di sini.
SELECT m.id, m.user_id, m.role, m.created_at, u.email, u.avatar_url, u.status
FROM memberships m
JOIN users u ON u.id = m.user_id
WHERE m.tenant_id = $1 AND u.deleted_at IS NULL
ORDER BY m.created_at, m.id;

-- name: UpdateMemberRole :exec
UPDATE memberships SET role = $3 WHERE user_id = $1 AND tenant_id = $2;

-- name: DeleteMembership :exec
-- Keluarkan anggota dari workspace (atau user keluar sendiri).
DELETE FROM memberships WHERE user_id = $1 AND tenant_id = $2;

-- name: ListMembershipsForUsers :many
-- Membership BANYAK user sekaligus (panel /dev: tampilkan workspace+role tiap
-- user). Satu query untuk semua id → hindari N+1 (Rule 13).
SELECT m.user_id, m.tenant_id, m.role, t.name, t.slug
FROM memberships m
JOIN tenants t ON t.id = m.tenant_id
WHERE m.user_id = ANY(@user_ids::bigint[])
ORDER BY m.user_id, m.created_at, m.id;

-- name: CountOwnedWorkspaces :one
-- Berapa workspace yang DIMILIKI user (role owner) — untuk cek kuota sebelum
-- membuat workspace baru. Diundang jadi member/admin TIDAK memakan kuota.
SELECT count(*)::bigint FROM memberships WHERE user_id = $1 AND role = 'owner';

-- name: CountTenantOwners :one
-- Jumlah owner di satu workspace — cegah menghapus/menurunkan owner terakhir
-- (workspace tanpa owner = yatim).
SELECT count(*)::bigint FROM memberships WHERE tenant_id = $1 AND role = 'owner';
