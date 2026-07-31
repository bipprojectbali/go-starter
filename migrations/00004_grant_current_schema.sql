-- +goose Up
-- +goose StatementBegin

-- 00001 memberikan hak app_rw ke `SCHEMA public` secara harfiah. Itu benar untuk
-- produksi (yang memang memakai public), tapi ia diam-diam mengunci migrasi ke
-- satu schema: dijalankan di schema lain, tabelnya terbuat dan app_rw TAK punya
-- akses apa pun ke sana.
--
-- Kegagalannya menyesatkan. Tabel ada, migrasi sukses, tapi setiap transaksi
-- aplikasi mati di `SET LOCAL ROLE app_rw` + query pertama dengan "permission
-- denied" — yang terbaca seperti masalah RLS, bukan masalah GRANT.
--
-- Yang membuat ini mendesak: test kini menjalankan migrasi di schema TERPISAH
-- per paket agar bisa paralel (lihat internal/testdb). Tapi manfaatnya melampaui
-- test — deployment yang menaruh aplikasi di schema non-public juga bekerja.
--
-- current_schema(), bukan nama harfiah: yang benar adalah "schema tempat migrasi
-- ini sedang berjalan", dan itu satu-satunya jawaban yang tetap benar di semua
-- tempat. format(%I) mengutip pengenalnya dengan aman.
DO $$
DECLARE
    sch text := current_schema();
BEGIN
    EXECUTE format('GRANT USAGE ON SCHEMA %I TO app_rw', sch);
    EXECUTE format('GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA %I TO app_rw', sch);
    EXECUTE format('GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA %I TO app_rw', sch);
    -- Tabel dari migrasi BERIKUTNYA terjangkau otomatis. Tanpa baris ini, tiap
    -- migrasi baru diam-diam membuat tabel yang tak bisa dibaca runtime —
    -- kegagalan yang baru muncul saat fiturnya dipakai, jauh dari sebabnya.
    EXECUTE format('ALTER DEFAULT PRIVILEGES IN SCHEMA %I '
                   'GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO app_rw', sch);
    EXECUTE format('ALTER DEFAULT PRIVILEGES IN SCHEMA %I '
                   'GRANT USAGE, SELECT ON SEQUENCES TO app_rw', sch);
END $$;

-- Idempoten & aman diulang: GRANT yang sudah ada tak menghasilkan error, dan di
-- schema public ini sekadar mengulang apa yang 00001 sudah lakukan.

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- Tak ada yang dipulihkan: mencabut GRANT ini akan mematikan runtime, dan
-- rollback yang membuat aplikasi tak bisa jalan bukan rollback.
SELECT 1;
-- +goose StatementEnd
