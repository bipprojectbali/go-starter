package handler

import (
	"testing"
	"time"

	"go_stater/internal/db"

	"github.com/jackc/pgx/v5/pgtype"
)

// seedPresence menyisipkan satu baris presence dengan bucket_at eksplisit (UTC).
func seedPresence(t *testing.T, env *testEnv, uid int64, bucket time.Time, hits int) {
	t.Helper()
	_, err := env.h.Pool.Exec(t.Context(),
		`INSERT INTO activity_presence (user_id, bucket_at, hits, last_seen_at)
		 VALUES ($1, $2, $3, $2)`,
		uid, bucket, hits)
	if err != nil {
		t.Fatalf("seed presence: %v", err)
	}
}

// TestPresenceByHour_TimezoneShift: bucket UTC dikelompokkan ke JAM LOKAL.
// Bucket 23:00 UTC = 06:00 WIB (UTC+7) → harus muncul di hour_local=6, bukan 23.
// Ini guard off-by-one timezone (gotcha paling rawan).
func TestPresenceByHour_TimezoneShift(t *testing.T) {
	env, uid := setupTest(t)
	ctx := t.Context()

	// 2 Jul 23:00 UTC → 3 Jul 06:00 WIB.
	seedPresence(t, env, uid, time.Date(2026, 7, 2, 23, 0, 0, 0, time.UTC), 5)

	from := pgtype.Timestamptz{Time: time.Date(2026, 7, 2, 0, 0, 0, 0, time.UTC), Valid: true}
	to := pgtype.Timestamptz{Time: time.Date(2026, 7, 4, 0, 0, 0, 0, time.UTC), Valid: true}
	rows, err := env.h.DB.PresenceByHour(ctx, db.PresenceByHourParams{
		Tz: "Asia/Jakarta", FromAt: from, ToAt: to,
	})
	if err != nil {
		t.Fatalf("PresenceByHour: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("harus 1 grup jam, dapat %d: %+v", len(rows), rows)
	}
	if rows[0].HourLocal != 6 {
		t.Errorf("bucket 23:00 UTC harus jam 6 WIB, dapat %d", rows[0].HourLocal)
	}
	if rows[0].TotalHits != 5 {
		t.Errorf("total_hits harus 5, dapat %d", rows[0].TotalHits)
	}
}

// TestPresenceKPIs_SumsAndDistinct: KPI agregasi benar (distinct user + sum hits).
func TestPresenceKPIs_SumsAndDistinct(t *testing.T) {
	env, uid := setupTest(t)
	ctx := t.Context()
	u2, err := env.h.DB.CreateUser(ctx, db.CreateUserParams{Email: "u2@local", PassHash: ptr("x")})
	if err != nil {
		t.Fatalf("seed u2: %v", err)
	}

	base := time.Date(2026, 7, 2, 10, 0, 0, 0, time.UTC)
	seedPresence(t, env, uid, base, 3)
	seedPresence(t, env, uid, base.Add(time.Hour), 2) // user sama, bucket beda
	seedPresence(t, env, u2.ID, base, 4)

	from := pgtype.Timestamptz{Time: time.Date(2026, 7, 2, 0, 0, 0, 0, time.UTC), Valid: true}
	to := pgtype.Timestamptz{Time: time.Date(2026, 7, 3, 0, 0, 0, 0, time.UTC), Valid: true}
	kpi, err := env.h.DB.PresenceKPIs(ctx, db.PresenceKPIsParams{FromAt: from, ToAt: to})
	if err != nil {
		t.Fatalf("PresenceKPIs: %v", err)
	}
	if kpi.ActiveUsers != 2 {
		t.Errorf("active_users (distinct) harus 2, dapat %d", kpi.ActiveUsers)
	}
	if kpi.TotalHits != 9 {
		t.Errorf("total_hits harus 9 (3+2+4), dapat %d", kpi.TotalHits)
	}
}

// TestPresenceKPIs_RangeExcludesOutside: baris di luar [from,to) tak dihitung.
func TestPresenceKPIs_RangeExcludesOutside(t *testing.T) {
	env, uid := setupTest(t)
	ctx := t.Context()

	inside := time.Date(2026, 7, 2, 12, 0, 0, 0, time.UTC)
	outside := time.Date(2026, 7, 5, 12, 0, 0, 0, time.UTC)
	seedPresence(t, env, uid, inside, 3)
	seedPresence(t, env, uid, outside, 99)

	from := pgtype.Timestamptz{Time: time.Date(2026, 7, 2, 0, 0, 0, 0, time.UTC), Valid: true}
	to := pgtype.Timestamptz{Time: time.Date(2026, 7, 3, 0, 0, 0, 0, time.UTC), Valid: true}
	kpi, err := env.h.DB.PresenceKPIs(ctx, db.PresenceKPIsParams{FromAt: from, ToAt: to})
	if err != nil {
		t.Fatalf("PresenceKPIs: %v", err)
	}
	if kpi.TotalHits != 3 {
		t.Errorf("hanya baris dalam rentang dihitung (3), dapat %d", kpi.TotalHits)
	}
}
