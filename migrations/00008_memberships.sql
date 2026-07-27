-- +goose Up
-- +goose StatementBegin
-- Membership many-to-many: SATU user boleh jadi anggota BANYAK workspace dengan
-- role berbeda per-workspace (pola Slack/Notion/GitHub). Menggantikan asumsi
-- "1 user = 1 tenant" dari migrasi 00007 — lihat docs/decisions/0003.
--
-- users jadi tabel GLOBAL (identitas murni: email/pass_hash/avatar) → KELUAR dari
-- RLS. Keanggotaan + role pindah ke sini. Isolasi DATA tetap via tenant_id + GUC
-- di oauth_accounts/audit_logs/activity_presence (policy 00007 TAK berubah).
--
-- memberships SENGAJA TANPA RLS: dibaca middleware Scope untuk MENENTUKAN tenant
-- aktif — kalau ber-RLS, butuh GUC yang justru belum di-set (chicken-and-egg).
-- Keamanan ditegakkan di query (selalu WHERE user_id = <uid sesi>).
CREATE TABLE IF NOT EXISTS memberships (
    id         BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    user_id    BIGINT NOT NULL REFERENCES users(id)   ON DELETE CASCADE,
    tenant_id  BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    role       TEXT NOT NULL DEFAULT 'member'
               CHECK (role IN ('owner','admin','member')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (user_id, tenant_id)   -- satu role per user per workspace
);
CREATE INDEX IF NOT EXISTS idx_memberships_user   ON memberships (user_id);
CREATE INDEX IF NOT EXISTS idx_memberships_tenant ON memberships (tenant_id);
-- +goose StatementEnd

-- +goose StatementBegin
-- Undangan bergabung ke workspace. email boleh milik orang yang BELUM punya akun
-- (itu justru kasus paling umum) → invite tak ber-FK ke users. token = rahasia
-- URL (crypto/rand hex, lihat internal/oauth.NewState). accepted_at NULL = pending.
-- TANPA RLS: dibaca lewat token di jalur publik (penerima belum tentu anggota).
CREATE TABLE IF NOT EXISTS invites (
    id          BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    tenant_id   BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    email       TEXT NOT NULL,
    role        TEXT NOT NULL DEFAULT 'member'
                CHECK (role IN ('admin','member')),  -- owner tak bisa diundang
    token       TEXT NOT NULL UNIQUE,
    invited_by  BIGINT REFERENCES users(id) ON DELETE SET NULL,
    accepted_at TIMESTAMPTZ,
    expires_at  TIMESTAMPTZ NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_invites_token  ON invites (token);
CREATE INDEX IF NOT EXISTS idx_invites_tenant ON invites (tenant_id, accepted_at);
-- +goose StatementEnd

-- +goose StatementBegin
-- Backfill: tiap user existing (1 tenant, role di kolom) → satu baris membership.
-- ON CONFLICT: idempotent bila migrasi dijalankan ulang.
INSERT INTO memberships (user_id, tenant_id, role)
SELECT id, tenant_id, role FROM users WHERE deleted_at IS NULL
ON CONFLICT (user_id, tenant_id) DO NOTHING;
-- +goose StatementEnd

-- +goose StatementBegin
-- Kuota workspace per user (lever monetisasi: free/pro/enterprise). Default dari
-- env MAX_WORKSPACES_PER_USER saat user dibuat; kolom agar bisa di-override
-- per-user oleh platform (super_admin/staff) tanpa ubah env.
ALTER TABLE users ADD COLUMN IF NOT EXISTS workspace_quota INT NOT NULL DEFAULT 3;
-- +goose StatementEnd

-- +goose StatementBegin
-- users KELUAR dari RLS: identitas global, bukan data ber-tenant. Dilakukan SETELAH
-- backfill agar tenant_id/role masih terbaca di atas.
-- CATATAN: ListUsers (panel /dev) jadi benar-benar lintas-workspace — itu memang
-- route platform (super_admin/staff). Daftar anggota per-workspace pakai memberships.
DROP POLICY IF EXISTS tenant_isolation ON users;
ALTER TABLE users NO FORCE ROW LEVEL SECURITY;
ALTER TABLE users DISABLE ROW LEVEL SECURITY;

DROP INDEX IF EXISTS idx_users_tenant;
ALTER TABLE users DROP CONSTRAINT IF EXISTS users_role_chk;
ALTER TABLE users DROP COLUMN IF EXISTS tenant_id;
ALTER TABLE users DROP COLUMN IF EXISTS role;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- Kembalikan kolom users + backfill dari memberships (ambil SATU membership per
-- user — yang terlama, mendekati kondisi "1 user = 1 tenant" sebelumnya).
ALTER TABLE users ADD COLUMN IF NOT EXISTS tenant_id BIGINT REFERENCES tenants(id);
ALTER TABLE users ADD COLUMN IF NOT EXISTS role TEXT NOT NULL DEFAULT 'member';

UPDATE users u SET tenant_id = m.tenant_id, role = m.role
FROM (
    SELECT DISTINCT ON (user_id) user_id, tenant_id, role
    FROM memberships ORDER BY user_id, created_at, id
) m
WHERE m.user_id = u.id AND u.tenant_id IS NULL;

-- User tanpa membership (mis. dibuat setelah migrasi Up) → tenant default pertama,
-- agar SET NOT NULL di bawah tak gagal.
UPDATE users SET tenant_id = (SELECT id FROM tenants ORDER BY id LIMIT 1)
WHERE tenant_id IS NULL;

ALTER TABLE users ALTER COLUMN tenant_id SET NOT NULL;
ALTER TABLE users ADD CONSTRAINT users_role_chk
    CHECK (role IN ('owner','admin','member')) NOT VALID;
CREATE INDEX IF NOT EXISTS idx_users_tenant ON users (tenant_id);
-- +goose StatementEnd

-- +goose StatementBegin
-- Pulihkan RLS users (policy identik dgn 00007).
ALTER TABLE users ENABLE ROW LEVEL SECURITY;
ALTER TABLE users FORCE  ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON users
    USING (COALESCE(current_setting('app.is_super', true),'off')='on'
           OR tenant_id = NULLIF(current_setting('app.tenant_id', true),'')::bigint)
    WITH CHECK (COALESCE(current_setting('app.is_super', true),'off')='on'
           OR tenant_id = NULLIF(current_setting('app.tenant_id', true),'')::bigint);
-- +goose StatementEnd

-- +goose StatementBegin
ALTER TABLE users DROP COLUMN IF EXISTS workspace_quota;
DROP TABLE IF EXISTS invites;
DROP TABLE IF EXISTS memberships;
-- +goose StatementEnd
