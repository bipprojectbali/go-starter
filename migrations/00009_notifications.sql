-- +goose Up
-- +goose StatementBegin
-- Notifikasi = umpan peristiwa milik SATU USER, lintas-workspace. Wadah bagi
-- undangan (yang sebelumnya hanya bisa ditemukan lewat token di URL) + kabar
-- keanggotaan (role diubah, dikeluarkan).
--
-- SENGAJA TANPA RLS — preseden memberships/users (00008): tabel yang di-key per
-- user_id dan memang lintas-workspace keluar dari RLS. Kalau ber-RLS, notifikasi
-- dari workspace SELAIN yang sedang aktif akan tersembunyi — padahal justru itu
-- inti fiturnya (undangan datang dari workspace yang belum jadi milik user).
-- Keamanan ditegakkan di query: SELALU `WHERE user_id = <uid sesi>` (diuji
-- eksplisit di notifications_test.go, karena di sini tak ada jaring RLS).
--
-- tenant_id = KONTEKS tampilan ("dari workspace mana"), BUKAN kunci isolasi.
-- NULLABLE: ada peristiwa yang tak terikat workspace.
CREATE TABLE IF NOT EXISTS notifications (
    id         BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    user_id    BIGINT NOT NULL REFERENCES users(id)   ON DELETE CASCADE,
    tenant_id  BIGINT          REFERENCES tenants(id) ON DELETE CASCADE,
    kind       TEXT NOT NULL
               CHECK (kind IN ('member.role.changed','member.removed','workspace.joined')),
    -- payload = detail siap-render (role lama/baru, nama workspace). Disimpan
    -- sebagai SNAPSHOT: pesan lama harus tetap terbaca apa adanya walau data
    -- sumbernya berubah/terhapus kemudian.
    payload    JSONB NOT NULL DEFAULT '{}'::jsonb,
    read_at    TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
-- Keyset pagination (pola ListUsers): urutan & cursor (created_at DESC, id DESC).
CREATE INDEX IF NOT EXISTS idx_notif_user
    ON notifications (user_id, created_at DESC, id DESC);
-- Partial: badge dirender di SETIAP halaman → hitungan belum-terbaca harus murah.
CREATE INDEX IF NOT EXISTS idx_notif_unread
    ON notifications (user_id) WHERE read_at IS NULL;
-- +goose StatementEnd

-- +goose StatementBegin
-- Undangan dicari per-EMAIL penerima (bukan user_id): saat diundang, penerima
-- belum tentu punya akun. Email dinormalisasi ke lowercase di Go sebelum insert
-- (pola auth.go/invite.go), tapi index expression ini yang membuat pencarian
-- lower(email) tetap ter-index — termasuk untuk baris lama yang mungkin belum
-- ternormalisasi. Partial: hanya undangan yang masih pending yang pernah dicari.
CREATE INDEX IF NOT EXISTS idx_invites_email
    ON invites (lower(email)) WHERE accepted_at IS NULL;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_invites_email;
DROP INDEX IF EXISTS idx_notif_unread;
DROP INDEX IF EXISTS idx_notif_user;
DROP TABLE IF EXISTS notifications;
-- +goose StatementEnd
