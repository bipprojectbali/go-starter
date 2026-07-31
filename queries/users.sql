-- name: CreateUser :one
-- users = tabel GLOBAL (identitas murni, TANPA tenant/role — keduanya pindah ke
-- memberships). Keanggotaan dibuat terpisah via CreateMembership dalam tx sama.
INSERT INTO users (email, pass_hash)
VALUES ($1, $2)
RETURNING *;

-- name: CreateOAuthUser :one
-- User baru dari OAuth: tanpa password, email terverifikasi provider, + avatar
-- & nama tampilan. Keduanya nullable — provider boleh tak mengirimkannya.
INSERT INTO users (email, email_verified, avatar_url, name)
VALUES ($1, true, $2, $3)
RETURNING *;

-- name: GetUserByEmail :one
-- Soft-delete gotcha: user terhapus tak boleh login.
SELECT * FROM users WHERE email = $1 AND deleted_at IS NULL;

-- name: GetUser :one
SELECT * FROM users WHERE id = $1 AND deleted_at IS NULL;

-- name: UpdateUserProfile :exec
-- Profil dari provider (avatar + nama tampilan) di-refresh TIAP LOGIN: keduanya
-- berubah di sisi Google tanpa memberi tahu kita, dan login adalah satu-satunya
-- saat kita mendengar kabar terbaru.
--
-- COALESCE, bukan penimpaan lugas: argumen NULL berarti "provider tak
-- mengirimkan apa pun kali ini", BUKAN "hapus yang tersimpan". Tanpa ini, satu
-- respons Google tanpa `picture` (boleh terjadi walau scope diminta) akan
-- menghapus avatar yang sudah benar. Nilai lama lebih baik daripada tak ada.
--
-- Konsekuensi yang disengaja: user yang MENGHAPUS fotonya di Google tetap
-- memakai foto lamanya di sini. Ditukar sadar dengan menghindari penghapusan
-- palsu, yang jauh lebih sering terjadi.
--
-- CATATAN untuk kelak: begitu ada fitur "edit profil" di aplikasi ini, refresh
-- otomatis ini akan MENIMPA nama yang disunting user tiap kali ia login. Saat
-- itu tiba, nama pilihan sendiri harus jadi kolom terpisah yang diutamakan —
-- jangan sekadar mematikan refresh, sebab avatar tetap perlu ikut berubah.
UPDATE users
SET avatar_url = COALESCE(sqlc.narg(avatar_url)::text, avatar_url),
    name       = COALESCE(sqlc.narg(name)::text, name)
WHERE id = sqlc.arg(id);

-- name: UpdateUserQuota :exec
-- Kuota workspace per user. NULL = IKUT DEFAULT GLOBAL (platform_settings), angka
-- = hak khusus milik user itu yang kebal perubahan global. Membedakan keduanya
-- adalah alasan kolom ini nullable — dgn NOT NULL, "kebetulan 3" tak bisa
-- dibedakan dari "sengaja diberi 3".
UPDATE users SET workspace_quota = $2 WHERE id = $1 AND deleted_at IS NULL;

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

-- name: CountQuotaOverrides :one
-- Berapa user memegang HAK KHUSUS (kuota di-override, bukan mengikuti global).
-- Dipakai halaman pengaturan untuk menyatakan dampak sebelum operator menyimpan:
-- mereka SENGAJA tak ikut berubah, dan tanpa angka ini operator mengira
-- pengaturannya tak bekerja.
SELECT count(*)::bigint FROM users
WHERE workspace_quota IS NOT NULL AND deleted_at IS NULL;
