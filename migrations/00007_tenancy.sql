-- +goose Up
-- +goose StatementBegin
-- Multi-tenancy: satu deployment, banyak tenant (organisasi). 1 user = 1 tenant.
-- Isolasi via Postgres Row-Level Security (RLS) — DB yang enforce, "lupa WHERE
-- tenant_id" bukan lagi kebocoran. Lihat docs/arch + CLAUDE.md gotcha RLS.
CREATE TABLE IF NOT EXISTS tenants (
    id         BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    name       TEXT NOT NULL,
    slug       TEXT NOT NULL UNIQUE,          -- subdomain/URL-safe id
    status     TEXT NOT NULL DEFAULT 'active', -- active|suspended
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
-- +goose StatementEnd

-- +goose StatementBegin
-- platform_staff: operator platform (support) — MUTABLE, dikelola super_admin.
-- BUKAN anggota tenant (tak punya tenant_id). super_admin sendiri TIDAK di sini
-- (env-only via SUPER_ADMIN_EMAILS, immutable). Tabel ini TAK ber-RLS (platform-scope).
CREATE TABLE IF NOT EXISTS platform_staff (
    id         BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    email      TEXT NOT NULL UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
-- +goose StatementEnd

-- +goose StatementBegin
-- Tenant default untuk backfill data existing (single-tenant → multi).
INSERT INTO tenants (name, slug)
SELECT 'Default', 'default'
WHERE NOT EXISTS (SELECT 1 FROM tenants WHERE slug = 'default');
-- +goose StatementEnd

-- +goose StatementBegin
-- tenant_id NULLABLE dulu (biar ALTER tak gagal di tabel berisi data), backfill,
-- baru SET NOT NULL. Semua tabel ber-data terikat tenant.
ALTER TABLE users             ADD COLUMN IF NOT EXISTS tenant_id BIGINT REFERENCES tenants(id);
ALTER TABLE oauth_accounts    ADD COLUMN IF NOT EXISTS tenant_id BIGINT REFERENCES tenants(id);
ALTER TABLE audit_logs        ADD COLUMN IF NOT EXISTS tenant_id BIGINT REFERENCES tenants(id);
ALTER TABLE activity_presence ADD COLUMN IF NOT EXISTS tenant_id BIGINT REFERENCES tenants(id);
-- +goose StatementEnd

-- +goose StatementBegin
-- Backfill semua data existing ke tenant default.
DO $$
DECLARE dft BIGINT;
BEGIN
    SELECT id INTO dft FROM tenants WHERE slug = 'default';
    UPDATE users             SET tenant_id = dft WHERE tenant_id IS NULL;
    UPDATE oauth_accounts    SET tenant_id = dft WHERE tenant_id IS NULL;
    UPDATE audit_logs        SET tenant_id = dft WHERE tenant_id IS NULL;
    UPDATE activity_presence SET tenant_id = dft WHERE tenant_id IS NULL;
END $$;
-- +goose StatementEnd

-- +goose StatementBegin
ALTER TABLE users             ALTER COLUMN tenant_id SET NOT NULL;
ALTER TABLE oauth_accounts    ALTER COLUMN tenant_id SET NOT NULL;
ALTER TABLE audit_logs        ALTER COLUMN tenant_id SET NOT NULL;
ALTER TABLE activity_presence ALTER COLUMN tenant_id SET NOT NULL;
-- +goose StatementEnd

-- +goose StatementBegin
-- Index tenant-first (query ter-scope RLS memfilter tenant_id implisit).
CREATE INDEX IF NOT EXISTS idx_users_tenant    ON users (tenant_id);
CREATE INDEX IF NOT EXISTS idx_oauth_tenant    ON oauth_accounts (tenant_id);
CREATE INDEX IF NOT EXISTS idx_audit_tenant    ON audit_logs (tenant_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_presence_tenant ON activity_presence (tenant_id, bucket_at DESC);
-- +goose StatementEnd

-- +goose StatementBegin
-- role CHECK users diperbarui: role tenant 'owner','admin','member'. Platform role
-- (super_admin/staff) TAK di kolom ini (env/platform_staff). Backfill role lama:
--   'user'→'member', 'admin'→'admin', 'super_admin'→'owner' (super_admin lama =
--   root env, tetap super_admin efektif; kolom di-set 'owner' agar valid).
ALTER TABLE users DROP CONSTRAINT IF EXISTS users_role_chk;
UPDATE users SET role = 'member' WHERE role = 'user';
UPDATE users SET role = 'owner'  WHERE role = 'super_admin';
ALTER TABLE users ALTER COLUMN role SET DEFAULT 'member';
ALTER TABLE users ADD CONSTRAINT users_role_chk
    CHECK (role IN ('owner','admin','member')) NOT VALID;
-- +goose StatementEnd

-- +goose StatementBegin
-- Least-privilege app role: NON-OWNER, NOBYPASSRLS → policy MENGIKAT runtime.
-- Migrator/boot connect sebagai owner (DATABASE_URL) → bypass, jalankan DDL/reconcile.
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'app_rw') THEN
        CREATE ROLE app_rw NOLOGIN NOBYPASSRLS;
    END IF;
END $$;
GRANT USAGE ON SCHEMA public TO app_rw;
GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO app_rw;
GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA public TO app_rw;
ALTER DEFAULT PRIVILEGES IN SCHEMA public
    GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO app_rw;
-- +goose StatementEnd

-- +goose StatementBegin
-- ENABLE + FORCE RLS. FORCE = pemilik tabel pun kena policy (kecuali superuser/
-- BYPASSRLS). App connect app_rw (non-owner) → policy mengikat.
ALTER TABLE users             ENABLE ROW LEVEL SECURITY;
ALTER TABLE users             FORCE  ROW LEVEL SECURITY;
ALTER TABLE oauth_accounts    ENABLE ROW LEVEL SECURITY;
ALTER TABLE oauth_accounts    FORCE  ROW LEVEL SECURITY;
ALTER TABLE audit_logs        ENABLE ROW LEVEL SECURITY;
ALTER TABLE audit_logs        FORCE  ROW LEVEL SECURITY;
ALTER TABLE activity_presence ENABLE ROW LEVEL SECURITY;
ALTER TABLE activity_presence FORCE  ROW LEVEL SECURITY;
-- +goose StatementEnd

-- +goose StatementBegin
-- Policy dua-GUC: app.is_super='on' (bypass platform) ATAU tenant_id cocok.
-- current_setting(...,true) missing_ok → NULL bila GUC absen; COALESCE/NULLIF
-- amankan (absen = deny tenant match, tapi is_super check tetap jalan).
CREATE POLICY tenant_isolation ON users
    USING (COALESCE(current_setting('app.is_super', true),'off')='on'
           OR tenant_id = NULLIF(current_setting('app.tenant_id', true),'')::bigint)
    WITH CHECK (COALESCE(current_setting('app.is_super', true),'off')='on'
           OR tenant_id = NULLIF(current_setting('app.tenant_id', true),'')::bigint);
CREATE POLICY tenant_isolation ON oauth_accounts
    USING (COALESCE(current_setting('app.is_super', true),'off')='on'
           OR tenant_id = NULLIF(current_setting('app.tenant_id', true),'')::bigint)
    WITH CHECK (COALESCE(current_setting('app.is_super', true),'off')='on'
           OR tenant_id = NULLIF(current_setting('app.tenant_id', true),'')::bigint);
CREATE POLICY tenant_isolation ON audit_logs
    USING (COALESCE(current_setting('app.is_super', true),'off')='on'
           OR tenant_id = NULLIF(current_setting('app.tenant_id', true),'')::bigint)
    WITH CHECK (COALESCE(current_setting('app.is_super', true),'off')='on'
           OR tenant_id = NULLIF(current_setting('app.tenant_id', true),'')::bigint);
CREATE POLICY tenant_isolation ON activity_presence
    USING (COALESCE(current_setting('app.is_super', true),'off')='on'
           OR tenant_id = NULLIF(current_setting('app.tenant_id', true),'')::bigint)
    WITH CHECK (COALESCE(current_setting('app.is_super', true),'off')='on'
           OR tenant_id = NULLIF(current_setting('app.tenant_id', true),'')::bigint);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP POLICY IF EXISTS tenant_isolation ON activity_presence;
DROP POLICY IF EXISTS tenant_isolation ON audit_logs;
DROP POLICY IF EXISTS tenant_isolation ON oauth_accounts;
DROP POLICY IF EXISTS tenant_isolation ON users;
-- +goose StatementEnd

-- +goose StatementBegin
ALTER TABLE activity_presence NO FORCE ROW LEVEL SECURITY;
ALTER TABLE activity_presence DISABLE ROW LEVEL SECURITY;
ALTER TABLE audit_logs        NO FORCE ROW LEVEL SECURITY;
ALTER TABLE audit_logs        DISABLE ROW LEVEL SECURITY;
ALTER TABLE oauth_accounts    NO FORCE ROW LEVEL SECURITY;
ALTER TABLE oauth_accounts    DISABLE ROW LEVEL SECURITY;
ALTER TABLE users             NO FORCE ROW LEVEL SECURITY;
ALTER TABLE users             DISABLE ROW LEVEL SECURITY;
-- +goose StatementEnd

-- +goose StatementBegin
-- Kembalikan role CHECK lama (user/admin/super_admin) + backfill balik.
ALTER TABLE users DROP CONSTRAINT IF EXISTS users_role_chk;
UPDATE users SET role = 'user' WHERE role = 'member';
UPDATE users SET role = 'super_admin' WHERE role = 'owner';
ALTER TABLE users ALTER COLUMN role SET DEFAULT 'user';
ALTER TABLE users ADD CONSTRAINT users_role_chk
    CHECK (role IN ('user','admin','super_admin')) NOT VALID;
-- +goose StatementEnd

-- +goose StatementBegin
ALTER TABLE activity_presence DROP COLUMN IF EXISTS tenant_id;
ALTER TABLE audit_logs        DROP COLUMN IF EXISTS tenant_id;
ALTER TABLE oauth_accounts    DROP COLUMN IF EXISTS tenant_id;
ALTER TABLE users             DROP COLUMN IF EXISTS tenant_id;
DROP TABLE IF EXISTS platform_staff;
DROP TABLE IF EXISTS tenants;
-- +goose StatementEnd
