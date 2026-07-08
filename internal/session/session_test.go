package session

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/alexedwards/scs/v2"
	"github.com/alexedwards/scs/v2/memstore"
)

// withSession menjalankan fn dalam request yang sudah melewati scs LoadAndSave
// (context punya session data). Memstore — tanpa Redis.
func withSession(t *testing.T, fn func(ctx context.Context, w http.ResponseWriter)) *httptest.ResponseRecorder {
	t.Helper()
	sm := scs.New()
	sm.Store = memstore.New()
	sm.Lifetime = time.Hour
	Init(sm)

	rec := httptest.NewRecorder()
	h := sm.LoadAndSave(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fn(r.Context(), w)
	}))
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	return rec
}

func TestSetIdentity_RoundTrip(t *testing.T) {
	withSession(t, func(ctx context.Context, _ http.ResponseWriter) {
		SetIdentity(ctx, 7, "u@x.com", "admin", true, "https://a/av")
		if UserID(ctx) != 7 {
			t.Errorf("UserID=%d, want 7", UserID(ctx))
		}
		if Email(ctx) != "u@x.com" {
			t.Errorf("Email=%q", Email(ctx))
		}
		if Role(ctx) != "admin" {
			t.Errorf("Role=%q", Role(ctx))
		}
		if !IsRoot(ctx) {
			t.Error("IsRoot harus true")
		}
		if AvatarURL(ctx) != "https://a/av" {
			t.Errorf("AvatarURL=%q", AvatarURL(ctx))
		}
	})
}

func TestSession_ZeroValues(t *testing.T) {
	withSession(t, func(ctx context.Context, _ http.ResponseWriter) {
		if UserID(ctx) != 0 {
			t.Error("UserID default 0")
		}
		if Email(ctx) != "" || Role(ctx) != "" || AvatarURL(ctx) != "" {
			t.Error("string default kosong")
		}
		if IsRoot(ctx) {
			t.Error("IsRoot default false")
		}
	})
}

func TestSetUserID_OnlySetsUserID(t *testing.T) {
	withSession(t, func(ctx context.Context, _ http.ResponseWriter) {
		SetUserID(ctx, 5)
		if UserID(ctx) != 5 {
			t.Errorf("UserID=%d", UserID(ctx))
		}
		// SetUserID tak menyentuh role/email (beda dgn SetIdentity).
		if Role(ctx) != "" || Email(ctx) != "" {
			t.Error("SetUserID tak boleh set role/email")
		}
	})
}

func TestOAuthFlow_RoundTripAndClear(t *testing.T) {
	withSession(t, func(ctx context.Context, _ http.ResponseWriter) {
		PutOAuthFlow(ctx, "st", "no", "vf")
		st, no, vf := OAuthFlow(ctx)
		if st != "st" || no != "no" || vf != "vf" {
			t.Errorf("OAuthFlow=(%q,%q,%q)", st, no, vf)
		}
		// One-time use: clear → semua kosong.
		ClearOAuthFlow(ctx)
		st, no, vf = OAuthFlow(ctx)
		if st != "" || no != "" || vf != "" {
			t.Errorf("setelah clear harus kosong: (%q,%q,%q)", st, no, vf)
		}
	})
}

func TestWriteCookie_EmitsSetCookie(t *testing.T) {
	rec := withSession(t, func(ctx context.Context, w http.ResponseWriter) {
		SetUserID(ctx, 1)
		if err := WriteCookie(ctx, w); err != nil {
			t.Fatalf("WriteCookie: %v", err)
		}
	})
	// Regresi bug scs+SSE: cookie WAJIB terkirim manual.
	if c := rec.Header().Get("Set-Cookie"); c == "" {
		t.Error("WriteCookie harus menulis Set-Cookie header")
	}
}
