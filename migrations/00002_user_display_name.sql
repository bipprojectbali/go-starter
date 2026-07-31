-- +goose Up

-- ═══════════════════════════════════════════════════════════════════════════
-- NAMA TAMPILAN user — mengembalikan data yang selama ini DIBUANG.
--
-- `internal/oauth` sudah membaca claim `name` dari Google sejak awal (ia ikut
-- diverifikasi bersama token), tapi tak ada tempat menyimpannya, jadi nilainya
-- dilempar setiap kali orang login. Akibatnya daftar anggota hanya punya email
-- sebagai penanda orang — dan email adalah PII yang tak punya alasan tampil di
-- layar rekan sekerja.
--
-- `maskEmail` lahir sebagai KOMPENSASI atas ketiadaan kolom ini (docstring-nya
-- menyebutnya terang-terangan). Ia TETAP DIPAKAI setelah migrasi ini, tapi
-- turun pangkat: dari penanda utama jadi cadangan untuk yang tak punya nama.
--
-- NULLABLE, dan itu keputusan: "belum pernah login lewat Google" berbeda dari
-- "namanya memang kosong". Dengan NOT NULL DEFAULT '' keduanya tak bisa
-- dibedakan, dan kode fallback jadi menebak-nebak. Akun password dev serta user
-- lama tak punya nama sama sekali — mereka jatuh ke fallback, bukan ke string
-- kosong yang merender baris hampa.
--
-- Nama TIDAK unik dan tak boleh dijadikan penanda identitas: dua orang bernama
-- sama itu lumrah, dan nilainya dikendalikan user di Google (bisa diisi apa
-- saja, termasuk sesuatu yang menyerupai alamat email orang lain). Yang
-- membedakan baris tetap `id`; aksi anggota tetap memakainya.
-- ═══════════════════════════════════════════════════════════════════════════

-- +goose StatementBegin
ALTER TABLE users ADD COLUMN IF NOT EXISTS name TEXT;

COMMENT ON COLUMN users.name IS
    'Nama tampilan (claim `name` Google). NULL = tak diketahui → tampilan jatuh '
    'ke email tersamarkan. User-controlled: jangan dipakai sebagai penanda unik '
    'maupun untuk keputusan otorisasi.';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE users DROP COLUMN IF EXISTS name;
-- +goose StatementEnd
