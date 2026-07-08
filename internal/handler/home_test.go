package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestHome_Anonymous: route "/" harus melayani landing page untuk pengunjung
// anonim — 200 OK, BUKAN redirect ke /login (inti fitur ini).
func TestHome_Anonymous(t *testing.T) {
	env, _ := setupTest(t)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := env.do(req, env.h.Home)

	if rec.Code != http.StatusOK {
		t.Fatalf("landing anonim harus 200 (bukan redirect), got %d", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != "" {
		t.Errorf("landing tidak boleh redirect, tapi Location=%q", loc)
	}
	body := rec.Body.String()
	// CTA anonim mengarah ke /login (adaptif: Google + password bila dev).
	if !strings.Contains(body, `href="/login"`) {
		t.Errorf("landing anonim harus tampil CTA Masuk (/login):\n%s", body)
	}
	if strings.Contains(body, `href="/todos"`) {
		t.Errorf("landing anonim tidak boleh tampil link /todos:\n%s", body)
	}
}

// TestHome_LoggedIn: user login melihat CTA menuju aplikasi (/todos), bukan
// tombol daftar.
func TestHome_LoggedIn(t *testing.T) {
	env, uid := setupTest(t)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := env.doAuthed(uid, req, env.h.Home)

	if rec.Code != http.StatusOK {
		t.Fatalf("landing login harus 200, got %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `href="/todos"`) {
		t.Errorf("landing user login harus tampil link ke /todos:\n%s", body)
	}
	if strings.Contains(body, `href="/register"`) {
		t.Errorf("landing user login tidak perlu tombol daftar:\n%s", body)
	}
}
