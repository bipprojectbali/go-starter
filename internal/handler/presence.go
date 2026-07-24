package handler

import (
	"context"
	"net/http"
	"sync"
	"time"

	"go_stater/internal/db"
	"go_stater/internal/session"
)

// presenceThrottle = jendela minimum antar-UPSERT presence per user. Bucket DB
// sudah 15-menit, tapi tanpa throttle user yang klik-klik memicu banyak UPSERT ke
// baris yang sama (boros walau tak menambah baris). 60 dtk memangkas itu ~99%.
const presenceThrottle = 60 * time.Second

// presenceTracker menyimpan waktu UPSERT terakhir per user (in-process). Tidak
// persisten antar-restart/instance — itu BENAR: throttle murni optimasi, bukan
// correctness. Setelah restart, UPSERT jalan lagi (data tetap akurat).
type presenceTracker struct {
	last sync.Map // user_id (int64) -> time.Time
}

// allow melaporkan apakah user boleh menulis presence sekarang (di luar jendela
// throttle), dan menandai waktu bila boleh. now di-parameter agar bisa di-test.
func (p *presenceTracker) allow(userID int64, now time.Time) bool {
	if v, ok := p.last.Load(userID); ok {
		if now.Sub(v.(time.Time)) < presenceThrottle {
			return false
		}
	}
	p.last.Store(userID, now)
	return true
}

// tracker global (satu instance untuk lifetime proses).
var tracker = &presenceTracker{}

// TrackPresence merekam jejak kehadiran user (untuk panel aktivitas). Pasang
// SETELAH RefreshIdentity (butuh uid valid). No-op untuk anonim. FAIL-SOFT: error
// atau panik saat rekam TIDAK pernah menggagalkan request user — hanya di-log
// (user_id saja, no PII). UPSERT throttled agar tak jadi write-storm (Rule 13).
func (h *Handler) TrackPresence(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		uid := session.UserID(ctx)
		if uid != 0 && tracker.allow(uid, time.Now()) {
			h.recordPresence(ctx, uid)
		}
		next.ServeHTTP(w, r)
	})
}

// recordPresence melakukan UPSERT presence, dibungkus recover agar panik driver
// pun tak merambat ke request. Error di-log, tak dikembalikan.
func (h *Handler) recordPresence(ctx context.Context, uid int64) {
	defer func() {
		if rec := recover(); rec != nil {
			h.Log.Error("presence: panic", "user_id", uid, "recover", rec)
		}
	}()
	// TenantID dari session (0 utk platform user — presence-nya tercatat lintas
	// scope via WithSuper tx). RLS WITH CHECK menolak tenant_id yang tak cocok
	// GUC, jadi nilai session HARUS = tenant tx (dijamin Scope middleware).
	if err := h.q(ctx).RecordPresence(ctx, db.RecordPresenceParams{
		UserID:   uid,
		TenantID: session.TenantID(ctx),
	}); err != nil {
		h.Log.Error("presence: record", "user_id", uid, "err", err)
	}
}
