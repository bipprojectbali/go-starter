-- +goose Up
-- +goose StatementBegin
-- pass_hash nullable: user yang mendaftar via Google tak punya password.
-- (idempotent secara efektif — DROP NOT NULL aman diulang di data lama.)
ALTER TABLE users ALTER COLUMN pass_hash DROP NOT NULL;
-- +goose StatementEnd

-- +goose StatementBegin
-- email_verified: dari provider OIDC (email_verified claim). Prasyarat auto-link
-- akun (anti pre-account-takeover). Default false untuk user password lama.
ALTER TABLE users ADD COLUMN IF NOT EXISTS email_verified BOOLEAN NOT NULL DEFAULT false;
-- +goose StatementEnd

-- +goose StatementBegin
-- Identitas OAuth terpisah (bukan kolom google_id) agar future-proof multi-provider.
-- Tautan via OIDC "sub" (immutable), BUKAN email (email bisa berubah → takeover).
CREATE TABLE oauth_accounts (
    id           BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    user_id      BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    provider     TEXT NOT NULL,              -- "google"
    provider_uid TEXT NOT NULL,              -- OIDC sub
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    -- Cegah user ganda tiap klik "Sign in with Google": satu identitas provider
    -- hanya boleh menaut ke satu baris.
    UNIQUE (provider, provider_uid)
);
-- +goose StatementEnd

-- +goose StatementBegin
-- Dipakai untuk list/unlink identitas per user.
CREATE INDEX idx_oauth_accounts_user ON oauth_accounts (user_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE oauth_accounts;
-- +goose StatementEnd

-- +goose StatementBegin
ALTER TABLE users DROP COLUMN IF EXISTS email_verified;
-- +goose StatementEnd

-- +goose StatementBegin
-- CATATAN: SET NOT NULL akan GAGAL bila sudah ada user OAuth-only (pass_hash NULL).
-- Itu ekspektasi — rollback perlu backfill/hapus user tersebut lebih dulu.
ALTER TABLE users ALTER COLUMN pass_hash SET NOT NULL;
-- +goose StatementEnd
