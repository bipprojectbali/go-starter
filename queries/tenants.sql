-- name: CreateTenant :one
-- Buat tenant baru (dipanggil saat register/oauth user baru — 1 user = 1 tenant).
INSERT INTO tenants (name, slug)
VALUES ($1, $2)
RETURNING *;

-- name: GetTenant :one
SELECT * FROM tenants WHERE id = $1;

-- name: GetTenantBySlug :one
-- SENGAJA tanpa filter deleted_at: middleware Scope perlu MEMBEDAKAN "workspace
-- tak pernah ada" dari "workspace terhapus" (0005) — keduanya berujung 404 bagi
-- user, tapi jalur restore/purge platform butuh barisnya tetap terbaca.
SELECT * FROM tenants WHERE slug = $1;

-- name: TenantSlugExists :one
-- Cek ketersediaan slug (untuk auto-suffix unik: acme -> acme-2 -> ...). Ikut
-- menghitung workspace TERHAPUS: slug tak dilepas selama masa tenggang, kalau
-- dilepas orang lain bisa mengambilnya & restore jadi mustahil (0005 §5).
SELECT EXISTS (SELECT 1 FROM tenants WHERE slug = $1)::boolean;

-- name: UpdateTenant :exec
-- Ganti NAMA tampilan workspace (owner-only, di-guard di handler). Slug SENGAJA
-- tak diubah — immutable setelah dibuat (stabilitas URL; ganti display != ganti URL).
UPDATE tenants SET name = $2 WHERE id = $1;

-- name: SuspendTenant :exec
-- PLATFORM-ONLY (super_admin/staff). Owner tak bisa membatalkannya sendiri —
-- kalau bisa, gunanya hilang (0005 §1). Alasan disimpan agar pertanyaan support
-- pertama ("kenapa workspace saya mati") terjawab tanpa menggali audit log.
UPDATE tenants
SET status = 'suspended', suspended_at = now(), suspended_by = $2, suspend_reason = $3
WHERE id = $1 AND deleted_at IS NULL;

-- name: UnsuspendTenant :exec
-- PLATFORM-ONLY. Membersihkan jejak suspensi agar kolomnya tak jadi sisa yang
-- menyesatkan saat suspensi berikutnya.
UPDATE tenants
SET status = 'active', suspended_at = NULL, suspended_by = NULL, suspend_reason = NULL
WHERE id = $1 AND status = 'suspended' AND deleted_at IS NULL;

-- name: ArchiveTenant :exec
-- OWNER. Workspace jadi READ-ONLY tapi datanya utuh. Guard `status = 'active'`
-- mencegah archive menimpa SUSPENSI platform — kalau tidak, owner bisa keluar
-- dari suspensi lewat pintu samping (archive lalu unarchive).
UPDATE tenants
SET status = 'archived', archived_at = now()
WHERE id = $1 AND status = 'active' AND deleted_at IS NULL;

-- name: UnarchiveTenant :exec
-- OWNER. Hanya dari 'archived' — tak bisa dipakai membatalkan suspensi platform.
UPDATE tenants
SET status = 'active', archived_at = NULL
WHERE id = $1 AND status = 'archived' AND deleted_at IS NULL;

-- name: SoftDeleteTenant :exec
-- Owner ATAU platform. Masa tenggang: baris tetap ada, slug TIDAK dilepas.
UPDATE tenants SET deleted_at = now() WHERE id = $1 AND deleted_at IS NULL;

-- name: RestoreTenant :exec
-- Batalkan penghapusan dalam masa tenggang. Status dikembalikan ke 'active':
-- workspace yang dihapus saat ter-arsip pun kembali sebagai aktif — pemulihan
-- harus meninggalkan keadaan yang bisa langsung dipakai, bukan setengah jalan.
UPDATE tenants
SET deleted_at = NULL, status = 'active', archived_at = NULL
WHERE id = $1 AND deleted_at IS NOT NULL;

-- name: ListExpiredTenants :many
-- Kandidat purge permanen: terhapus melewati masa tenggang. Dipanggil perintah
-- terjadwal, TAK PERNAH di jalur request (purge = kerja berat & tak reversibel).
SELECT * FROM tenants
WHERE deleted_at IS NOT NULL AND deleted_at < $1
ORDER BY deleted_at;

-- name: PurgeTenant :exec
-- Hapus PERMANEN. memberships/invites/notifications ikut CASCADE (data
-- operasional); audit_logs TIDAK — FK-nya ON DELETE SET NULL sejak migrasi 00010,
-- sebab bukti tak boleh lenyap bersama yang dibuktikan (0005 §6).
DELETE FROM tenants WHERE id = $1 AND deleted_at IS NOT NULL;

-- name: ListTenantsForPlatform :many
-- Daftar workspace untuk panel /dev (lintas-workspace — route platform). Termasuk
-- yang suspended/archived: justru itu yang perlu dilihat operator. Yang TERHAPUS
-- ikut tampil agar restore masih mungkin selama masa tenggang.
SELECT t.*, count(m.user_id)::bigint AS member_count
FROM tenants t
LEFT JOIN memberships m ON m.tenant_id = t.id
GROUP BY t.id
ORDER BY t.created_at DESC, t.id DESC
LIMIT $1 OFFSET $2;

-- name: CountTenantsForPlatform :one
SELECT count(*)::bigint FROM tenants;

-- name: CountTenants :one
-- Jumlah workspace yang masih hidup. Dipakai bootstrap mode single (0006):
-- 0 → buat tenant tunggal, 1 → lanjut, >1 → app menolak start.
SELECT count(*)::bigint FROM tenants WHERE deleted_at IS NULL;

-- name: SetTenantSlug :exec
-- Ubah slug + nama sekaligus. HANYA dipakai bootstrap mode single untuk
-- mengadopsi workspace KOSONG bawaan migrasi (slug "default") jadi aplikasi
-- tunggal. Bukan operasi umum: slug immutable sejak 0004 karena mengubahnya
-- mematikan setiap tautan tersimpan — pemanggil wajib sudah memastikan
-- workspace itu belum berisi anggota.
UPDATE tenants SET slug = $2, name = $3 WHERE id = $1;
