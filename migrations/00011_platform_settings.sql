-- +goose Up
-- +goose StatementBegin
-- platform_settings = pengaturan seluruh platform yang bisa diubah SAAT JALAN,
-- tanpa restart. Sebelumnya kuota workspace hanya ada di env
-- MAX_WORKSPACES_PER_USER — dan env itu cuma dipakai saat user DIBUAT, lalu
-- disalin ke users.workspace_quota. Akibatnya mengubah env tak berpengaruh sama
-- sekali pada user yang sudah ada: itu "nilai awal", bukan "aturan global".
--
-- Key-value (bukan satu kolom per pengaturan): menambah pengaturan berikutnya
-- tak menuntut migrasi baru. Harga yang dibayar — nilai bertipe TEXT dan harus
-- di-parse di Go — sepadan untuk pengaturan yang jumlahnya sedikit & jarang
-- dibaca (di-cache proses, lihat internal/settings).
--
-- TANPA RLS: pengaturan berlaku LINTAS-workspace (preseden platform_staff di
-- 00007). Ditulis hanya lewat jalur platform yang sudah dijaga RequireEnforce.
CREATE TABLE IF NOT EXISTS platform_settings (
    key        TEXT PRIMARY KEY,
    value      TEXT NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_by BIGINT REFERENCES users(id) ON DELETE SET NULL
);
-- +goose StatementEnd

-- +goose StatementBegin
-- NULL = "IKUT DEFAULT GLOBAL". Ini inti perubahannya: dengan INT NOT NULL,
-- mustahil membedakan "kebetulan bernilai 3" dari "sengaja diberi 3" — sehingga
-- menaikkan default global tak bisa menyentuh siapa pun tanpa menimpa override
-- yang mungkin disengaja.
--
-- Setelah ini: NULL → ikut global (berubah seketika saat global diubah),
-- angka → hak khusus milik user itu (kebal perubahan global).
ALTER TABLE users ALTER COLUMN workspace_quota DROP DEFAULT;
ALTER TABLE users ALTER COLUMN workspace_quota DROP NOT NULL;
-- +goose StatementEnd

-- +goose StatementBegin
-- Backfill: baris yang nilainya SAMA dengan default lama (3) hampir pasti tak
-- pernah di-override — nilainya datang dari DEFAULT kolom, bukan dari keputusan
-- siapa pun. Dijadikan NULL agar ikut global.
--
-- Yang nilainya BERBEDA dari 3 dipertahankan apa adanya: itu satu-satunya bukti
-- yang kita punya bahwa seseorang pernah menetapkannya dengan sengaja. Salah
-- arah di sini tak bisa dibatalkan (informasinya hilang), jadi dipilih yang
-- konservatif: hanya yang jelas-jelas bawaan yang dikosongkan.
UPDATE users SET workspace_quota = NULL WHERE workspace_quota = 3;
-- +goose StatementEnd

-- +goose StatementBegin
-- Nilai awal default global. ON CONFLICT: idempotent bila migrasi diulang.
-- Angka 3 = default lama, supaya perilaku TEPAT SEBELUM dan SESUDAH migrasi
-- identik. Mengubahnya jadi keputusan operator lewat panel, bukan efek samping
-- migrasi.
INSERT INTO platform_settings (key, value)
VALUES ('workspace_quota_default', '3')
ON CONFLICT (key) DO NOTHING;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- Kembalikan NULL ke angka konkret sebelum NOT NULL dipasang lagi. Nilainya
-- diambil dari setting global agar hasil down-migrate mencerminkan aturan yang
-- sedang berlaku, bukan angka mati.
UPDATE users
SET workspace_quota = COALESCE(
    (SELECT value::int FROM platform_settings WHERE key = 'workspace_quota_default'), 3)
WHERE workspace_quota IS NULL;
ALTER TABLE users ALTER COLUMN workspace_quota SET NOT NULL;
ALTER TABLE users ALTER COLUMN workspace_quota SET DEFAULT 3;
-- +goose StatementEnd

-- +goose StatementBegin
DROP TABLE IF EXISTS platform_settings;
-- +goose StatementEnd
