-- +goose Up

-- ═══════════════════════════════════════════════════════════════════════════
-- SKEMA DASAR go_starter — satu migrasi, satu keadaan awal.
--
-- Menggantikan 11 migrasi bertahap (users → todos → oauth → rbac → activity →
-- drop_todos → tenancy → memberships → notifications → lifecycle → settings).
-- Riwayat itu ditulis seolah ada data produksi yang harus diselamatkan; tak
-- pernah ada. Yang tersisa hanyalah biayanya:
--
--   • tabel `todos` dibuat lalu dibuang dalam riwayat yang sama;
--   • tenant "default" di-seed sebagai wadah backfill data yang tak pernah ada
--     — lalu setiap DB baru punya satu workspace ber-slug salah, dan kode boot
--     harus mengadopsinya;
--   • backfill `workspace_quota = 3 → NULL` menebak-nebak niat user yang nol.
--
-- Menyatukannya HANYA mungkin selama belum ada deployment. Setelah ada, jendela
-- ini tertutup permanen dan migrasi berikutnya wajib inkremental seperti biasa.
--
-- Urutan di bawah mengikuti ketergantungan FK: users & tenants lebih dulu,
-- sisanya menyusul.
-- ═══════════════════════════════════════════════════════════════════════════

-- +goose StatementBegin
-- users = identitas GLOBAL, lintas-workspace. SENGAJA tanpa tenant_id & tanpa
-- role: orang yang sama bisa owner di satu workspace dan member di workspace
-- lain, jadi keduanya milik `memberships`, bukan milik orang.
--
-- Konsekuensi yang harus diingat: tabel ini di LUAR RLS. Yang menjaganya adalah
-- filter query, dan daftar anggota workspace WAJIB lewat memberships.
CREATE TABLE users (
    id             BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    email          TEXT NOT NULL UNIQUE,
    pass_hash      TEXT,                            -- NULL = akun OAuth-only
    email_verified BOOLEAN NOT NULL DEFAULT false,
    status         TEXT NOT NULL DEFAULT 'active',  -- gate login, ortogonal dari role
    avatar_url     TEXT,                            -- claim `picture` dari Google
    deleted_at     TIMESTAMPTZ,                     -- soft-delete
    -- NULL = IKUT DEFAULT GLOBAL (platform_settings), angka = hak khusus yang
    -- KEBAL perubahan global. Pembedaan itu mustahil dengan kolom NOT NULL:
    -- "kebetulan bernilai 3" tak bisa dibedakan dari "sengaja diberi 3", dan
    -- menaikkan default global akan menimpa override yang mungkin disengaja.
    workspace_quota INTEGER,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT users_status_chk CHECK (status IN ('active','disabled','blocked'))
);
-- Partial: hanya user hidup. Sekaligus pengingat gotcha soft-delete — query
-- yang lupa `deleted_at IS NULL` tak akan memakai indeks ini.
CREATE INDEX idx_users_active ON users (created_at DESC, id DESC) WHERE deleted_at IS NULL;
-- +goose StatementEnd

-- +goose StatementBegin
-- tenants = workspace. Di mode single tetap ada TEPAT SATU baris di sini: mode
-- single adalah multi-tenant dengan N=1, bukan jalur kode kedua.
CREATE TABLE tenants (
    id             BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    name           TEXT NOT NULL,
    -- Slug IMMUTABLE setelah dibuat: ia ada di setiap URL yang sudah tersebar.
    -- Tak dilepas pula saat workspace terhapus — kalau dilepas, orang lain bisa
    -- mengambilnya dan restore jadi mustahil.
    slug           TEXT NOT NULL UNIQUE,
    -- Bedanya KEWENANGAN, bukan rasa: `suspended` = tindakan PLATFORM (owner tak
    -- bisa membatalkannya sendiri — kalau bisa, gunanya hilang); `archived` =
    -- keputusan OWNER (harus bisa dibuka lagi tanpa memohon).
    status         TEXT NOT NULL DEFAULT 'active',
    -- Workspace PRIMER: rumah aplikasi itu sendiri. Di mode single inilah
    -- satu-satunya workspace; setelah naik ke multi ia tetap ada sebagai satu di
    -- antara banyak, sehingga tak ada tautan yang mati saat mode berubah.
    --
    -- Kolom, bukan perbandingan slug yang tersebar di banyak tempat: yang
    -- bergantung padanya adalah penolakan arsip/hapus, dan aturan sepenting itu
    -- tak boleh bergantung pada string yang kebetulan cocok.
    is_primary     BOOLEAN NOT NULL DEFAULT false,
    deleted_at     TIMESTAMPTZ,                     -- soft-delete, tenggang 30 hari
    suspended_at   TIMESTAMPTZ,
    suspended_by   BIGINT REFERENCES users(id) ON DELETE SET NULL,
    suspend_reason TEXT,
    archived_at    TIMESTAMPTZ,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT tenants_status_check CHECK (status IN ('active','suspended','archived'))
);
CREATE INDEX idx_tenants_live    ON tenants (id)         WHERE deleted_at IS NULL;
CREATE INDEX idx_tenants_deleted ON tenants (deleted_at) WHERE deleted_at IS NOT NULL;
-- TEPAT SATU primer. Unique partial, bukan pemeriksaan di Go: dua workspace
-- primer berarti dua "rumah aplikasi", dan yang menemukannya adalah user yang
-- tiba-tiba mendarat di tempat lain.
CREATE UNIQUE INDEX idx_tenants_primary ON tenants ((true)) WHERE is_primary;
-- +goose StatementEnd

-- +goose StatementBegin
-- memberships = user × tenant × role. INI tempat role tenant hidup.
--
-- SENGAJA TANPA RLS: tabel ini justru dibaca untuk MENENTUKAN scope — ia tak
-- bisa bergantung pada GUC yang belum di-set (chicken-and-egg). Keamanannya
-- dari filter query (`WHERE user_id = <uid sesi>`), dengan test isolasi
-- antar-user sebagai pengganti jaring RLS.
CREATE TABLE memberships (
    id         BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    user_id    BIGINT NOT NULL REFERENCES users(id)   ON DELETE CASCADE,
    tenant_id  BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    role       TEXT NOT NULL DEFAULT 'member',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (user_id, tenant_id),
    CONSTRAINT memberships_role_check CHECK (role IN ('owner','admin','member'))
);
CREATE INDEX idx_memberships_user   ON memberships (user_id);
CREATE INDEX idx_memberships_tenant ON memberships (tenant_id);
-- +goose StatementEnd

-- +goose StatementBegin
-- oauth_accounts — identitas eksternal. tenant_id disimpan sebagai konteks
-- pendaftaran (workspace tempat akun ini pertama kali muncul).
CREATE TABLE oauth_accounts (
    id           BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    user_id      BIGINT NOT NULL REFERENCES users(id)   ON DELETE CASCADE,
    tenant_id    BIGINT NOT NULL REFERENCES tenants(id),
    provider     TEXT NOT NULL,
    provider_uid TEXT NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (provider, provider_uid)
);
CREATE INDEX idx_oauth_accounts_user ON oauth_accounts (user_id);
CREATE INDEX idx_oauth_tenant        ON oauth_accounts (tenant_id);
-- +goose StatementEnd

-- +goose StatementBegin
-- audit_logs — append-only. metadata JSONB TANPA PII: simpan id, bukan email.
--
-- tenant_id NULLABLE + ON DELETE SET NULL, dan itu KEPUTUSAN, bukan kelonggaran:
-- dengan NOT NULL tanpa CASCADE, `DELETE FROM tenants` GAGAL di tengah jalan —
-- setelah memberships/invites/notifications terlanjur terhapus. Bukti tak boleh
-- lenyap bersama yang dibuktikan.
--
-- target_id BUKAN FK: ia harus tetap terbaca setelah targetnya hard-delete.
CREATE TABLE audit_logs (
    id            BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    actor_user_id BIGINT REFERENCES users(id)   ON DELETE SET NULL,
    tenant_id     BIGINT REFERENCES tenants(id) ON DELETE SET NULL,
    action        TEXT NOT NULL,                 -- "user.role.update", "auth.login", ...
    target_type   TEXT NOT NULL,
    target_id     BIGINT,
    metadata      JSONB NOT NULL DEFAULT '{}',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_audit_actor  ON audit_logs (actor_user_id, created_at DESC);
CREATE INDEX idx_audit_tenant ON audit_logs (tenant_id, created_at DESC);
-- +goose StatementEnd

-- +goose StatementBegin
-- activity_presence — kehadiran ter-AGREGASI, bukan satu baris per request.
-- Kunci primer (user_id, bucket_at) membuat UPSERT bucket 15-menit menaikkan
-- `hits`, sehingga beban tulis tak tumbuh seiring lalu lintas (Rule 13).
--
-- bucket_at & last_seen_at TIMESTAMPTZ (UTC). "Jam berapa user aktif" dikonversi
-- ke waktu lokal saat AGREGASI (`AT TIME ZONE`), tak pernah disimpan lokal.
CREATE TABLE activity_presence (
    user_id      BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    bucket_at    TIMESTAMPTZ NOT NULL,
    tenant_id    BIGINT NOT NULL REFERENCES tenants(id),
    hits         INTEGER NOT NULL DEFAULT 1,
    last_seen_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, bucket_at)
);
CREATE INDEX idx_presence_bucket ON activity_presence (bucket_at DESC);
CREATE INDEX idx_presence_tenant ON activity_presence (tenant_id, bucket_at DESC);
-- +goose StatementEnd

-- +goose StatementBegin
-- invites — undangan bergabung. TANPA RLS: dibuka di jalur PUBLIK (penerima
-- belum jadi anggota, jadi belum punya scope). Penjaganya token rahasia.
CREATE TABLE invites (
    id          BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    tenant_id   BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    email       TEXT NOT NULL,
    role        TEXT NOT NULL DEFAULT 'member',
    token       TEXT NOT NULL UNIQUE,
    invited_by  BIGINT REFERENCES users(id) ON DELETE SET NULL,
    accepted_at TIMESTAMPTZ,
    expires_at  TIMESTAMPTZ NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    -- `owner` TIDAK bisa diundang: kepemilikan diserahkan lewat panel anggota
    -- oleh owner yang ada, bukan lewat tautan yang bisa diteruskan siapa pun.
    CONSTRAINT invites_role_check CHECK (role IN ('admin','member'))
);
CREATE INDEX idx_invites_tenant ON invites (tenant_id, accepted_at);
CREATE INDEX idx_invites_token  ON invites (token);
CREATE INDEX idx_invites_email  ON invites (lower(email)) WHERE accepted_at IS NULL;
-- +goose StatementEnd

-- +goose StatementBegin
-- notifications — TANPA RLS, dan alasannya BERBEDA dari memberships/invites.
-- Bukan chicken-and-egg, tapi SEMANTIK: notifikasi milik USER dan lintas-
-- workspace. Undangan justru datang dari workspace yang BELUM jadi miliknya,
-- jadi policy `tenant_id = GUC` akan menyembunyikan persis inti fiturnya.
--
-- Karena itu tenant_id di sini NULLABLE dan hanya KONTEKS TAMPILAN — bukan kunci
-- isolasi. Konsekuensi: jangan pernah dibaca lewat query ter-scope satu tenant.
CREATE TABLE notifications (
    id         BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    user_id    BIGINT NOT NULL REFERENCES users(id)   ON DELETE CASCADE,
    tenant_id  BIGINT REFERENCES tenants(id) ON DELETE CASCADE,
    kind       TEXT NOT NULL,
    payload    JSONB NOT NULL DEFAULT '{}',
    read_at    TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT notifications_kind_check
        CHECK (kind IN ('member.role.changed','member.removed','workspace.joined'))
);
CREATE INDEX idx_notif_user   ON notifications (user_id, created_at DESC, id DESC);
CREATE INDEX idx_notif_unread ON notifications (user_id) WHERE read_at IS NULL;
-- +goose StatementEnd

-- +goose StatementBegin
-- platform_staff — operator platform berskala TERBATAS (di bawah super_admin).
-- Lingkupnya platform, bukan tenant → TANPA RLS, sehingga terbaca baik di jalur
-- ber-tenant maupun jalur super.
--
-- super_admin sendiri TIDAK ada di sini: ia ENV-ONLY (SUPER_ADMIN_EMAILS), nol
-- baris DB. Otoritas tertinggi yang bisa ditulis lewat aplikasi adalah otoritas
-- yang bisa dinaikkan lewat aplikasi.
CREATE TABLE platform_staff (
    id         BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    email      TEXT NOT NULL UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
-- +goose StatementEnd

-- +goose StatementBegin
-- platform_settings — pengaturan seluruh platform yang bisa diubah SAAT JALAN.
-- Key-value supaya pengaturan berikutnya tak menuntut migrasi baru; harganya
-- (nilai TEXT, di-parse di Go) sepadan untuk yang jumlahnya sedikit & di-cache.
CREATE TABLE platform_settings (
    key        TEXT PRIMARY KEY,
    value      TEXT NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_by BIGINT REFERENCES users(id) ON DELETE SET NULL
);
-- +goose StatementEnd

-- +goose StatementBegin
-- RATCHET MODE — single boleh naik ke multi; multi TAK PERNAH turun.
--
-- Mode dulu hidup di env APP_MODE. Env bisa dibalik: menurunkannya kembali ke
-- `single` setelah ada banyak workspace akan menyembunyikan sisanya — kehilangan
-- data yang tampak seperti bug UI. Perlindungannya dulu berupa pemeriksaan saat
-- boot, yaitu aturan yang harus diingat dan dijalankan dengan benar.
--
-- Di sini keadaannya pindah ke DB dan penurunannya ditolak oleh DATABASE. Bukan
-- "dicegah oleh kode yang ingat memeriksa" — mustahil secara konstruksi, apa pun
-- yang dilakukan aplikasi di atasnya.
-- DUA trigger, bukan satu. UPDATE saja tidak cukup: baris yang absen berarti
-- "single" bagi aplikasi, jadi DELETE adalah penurunan yang menyamar — dan
-- `DELETE; INSERT 'single'` melewati trigger UPDATE tanpa menyentuhnya sama
-- sekali. Terbukti bisa saat diuji; ditutup di sini.
CREATE FUNCTION forbid_tenancy_downgrade() RETURNS trigger AS $$
BEGIN
    IF OLD.key <> 'tenancy_mode' OR OLD.value <> 'multi' THEN
        RETURN COALESCE(NEW, OLD);   -- bukan urusan trigger ini
    END IF;
    IF TG_OP = 'DELETE' THEN
        RAISE EXCEPTION
            'baris tenancy_mode tak boleh dihapus setelah bernilai multi: ketiadaannya dibaca sebagai mode single';
    END IF;
    IF NEW.value <> 'multi' THEN
        RAISE EXCEPTION
            'tenancy_mode tak bisa turun dari multi ke %: workspace selain yang primer akan lenyap dari pandangan tanpa jejak',
            NEW.value;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_forbid_tenancy_downgrade
    BEFORE UPDATE ON platform_settings
    FOR EACH ROW EXECUTE FUNCTION forbid_tenancy_downgrade();

CREATE TRIGGER trg_forbid_tenancy_delete
    BEFORE DELETE ON platform_settings
    FOR EACH ROW EXECUTE FUNCTION forbid_tenancy_downgrade();
-- +goose StatementEnd

-- +goose StatementBegin
-- Role runtime least-privilege. NOLOGIN — SENGAJA, dan inilah bedanya dari
-- rancangan sebelumnya: role ini tak pernah dipakai untuk connect, melainkan
-- DIMASUKI di dalam transaksi lewat `SET LOCAL ROLE app_rw` (lihat
-- internal/db/tenant.go). Terverifikasi: di dalam tx, hak turun ke app_rw
-- (superuser pun tercabut) dan pulih otomatis saat COMMIT maupun ROLLBACK —
-- jadi tak ada yang bocor ke peminjam pool berikutnya.
--
-- Akibatnya: SATU DSN saja. Tak ada APP_DATABASE_URL, tak ada password kedua
-- untuk dilupakan atau bocor, tak ada entri userlist.txt di PgBouncer. Dan RLS
-- ikut mengikat di DEV — query yang lupa `WHERE tenant_id` gagal di laptop,
-- bukan di produksi.
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'app_rw') THEN
        CREATE ROLE app_rw NOLOGIN NOBYPASSRLS;
    END IF;
END $$;

GRANT USAGE ON SCHEMA public TO app_rw;
GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO app_rw;
GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA public TO app_rw;
-- Tabel dari migrasi BERIKUTNYA terjangkau otomatis — tanpa ini, setiap migrasi
-- baru diam-diam menciptakan tabel yang tak bisa dibaca runtime.
ALTER DEFAULT PRIVILEGES IN SCHEMA public
    GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO app_rw;
ALTER DEFAULT PRIVILEGES IN SCHEMA public
    GRANT USAGE, SELECT ON SEQUENCES TO app_rw;

-- WAJIB: `SET LOCAL ROLE app_rw` hanya diizinkan bila role saat ini ANGGOTA
-- app_rw. Superuser lolos otomatis, tapi owner biasa TIDAK — tanpa baris ini
-- deployment yang memakai owner non-superuser gagal di setiap transaksi.
GRANT app_rw TO CURRENT_USER;
-- +goose StatementEnd

-- +goose StatementBegin
-- ENABLE + FORCE RLS. FORCE = pemilik tabel pun terkena policy; tanpanya owner
-- bypass diam-diam dan seluruh jaring ini hanya dekorasi.
ALTER TABLE oauth_accounts    ENABLE ROW LEVEL SECURITY;
ALTER TABLE oauth_accounts    FORCE  ROW LEVEL SECURITY;
ALTER TABLE audit_logs        ENABLE ROW LEVEL SECURITY;
ALTER TABLE audit_logs        FORCE  ROW LEVEL SECURITY;
ALTER TABLE activity_presence ENABLE ROW LEVEL SECURITY;
ALTER TABLE activity_presence FORCE  ROW LEVEL SECURITY;
-- +goose StatementEnd

-- +goose StatementBegin
-- Policy dua-GUC: app.is_super='on' (jalur platform) ATAU tenant_id cocok.
-- current_setting(...,true) → NULL bila GUC absen; NULLIF/COALESCE membuat
-- "GUC absen" berarti TAK ADA BARIS, bukan semua baris.
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

-- ═══════════════════════════════════════════════════════════════════════════
-- TIDAK ADA SEED DI SINI — dan itu disengaja.
--
-- Riwayat lama men-seed tenant "default" sebagai wadah backfill, sehingga setiap
-- DB baru lahir dengan satu workspace ber-slug salah yang harus diadopsi kode
-- boot. Workspace primer kini dibuat aplikasi saat startup (BootstrapPrimary),
-- dengan nama dari APP_NAME — tempat yang tahu namanya, bukan migrasi yang tak
-- tahu apa-apa tentang aplikasi di atasnya.
-- ═══════════════════════════════════════════════════════════════════════════

-- +goose Down
-- +goose StatementBegin
DROP TRIGGER IF EXISTS trg_forbid_tenancy_delete ON platform_settings;
DROP TRIGGER IF EXISTS trg_forbid_tenancy_downgrade ON platform_settings;
DROP FUNCTION IF EXISTS forbid_tenancy_downgrade();
DROP TABLE IF EXISTS platform_settings;
DROP TABLE IF EXISTS platform_staff;
DROP TABLE IF EXISTS notifications;
DROP TABLE IF EXISTS invites;
DROP TABLE IF EXISTS activity_presence;
DROP TABLE IF EXISTS audit_logs;
DROP TABLE IF EXISTS oauth_accounts;
DROP TABLE IF EXISTS memberships;
DROP TABLE IF EXISTS tenants;
DROP TABLE IF EXISTS users;
-- Role app_rw SENGAJA tak di-drop: ia bisa dipakai database lain di cluster yang
-- sama, dan DROP ROLE yang gagal karena masih ada dependensi akan menggagalkan
-- seluruh rollback.
-- +goose StatementEnd
