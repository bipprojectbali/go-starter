-- name: CreateInvite :one
-- Undangan bergabung ke workspace. token = rahasia URL (crypto/rand hex via
-- oauth.NewState). email boleh milik orang yang BELUM punya akun.
INSERT INTO invites (tenant_id, email, role, token, invited_by, expires_at)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: GetInviteByToken :one
-- Jalur PUBLIK (/invite/{token}) — penerima belum tentu login/anggota. Validasi
-- kedaluwarsa & sudah-dipakai dilakukan di handler agar pesannya spesifik.
SELECT i.*, t.name AS tenant_name, t.slug AS tenant_slug
FROM invites i
JOIN tenants t ON t.id = i.tenant_id
WHERE i.token = $1;

-- name: ListInvitesByTenant :many
-- Undangan PENDING satu workspace (panel anggota) — yang sudah diterima disaring.
SELECT * FROM invites
WHERE tenant_id = $1 AND accepted_at IS NULL AND expires_at > now()
ORDER BY created_at DESC;

-- name: AcceptInvite :exec
-- Tandai terpakai (one-time). Guard accepted_at IS NULL → race dua klik tak
-- menghasilkan dua membership (UNIQUE di memberships jadi jaring kedua).
UPDATE invites SET accepted_at = now()
WHERE token = $1 AND accepted_at IS NULL AND expires_at > now();

-- name: DeleteInvite :exec
-- Batalkan undangan yang belum diterima.
DELETE FROM invites WHERE id = $1 AND tenant_id = $2;
