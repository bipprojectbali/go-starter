-- name: RecordPresence :exec
-- Presence bucket 15-menit. bucket_at di-floor SERVER-SIDE ke kelipatan 900 dtk
-- (jangan hitung di Go — hindari drift clock). Agregasi di level baris: request
-- berulang dalam bucket sama hanya menaikkan hits, bukan insert baris baru.
INSERT INTO activity_presence (user_id, bucket_at, hits, last_seen_at, tenant_id)
VALUES (
    sqlc.arg(user_id)::bigint,
    to_timestamp(floor(extract(epoch FROM now()) / 900) * 900),
    1, now(),
    sqlc.arg(tenant_id)::bigint
)
ON CONFLICT (user_id, bucket_at)
DO UPDATE SET hits = activity_presence.hits + 1, last_seen_at = now();

-- name: PresenceByHour :many
-- Distribusi aktivitas per JAM-LOKAL untuk satu rentang (bar "aktivitas per jam").
-- AT TIME ZONE mengonversi bucket UTC ke jam lokal. Handler mengisi jam kosong
-- jadi 0 (sumbu X ECharts butuh 0..23 lengkap). COALESCE+cast: SUM/COUNT nullable
-- → paksa int64 (bukan pgtype.Numeric).
SELECT
    extract(hour FROM bucket_at AT TIME ZONE sqlc.arg(tz)::text)::int AS hour_local,
    COALESCE(count(DISTINCT user_id), 0)::bigint AS active_users,
    COALESCE(sum(hits), 0)::bigint             AS total_hits
FROM activity_presence
WHERE bucket_at >= sqlc.arg(from_at)::timestamptz
  AND bucket_at <  sqlc.arg(to_at)::timestamptz
GROUP BY hour_local
ORDER BY hour_local;

-- name: PresenceByDay :many
-- Tren per HARI-LOKAL untuk rentang mingguan/bulanan (line chart).
SELECT
    (date_trunc('day', bucket_at AT TIME ZONE sqlc.arg(tz)::text))::date AS day_local,
    COALESCE(count(DISTINCT user_id), 0)::bigint AS active_users,
    COALESCE(sum(hits), 0)::bigint             AS total_hits
FROM activity_presence
WHERE bucket_at >= sqlc.arg(from_at)::timestamptz
  AND bucket_at <  sqlc.arg(to_at)::timestamptz
GROUP BY day_local
ORDER BY day_local;

-- name: PresenceKPIs :one
-- KPI ringkas untuk stat cards pada rentang aktif.
SELECT
    COALESCE(count(DISTINCT user_id), 0)::bigint AS active_users,
    COALESCE(sum(hits), 0)::bigint             AS total_hits
FROM activity_presence
WHERE bucket_at >= sqlc.arg(from_at)::timestamptz
  AND bucket_at <  sqlc.arg(to_at)::timestamptz;

-- name: PresenceUserSpans :many
-- "User aktif jam berapa s/d jam berapa" untuk rentang: first/last seen per user +
-- total hit. JOIN users untuk email TAMPILAN panel (dev-only UI, bukan log — tabel
-- activity_presence sendiri hanya simpan user_id). first/last dikembalikan sebagai
-- timestamptz UTC; handler memformatnya ke jam lokal (appTZ) — lebih bersih dari
-- AT TIME ZONE di SQL (yang bikin sqlc emit interface{}).
SELECT
    ap.user_id,
    u.email,
    min(ap.bucket_at)::timestamptz    AS first_seen,
    max(ap.last_seen_at)::timestamptz AS last_seen,
    COALESCE(sum(ap.hits), 0)::bigint  AS total_hits
FROM activity_presence ap
JOIN users u ON u.id = ap.user_id
WHERE ap.bucket_at >= sqlc.arg(from_at)::timestamptz
  AND ap.bucket_at <  sqlc.arg(to_at)::timestamptz
GROUP BY ap.user_id, u.email
ORDER BY first_seen;

-- name: ListAuthEvents :many
-- Login/logout terbaru (subset audit_logs) untuk tabel aktivitas panel.
SELECT * FROM audit_logs
WHERE action IN ('auth.login', 'auth.logout')
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg(page_size);
