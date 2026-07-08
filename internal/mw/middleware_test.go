package mw

import (
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"time"

	"go_stater/internal/session"

	"github.com/alexedwards/scs/v2"
	"github.com/alexedwards/scs/v2/memstore"
)

// --- RequestID ---

func TestRequestID_SetsHeaderAndContext(t *testing.T) {
	var seen string
	h := RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = RequestIDFromContext(r.Context())
	}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	if seen == "" {
		t.Fatal("request id tak masuk context")
	}
	// 8 byte → 16 hex.
	if !regexp.MustCompile(`^[0-9a-f]{16}$`).MatchString(seen) {
		t.Errorf("id harus 16 hex: %q", seen)
	}
	if got := rec.Header().Get("X-Request-ID"); got != seen {
		t.Errorf("header X-Request-ID=%q, context=%q — harus sama", got, seen)
	}
}

func TestRequestIDFromContext_Missing(t *testing.T) {
	if id := RequestIDFromContext(context.Background()); id != "" {
		t.Errorf("context kosong harus \"\", got %q", id)
	}
}

// --- Recover ---

func discardLogger() (*slog.Logger, *bytes.Buffer) {
	var buf bytes.Buffer
	return slog.New(slog.NewTextHandler(&buf, nil)), &buf
}

func TestRecover_Returns500AndLogs(t *testing.T) {
	log, buf := discardLogger()
	h := Recover(log)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("boom")
	}))
	rec := httptest.NewRecorder()
	// Tak boleh panic keluar.
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("panic harus 500, got %d", rec.Code)
	}
	if !strings.Contains(buf.String(), "boom") && !strings.Contains(buf.String(), "panic") {
		t.Errorf("panic harus ter-log:\n%s", buf.String())
	}
}

func TestRecover_AbortHandlerRepanics(t *testing.T) {
	log, _ := discardLogger()
	h := Recover(log)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic(http.ErrAbortHandler)
	}))
	defer func() {
		if rec := recover(); rec != http.ErrAbortHandler {
			t.Errorf("ErrAbortHandler harus di-re-panic, got %v", rec)
		}
	}()
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))
	t.Error("seharusnya sudah panic sebelum sini")
}

func TestRecover_NoPanicPassthrough(t *testing.T) {
	log, buf := discardLogger()
	h := Recover(log)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusOK {
		t.Errorf("normal harus 200, got %d", rec.Code)
	}
	if strings.Contains(buf.String(), "panic") {
		t.Error("tak boleh log panic untuk request normal")
	}
}

// --- RequestLog ---

func TestRequestLog_SkipsHealth(t *testing.T) {
	log, buf := discardLogger()
	h := RequestLog(log)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	for _, p := range []string{"/healthz", "/readyz"} {
		h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, p, nil))
	}
	if buf.Len() != 0 {
		t.Errorf("health probe tak boleh di-log:\n%s", buf.String())
	}
}

func TestRequestLog_CapturesStatus(t *testing.T) {
	log, buf := discardLogger()
	h := RequestLog(log)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/foo", nil))
	out := buf.String()
	if !strings.Contains(out, "status=404") {
		t.Errorf("status 404 harus ter-log:\n%s", out)
	}
	if !strings.Contains(out, "/foo") {
		t.Errorf("path harus ter-log:\n%s", out)
	}
}

func TestStatusRecorder_Unwrap(t *testing.T) {
	// Unwrap wajib untuk http.ResponseController (SSE flush) — regresi.
	orig := httptest.NewRecorder()
	sr := &statusRecorder{ResponseWriter: orig, status: 200}
	if sr.Unwrap() != orig {
		t.Error("Unwrap harus kembalikan writer asli")
	}
}

// --- RequireAuth ---

func authHarness(t *testing.T) *scs.SessionManager {
	t.Helper()
	sm := scs.New()
	sm.Store = memstore.New()
	sm.Lifetime = time.Hour
	session.Init(sm)
	return sm
}

func TestRequireAuth_Anonymous_HTTPRedirect(t *testing.T) {
	sm := authHarness(t)
	reached := false
	guarded := RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { reached = true }))
	rec := httptest.NewRecorder()
	sm.LoadAndSave(guarded).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/todos", nil))

	if rec.Code != http.StatusSeeOther {
		t.Errorf("anonim harus 303, got %d", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != "/login" {
		t.Errorf("harus redirect /login, got %q", loc)
	}
	if reached {
		t.Error("handler tak boleh tercapai untuk anonim")
	}
}

func TestRequireAuth_Anonymous_Datastar(t *testing.T) {
	sm := authHarness(t)
	guarded := RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	req := httptest.NewRequest(http.MethodGet, "/todos", nil)
	req.Header.Set("Datastar-Request", "true")
	rec := httptest.NewRecorder()
	sm.LoadAndSave(guarded).ServeHTTP(rec, req)

	// Datastar → balas SSE (redirect event), bukan 303.
	if !strings.Contains(rec.Body.String(), "/login") {
		t.Errorf("Datastar anonim harus SSE redirect ke /login:\n%s", rec.Body.String())
	}
}

func TestRequireAuth_Authenticated(t *testing.T) {
	sm := authHarness(t)
	reached := false
	guarded := RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reached = true
		w.WriteHeader(http.StatusOK)
	}))
	wrapped := sm.LoadAndSave(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		session.SetUserID(r.Context(), 1)
		guarded.ServeHTTP(w, r)
	}))
	rec := httptest.NewRecorder()
	wrapped.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/todos", nil))

	if !reached || rec.Code != http.StatusOK {
		t.Errorf("user login harus lolos, reached=%v code=%d", reached, rec.Code)
	}
}
