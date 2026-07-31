-- name: CreateAuditLog :one
-- Jejak aksi admin. metadata TANPA PII (id saja, bukan email/nama).
INSERT INTO audit_logs (actor_user_id, action, target_type, target_id, metadata, tenant_id)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: ListAuditLogs :many
SELECT * FROM audit_logs
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg(page_size);

-- name: PurgeAuditLogsBefore :execrows
-- Retensi: buang jejak yang lebih tua dari batas. Dipanggil pemeliharaan
-- terjadwal, TAK PERNAH di jalur request.
--
-- Tabel ini append-only dan dulu tumbuh selamanya. Selama tak pernah dibaca itu
-- cuma soal disk; begitu ia jadi halaman yang aktif dibuka, biayanya ikut ke
-- setiap query. Yang dibayar: peristiwa lebih lama dari batas TAK BISA
-- diselidiki lagi — karena itu batasnya hidup di platform_settings (bisa
-- dinaikkan operator sebelum sesuatu telanjur hilang), bukan dipaku di kode.
--
-- Mengembalikan JUMLAH baris terhapus supaya pemeliharaan bisa mencatat apa yang
-- sebenarnya terjadi: pekerjaan pembersihan yang diam tak bisa dibedakan dari
-- pekerjaan yang tak pernah jalan.
DELETE FROM audit_logs WHERE created_at < sqlc.arg(before)::timestamptz;

-- name: ListActivityTrail :many
-- Jejak aktivitas untuk panel /dev/logs: SEMUA aksi, bukan hanya login/logout.
--
-- Sebelum ini panel hanya menampilkan `action IN ('auth.login','auth.logout')`,
-- sementara 14 jenis aksi lain ikut tercatat dan tak pernah dilihat siapa pun —
-- biayanya sudah dibayar penuh (kolom, index, penulisan di 16 tempat), hanya
-- jalur bacanya yang berhenti di satu filter.
--
-- NAMA di-JOIN saat DIBACA, tak pernah disalin ke metadata: kolom itu sengaja
-- bebas PII, dan nama yang disalin ke sana jadi salinan permanen yang tak ikut
-- terhapus bersama usernya. LEFT JOIN, sebab jejak wajib tetap terbaca setelah
-- pelaku/sasarannya hilang — actor_user_id ON DELETE SET NULL, dan target_id
-- sengaja BUKAN FK.
--
-- Sasaran dicari dari DUA tabel sesuai target_type, dan syarat itu WAJIB ada di
-- klausa JOIN-nya: tanpa `target_type = '...'`, id workspace akan mencocoki
-- baris users yang id-nya kebetulan sama, lalu memampangkan nama orang yang tak
-- terlibat. Salah, dan terlihat meyakinkan.
--
-- Filter opsional lewat sqlc.narg (NULL = tanpa filter) supaya satu query
-- melayani semua kombinasi — dua query terpisah akan berbeda diam-diam begitu
-- salah satunya diubah.
SELECT
    a.id,
    a.action,
    a.target_type,
    a.target_id,
    a.metadata,
    a.created_at,
    a.actor_user_id,
    actor.name       AS actor_name,
    actor.email      AS actor_email,
    tu.name          AS target_user_name,
    tu.email         AS target_user_email,
    tt.name          AS target_workspace_name
FROM audit_logs a
LEFT JOIN users   actor ON actor.id = a.actor_user_id
LEFT JOIN users   tu    ON tu.id    = a.target_id AND a.target_type IN ('user', 'session')
LEFT JOIN tenants tt    ON tt.id    = a.target_id AND a.target_type = 'workspace'
WHERE a.created_at >= sqlc.arg(from_at)::timestamptz
  AND a.created_at <  sqlc.arg(to_at)::timestamptz
  AND (a.created_at, a.id) < (sqlc.arg(cursor_created_at)::timestamptz, sqlc.arg(cursor_id)::bigint)
  -- Prefiks, bukan sama-dengan: satu pilihan "workspace" menyaring seluruh
  -- keluarga workspace.* tanpa menuntut daftar aksi di UI ikut diperbarui tiap
  -- kali ada aksi baru — daftar yang harus dijaga selaras pasti tertinggal.
  AND (sqlc.narg(action_prefix)::text IS NULL OR a.action LIKE sqlc.narg(action_prefix)::text || '%')
  AND (sqlc.narg(actor_id)::bigint IS NULL OR a.actor_user_id = sqlc.narg(actor_id)::bigint)
ORDER BY a.created_at DESC, a.id DESC
LIMIT sqlc.arg(page_size);

-- name: ListActivityActors :many
-- Orang yang punya jejak pada rentang ini — isi dropdown "filter per-orang".
--
-- Diturunkan dari DATA, bukan dari daftar user: memilih orang yang tak punya
-- jejak sama sekali selalu menghasilkan halaman kosong, dan pilihan yang pasti
-- kosong lebih buruk daripada pilihan yang tak ada. Ikut mengecil sendiri saat
-- rentangnya dipersempit.
SELECT
    a.actor_user_id::bigint AS actor_id,
    u.name                  AS actor_name,
    u.email                 AS actor_email,
    count(*)::bigint        AS events
FROM audit_logs a
JOIN users u ON u.id = a.actor_user_id
WHERE a.created_at >= sqlc.arg(from_at)::timestamptz
  AND a.created_at <  sqlc.arg(to_at)::timestamptz
GROUP BY a.actor_user_id, u.name, u.email
ORDER BY events DESC, u.email;

-- name: CountActivityByAction :many
-- Jumlah peristiwa per KELUARGA aksi pada rentang ini (auth, workspace, member,
-- user, invite, settings, platform) — dipakai untuk melabeli opsi filter dengan
-- angka, sehingga operator tahu mana yang berisi sebelum mengkliknya.
--
-- split_part di segmen pertama: penamaan aksi kita selalu `keluarga.sisanya`
-- (auth.login, workspace.create, member.role.update), jadi keluarga bisa
-- diturunkan tanpa tabel pemetaan yang harus dijaga selaras.
SELECT
    split_part(a.action, '.', 1) AS family,
    count(*)::bigint             AS events
FROM audit_logs a
WHERE a.created_at >= sqlc.arg(from_at)::timestamptz
  AND a.created_at <  sqlc.arg(to_at)::timestamptz
GROUP BY family
ORDER BY events DESC, family;
