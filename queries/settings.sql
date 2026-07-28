-- name: GetSetting :one
-- Baca satu pengaturan platform. Tak ditemukan = keadaan SAH (pemanggil jatuh ke
-- default bawaan kode), bukan error yang perlu diributkan.
SELECT * FROM platform_settings WHERE key = $1;

-- name: ListSettings :many
-- Semua pengaturan sekaligus — dipakai halaman /dev/settings dan pemuatan cache
-- saat boot. Jumlahnya sedikit, jadi tak dipaginasi (beda dari daftar user).
SELECT * FROM platform_settings ORDER BY key;

-- name: UpsertSetting :exec
-- Simpan pengaturan. UPSERT karena baris mungkin belum ada (deployment baru yang
-- tak menjalankan seed): pemanggil tak perlu tahu bedanya.
INSERT INTO platform_settings (key, value, updated_by, updated_at)
VALUES ($1, $2, $3, now())
ON CONFLICT (key) DO UPDATE
SET value = EXCLUDED.value, updated_by = EXCLUDED.updated_by, updated_at = now();
