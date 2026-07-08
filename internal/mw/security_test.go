package mw

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSecurityHeaders_CSP(t *testing.T) {
	h := SecurityHeaders(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	csp := rec.Header().Get("Content-Security-Policy")
	if csp == "" {
		t.Fatal("CSP header wajib ada")
	}

	// REGRESI KRITIS: Datastar butuh 'unsafe-eval' (new Function untuk evaluasi
	// ekspresi data-*). Tanpa ini SELURUH Datastar mati di browser — @post,
	// signals, patch tak jalan. Jangan pernah hapus.
	if !strings.Contains(csp, "script-src 'self' 'unsafe-eval'") {
		t.Errorf("CSP script-src HARUS punya 'unsafe-eval' (Datastar mati tanpa ini):\n%s", csp)
	}
	// Font Basecoat (data-uri) tak boleh diblok.
	if !strings.Contains(csp, "font-src 'self' data:") {
		t.Errorf("CSP harus izinkan font-src data::\n%s", csp)
	}
	// Avatar Google.
	if !strings.Contains(csp, "googleusercontent.com") {
		t.Errorf("CSP img-src harus izinkan googleusercontent:\n%s", csp)
	}

	// Header keamanan lain tetap ada.
	if rec.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Error("X-Content-Type-Options: nosniff wajib")
	}
	if rec.Header().Get("X-Frame-Options") != "DENY" {
		t.Error("X-Frame-Options: DENY wajib")
	}
}
