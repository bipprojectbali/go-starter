-- +goose Up
-- +goose StatementBegin
-- Siklus hidup workspace (keputusan 0005). Kolom `tenants.status` SUDAH ada sejak
-- 00007 dgn komentar "active|suspended" — tapi TAK PERNAH dibaca kode mana pun.
-- Kolom yang berbohong lebih berbahaya daripada yang tak ada: ia tampak seperti
-- pengaman terpasang. Di sini ia diberi CHECK + nilai ketiga, lalu (di Go)
-- ditegakkan di middleware Scope.
--
-- TIGA keadaan, dibedakan oleh SIAPA yang berwenang — bukan oleh rasa:
--   suspended → tindakan PLATFORM terhadap workspace (tunggakan/penyalahgunaan).
--               Owner TIDAK boleh membatalkannya sendiri, kalau bisa gunanya hilang.
--   archived  → keputusan OWNER bahwa pekerjaannya selesai. Owner harus bisa
--               membuka kembali tanpa memohon ke siapa pun.
-- Menyatukannya jadi satu "pause" pasti gagal di salah satu sisi kewenangan.
ALTER TABLE tenants
    ADD CONSTRAINT tenants_status_check
    CHECK (status IN ('active', 'suspended', 'archived'));
-- +goose StatementEnd

-- +goose StatementBegin
-- Soft-delete = dimensi ORTOGONAL terhadap status (workspace bisa dihapus saat
-- sedang di-suspend), mengikuti preseden users yang memisahkan status & deleted_at.
-- Masa tenggang: baris tetap ada, slug TIDAK dilepas (UNIQUE tetap berlaku) —
-- kalau dilepas, orang lain bisa mengambil slug itu & restore jadi mustahil.
ALTER TABLE tenants ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ;

-- Jejak SIAPA & KAPAN untuk tiap peralihan. Bukan hiasan: "kenapa workspace saya
-- mati" adalah pertanyaan support pertama, dan jawabannya harus ada di baris ini
-- tanpa perlu menggali audit log.
ALTER TABLE tenants ADD COLUMN IF NOT EXISTS suspended_at TIMESTAMPTZ;
ALTER TABLE tenants ADD COLUMN IF NOT EXISTS suspended_by BIGINT REFERENCES users(id) ON DELETE SET NULL;
ALTER TABLE tenants ADD COLUMN IF NOT EXISTS suspend_reason TEXT;
ALTER TABLE tenants ADD COLUMN IF NOT EXISTS archived_at TIMESTAMPTZ;

-- Daftar workspace HIDUP dibaca di tiap render sidebar (switcher) → partial index
-- agar baris terhapus tak ikut dipindai.
CREATE INDEX IF NOT EXISTS idx_tenants_live ON tenants (id) WHERE deleted_at IS NULL;
-- Purge terjadwal memindai berdasar umur penghapusan.
CREATE INDEX IF NOT EXISTS idx_tenants_deleted ON tenants (deleted_at) WHERE deleted_at IS NOT NULL;
-- +goose StatementEnd

-- +goose StatementBegin
-- BUKTI TAK BOLEH LENYAP BERSAMA YANG DIBUKTIKAN. audit_logs.tenant_id semula
-- NOT NULL + FK tanpa CASCADE — akibatnya DELETE FROM tenants GAGAL di tengah
-- jalan: memberships/invites/notifications sudah ikut CASCADE terhapus, lalu
-- error FK audit membatalkan sisanya. Justru pada peristiwa terpenting
-- (penghapusan workspace) jejaknya paling dibutuhkan.
--
-- Dibuat NULLABLE dulu — SET NULL mustahil pada kolom NOT NULL.
ALTER TABLE audit_logs ALTER COLUMN tenant_id DROP NOT NULL;
ALTER TABLE audit_logs DROP CONSTRAINT IF EXISTS audit_logs_tenant_id_fkey;
ALTER TABLE audit_logs
    ADD CONSTRAINT audit_logs_tenant_id_fkey
    FOREIGN KEY (tenant_id) REFERENCES tenants(id) ON DELETE SET NULL;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- Kembalikan FK audit ke bentuk semula. Baris ber-tenant_id NULL (sisa workspace
-- yang sudah dipurge) TAK BISA dijadikan NOT NULL lagi — dihapus, karena tanpa
-- tenant ia memang tak punya tempat di skema lama.
DELETE FROM audit_logs WHERE tenant_id IS NULL;
ALTER TABLE audit_logs DROP CONSTRAINT IF EXISTS audit_logs_tenant_id_fkey;
ALTER TABLE audit_logs
    ADD CONSTRAINT audit_logs_tenant_id_fkey
    FOREIGN KEY (tenant_id) REFERENCES tenants(id);
ALTER TABLE audit_logs ALTER COLUMN tenant_id SET NOT NULL;
-- +goose StatementEnd

-- +goose StatementBegin
DROP INDEX IF EXISTS idx_tenants_deleted;
DROP INDEX IF EXISTS idx_tenants_live;
-- Workspace terarsip kembali aktif: nilai 'archived' tak ada di skema lama.
UPDATE tenants SET status = 'active' WHERE status = 'archived';
ALTER TABLE tenants DROP CONSTRAINT IF EXISTS tenants_status_check;
ALTER TABLE tenants DROP COLUMN IF EXISTS archived_at;
ALTER TABLE tenants DROP COLUMN IF EXISTS suspend_reason;
ALTER TABLE tenants DROP COLUMN IF EXISTS suspended_by;
ALTER TABLE tenants DROP COLUMN IF EXISTS suspended_at;
ALTER TABLE tenants DROP COLUMN IF EXISTS deleted_at;
-- +goose StatementEnd
