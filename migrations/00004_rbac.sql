-- +goose Up
-- +goose StatementBegin
-- RBAC & user management. role = otoritas (Casbin subject); status = state
-- operasional (gate login); deleted_at = soft-delete. Tiga concern ortogonal.
ALTER TABLE users ADD COLUMN IF NOT EXISTS role       TEXT NOT NULL DEFAULT 'user';
ALTER TABLE users ADD COLUMN IF NOT EXISTS status     TEXT NOT NULL DEFAULT 'active';
ALTER TABLE users ADD COLUMN IF NOT EXISTS avatar_url TEXT;             -- foto Google (claim picture)
ALTER TABLE users ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ;      -- soft-delete
-- +goose StatementEnd

-- +goose StatementBegin
-- Guard nilai valid. NOT VALID: hanya cek baris baru, tak gagal di data lama.
ALTER TABLE users ADD CONSTRAINT users_role_chk
    CHECK (role IN ('user','admin','super_admin')) NOT VALID;
ALTER TABLE users ADD CONSTRAINT users_status_chk
    CHECK (status IN ('active','disabled','blocked')) NOT VALID;
-- +goose StatementEnd

-- +goose StatementBegin
-- List user aktif untuk panel (keyset created_at,id). Partial: hanya yang belum
-- di-soft-delete — sekaligus gotcha soft-delete: query wajib filter deleted_at.
CREATE INDEX IF NOT EXISTS idx_users_active
    ON users (created_at DESC, id DESC) WHERE deleted_at IS NULL;
-- +goose StatementEnd

-- +goose StatementBegin
-- Jejak audit aksi admin. Append-only. target_id BUKAN FK (tahan bila target
-- kelak hard-delete). metadata JSONB TANPA PII (rule 12) — simpan id, bukan email.
CREATE TABLE IF NOT EXISTS audit_logs (
    id            BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    actor_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    action        TEXT NOT NULL,                -- "user.role.update", "user.disable", ...
    target_type   TEXT NOT NULL,                -- "user"
    target_id     BIGINT,
    metadata      JSONB NOT NULL DEFAULT '{}',  -- {"from":"user","to":"admin"}
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_audit_actor ON audit_logs (actor_user_id, created_at DESC);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS audit_logs;
-- +goose StatementEnd

-- +goose StatementBegin
DROP INDEX IF EXISTS idx_users_active;
-- +goose StatementEnd

-- +goose StatementBegin
ALTER TABLE users DROP CONSTRAINT IF EXISTS users_status_chk;
ALTER TABLE users DROP CONSTRAINT IF EXISTS users_role_chk;
-- +goose StatementEnd

-- +goose StatementBegin
ALTER TABLE users DROP COLUMN IF EXISTS deleted_at;
ALTER TABLE users DROP COLUMN IF EXISTS avatar_url;
ALTER TABLE users DROP COLUMN IF EXISTS status;
ALTER TABLE users DROP COLUMN IF EXISTS role;
-- +goose StatementEnd
