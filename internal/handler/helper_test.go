package handler

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"testing"
	"time"

	"go_stater/internal/db"
	"go_stater/internal/session"

	"github.com/alexedwards/scs/v2"
	"github.com/alexedwards/scs/v2/memstore"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// testEnv menampung Handler + session manager untuk test.
type testEnv struct {
	h  *Handler
	sm *scs.SessionManager
}

// setupTest menyiapkan Handler + user seed + session manager (memstore, bukan
// Redis — test tak butuh Redis). Skip bila TEST_DATABASE_URL kosong.
func setupTest(t *testing.T) (*testEnv, int64) {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL tidak di-set; lewati test yang butuh DB")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("pool: %v", err)
	}
	t.Cleanup(pool.Close)

	q := db.New(pool)
	// audit_logs (actor SET NULL) & activity_presence (FK CASCADE dari users, tapi
	// eksplisit agar isolasi test pasti) ikut dibersihkan.
	if _, err := pool.Exec(ctx, "TRUNCATE activity_presence, audit_logs, todos, users RESTART IDENTITY CASCADE"); err != nil {
		t.Fatalf("truncate: %v", err)
	}
	u, err := q.CreateUser(ctx, db.CreateUserParams{Email: "test@local", PassHash: ptr("x")})
	if err != nil {
		t.Fatalf("seed user: %v", err)
	}

	// Session manager pakai memstore agar test tak butuh Redis.
	sm := scs.New()
	sm.Store = memstore.New()
	sm.Lifetime = time.Hour
	session.Init(sm)

	h := &Handler{DB: q, Pool: pool, Log: slog.New(slog.NewTextHandler(io.Discard, nil))}
	return &testEnv{h: h, sm: sm}, u.ID
}

// doAuthed menjalankan handler dalam konteks session dengan user login.
// scs butuh request melewati LoadAndSave agar context punya session data;
// kita bungkus handler dengan LoadAndSave lalu inject userID lewat pre-handler.
func (e *testEnv) doAuthed(userID int64, req *http.Request, fn http.HandlerFunc) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	wrapped := e.sm.LoadAndSave(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if userID != 0 {
			session.SetUserID(r.Context(), userID)
		}
		fn(w, r)
	}))
	wrapped.ServeHTTP(rec, req)
	return rec
}

// do menjalankan handler tanpa session user (anonim), tetap lewat LoadAndSave.
func (e *testEnv) do(req *http.Request, fn http.HandlerFunc) *httptest.ResponseRecorder {
	return e.doAuthed(0, req, fn)
}

// withChiParam menyisipkan URL param chi (mis. {id}) ke request untuk test.
func withChiParam(r *http.Request, key, val string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add(key, val)
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
}

func itoa(n int64) string { return strconv.FormatInt(n, 10) }

// ptr mengembalikan pointer ke nilai — untuk field nullable sqlc (mis. PassHash *string).
func ptr[T any](v T) *T { return &v }
