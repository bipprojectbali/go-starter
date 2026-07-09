-- +goose Up
-- +goose StatementBegin
-- Presence agregat per-blok-15-menit. Satu baris = (user, bucket 15-mnt UTC).
-- Agregasi terjadi di LEVEL BARIS via UPSERT (ON CONFLICT hits+1) → bukan
-- write-storm: tiap request cuma menaikkan counter baris yang sama, bukan insert
-- baris baru. Menjawab "user aktif jam berapa s/d jam berapa" tanpa event-log
-- bervolume tinggi. bucket_at disimpan UTC (TIMESTAMPTZ); konversi ke jam lokal
-- terjadi saat baca (AT TIME ZONE). user_id CASCADE: presence ikut terhapus bila
-- user hard-delete (data tak relevan lagi).
CREATE TABLE IF NOT EXISTS activity_presence (
    user_id      BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    bucket_at    TIMESTAMPTZ NOT NULL,   -- awal blok 15-menit (UTC), di-floor
    hits         INT NOT NULL DEFAULT 1,
    last_seen_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, bucket_at)
);
-- +goose StatementEnd

-- +goose StatementBegin
-- Query dashboard menyapu rentang waktu lalu GROUP BY jam/hari lintas-user.
-- Index pada bucket_at mempercepat range-scan (WHERE bucket_at >= .. AND < ..).
CREATE INDEX IF NOT EXISTS idx_presence_bucket ON activity_presence (bucket_at DESC);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS activity_presence;
-- +goose StatementEnd
