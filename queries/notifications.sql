-- name: CreateNotification :one
-- Dipanggil saat peristiwa keanggotaan terjadi (role diubah, dikeluarkan).
-- payload = snapshot detail siap-render; lihat migrasi 00009.
INSERT INTO notifications (user_id, tenant_id, kind, payload)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: ListNotifications :many
-- Keyset pagination (pola ListUsers) — cast ::timestamptz/::bigint WAJIB, tanpa
-- itu sqlc salah infer tipe cursor_id pada row-value comparison.
--
-- `user_id = $1` BUKAN sekadar filter tampilan: notifications sengaja TANPA RLS
-- (lihat 00009), jadi klausa ini adalah SATU-SATUNYA penjaga isolasi antar-user.
-- Jangan pernah menghapusnya "karena sudah difilter di Go".
SELECT n.*, t.name AS tenant_name
FROM notifications n
LEFT JOIN tenants t ON t.id = n.tenant_id
WHERE n.user_id = $1
  AND (n.created_at, n.id) < (sqlc.arg(cursor_created_at)::timestamptz, sqlc.arg(cursor_id)::bigint)
ORDER BY n.created_at DESC, n.id DESC
LIMIT sqlc.arg(page_size);

-- name: CountUnreadNotifications :one
-- Badge sidebar — dirender di SETIAP halaman, ditopang index partial
-- idx_notif_unread agar tak menyentuh baris yang sudah terbaca.
SELECT count(*)::bigint FROM notifications
WHERE user_id = $1 AND read_at IS NULL;

-- name: MarkNotificationsRead :exec
-- Auto-read saat halaman dibuka. Undangan TIDAK tersentuh di sini — memang tak
-- disimpan di tabel ini (sumber kebenarannya tetap `invites`), sehingga undangan
-- pending tetap terhitung di badge sampai benar-benar ditindak.
UPDATE notifications SET read_at = now()
WHERE user_id = $1 AND read_at IS NULL;
