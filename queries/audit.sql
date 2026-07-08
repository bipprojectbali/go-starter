-- name: CreateAuditLog :one
-- Jejak aksi admin. metadata TANPA PII (id saja, bukan email/nama).
INSERT INTO audit_logs (actor_user_id, action, target_type, target_id, metadata)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: ListAuditLogs :many
SELECT * FROM audit_logs
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg(page_size);
