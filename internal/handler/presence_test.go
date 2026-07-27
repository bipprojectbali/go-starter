package handler

import (
	"testing"
	"time"

	"go_starter/internal/db"
)

// TestPresenceThrottle_Window: allow() menolak panggilan kedua dalam jendela
// throttle, mengizinkan setelahnya. Ini yang mencegah write-storm (Rule 13).
func TestPresenceThrottle_Window(t *testing.T) {
	p := &presenceTracker{}
	t0 := time.Date(2026, 7, 2, 10, 0, 0, 0, time.UTC)

	if !p.allow(7, t0) {
		t.Fatal("panggilan pertama harus diizinkan")
	}
	if p.allow(7, t0.Add(30*time.Second)) {
		t.Error("dalam jendela throttle (30s < 60s) harus ditolak")
	}
	if !p.allow(7, t0.Add(61*time.Second)) {
		t.Error("setelah jendela throttle harus diizinkan lagi")
	}
	// User berbeda tak saling memengaruhi.
	if !p.allow(8, t0.Add(30*time.Second)) {
		t.Error("user lain harus punya jendela sendiri")
	}
}

// TestRecordPresence_UpsertAggregates: UPSERT dua kali dalam bucket sama →
// SATU baris, hits=2. Ini bukti presence teragregasi (bukan insert-per-request).
func TestRecordPresence_UpsertAggregates(t *testing.T) {
	env, uid := setupTest(t)
	ctx := t.Context()

	for i := 0; i < 3; i++ {
		if err := env.q.RecordPresence(ctx, db.RecordPresenceParams{UserID: uid, TenantID: env.tenantID}); err != nil {
			t.Fatalf("RecordPresence #%d: %v", i, err)
		}
	}

	var rows, hits int
	err := env.h.Pool.QueryRow(ctx,
		"SELECT count(*), COALESCE(sum(hits),0) FROM activity_presence WHERE user_id=$1", uid,
	).Scan(&rows, &hits)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if rows != 1 {
		t.Errorf("harus 1 baris (bucket sama), dapat %d", rows)
	}
	if hits != 3 {
		t.Errorf("hits harus 3 (UPSERT +1 tiap kali), dapat %d", hits)
	}
}
