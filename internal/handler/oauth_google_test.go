package handler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"go_starter/internal/db"
	"go_starter/internal/oauth"
	"go_starter/internal/session"
)

// errEmailNotVerified mensimulasikan penolakan VerifyIDToken saat email_verified=false.
var errEmailNotVerified = errors.New("oauth: email belum terverifikasi provider")

// stubProvider mengimplementasikan googleProvider tanpa memanggil Google.
// Exchange mengembalikan rawIDToken palsu; VerifyIDToken mengembalikan claims
// yang di-set test (atau error) — memungkinkan uji jalur sukses & penolakan.
type stubProvider struct {
	claims           *oauth.Claims
	verifyErr        error
	exchangeErr      error
	gotNonce         string // nonce yang diterima VerifyIDToken (untuk assert)
	gotSelectAccount bool   // nilai selectAccount yang diterima AuthURL (untuk assert)
}

func (s *stubProvider) AuthURL(state, nonce, verifier string, selectAccount bool) string {
	s.gotSelectAccount = selectAccount
	return "https://accounts.google.com/o/oauth2/v2/auth?state=" + state
}
func (s *stubProvider) Exchange(ctx context.Context, code, verifier string) (string, error) {
	if s.exchangeErr != nil {
		return "", s.exchangeErr
	}
	return "raw-id-token", nil
}
func (s *stubProvider) VerifyIDToken(ctx context.Context, rawIDToken, wantNonce string) (*oauth.Claims, error) {
	s.gotNonce = wantNonce
	if s.verifyErr != nil {
		return nil, s.verifyErr
	}
	return s.claims, nil
}

// doCallback menjalankan GoogleCallback dalam satu session yang sudah di-set
// flow OAuth (state/nonce/verifier). queryState = nilai ?state= di callback.
// Mengembalikan recorder untuk assert status/redirect.
func (e *testEnv) doCallback(t *testing.T, storedState, queryState string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/auth/callback/google?code=abc&state="+queryState, nil)
	wrapped := e.sm.LoadAndSave(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulasikan state yang tadi disimpan GoogleLogin.
		session.PutOAuthFlow(r.Context(), storedState, "the-nonce", "the-verifier")
		e.h.GoogleCallback(w, r)
	}))
	wrapped.ServeHTTP(rec, req)
	return rec
}

func TestGoogleLogin_SwitchAccount(t *testing.T) {
	env, _ := setupTest(t)
	stub := &stubProvider{}
	SetGoogleOAuth(stub)
	t.Cleanup(func() { SetGoogleOAuth(nil) })

	// ?switch=1 → jalur "Ganti akun": handler harus meminta pemilih akun.
	stub.gotSelectAccount = false
	rec := env.do(httptest.NewRequest(http.MethodGet, "/api/auth/google?switch=1", nil), env.h.GoogleLogin)
	if rec.Code != http.StatusFound {
		t.Fatalf("GoogleLogin harus redirect 302, got %d", rec.Code)
	}
	if !stub.gotSelectAccount {
		t.Error("?switch=1 harus meneruskan selectAccount=true ke AuthURL")
	}

	// Login normal (tanpa ?switch) → flow mulus, TANPA pemilih akun.
	stub.gotSelectAccount = true // set true dulu → pastikan handler benar-benar mereset
	env.do(httptest.NewRequest(http.MethodGet, "/api/auth/google", nil), env.h.GoogleLogin)
	if stub.gotSelectAccount {
		t.Error("login normal tak boleh set selectAccount=true")
	}
}

func TestGoogleCallback_StateMismatch(t *testing.T) {
	env, _ := setupTest(t)
	SetGoogleOAuth(&stubProvider{claims: &oauth.Claims{Sub: "g1", Email: "a@x.com", EmailVerified: true}})
	t.Cleanup(func() { SetGoogleOAuth(nil) })

	rec := env.doCallback(t, "real-state", "attacker-state")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("state mismatch harus 400, got %d", rec.Code)
	}
}

func TestGoogleCallback_EmailNotVerified(t *testing.T) {
	env, _ := setupTest(t)
	// VerifyIDToken menolak email belum terverifikasi → kembalikan error.
	SetGoogleOAuth(&stubProvider{verifyErr: errEmailNotVerified})
	t.Cleanup(func() { SetGoogleOAuth(nil) })

	rec := env.doCallback(t, "s1", "s1")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("email tak terverifikasi harus 401, got %d", rec.Code)
	}
}

func TestGoogleCallback_NewUser(t *testing.T) {
	env, _ := setupTest(t)
	sub := "google-sub-123"
	SetGoogleOAuth(&stubProvider{claims: &oauth.Claims{Sub: sub, Email: "baru@gmail.com", EmailVerified: true}})
	t.Cleanup(func() { SetGoogleOAuth(nil) })

	rec := env.doCallback(t, "s1", "s1")
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("login sukses harus redirect 303, got %d: %s", rec.Code, rec.Body.String())
	}
	if loc := rec.Header().Get("Location"); loc != "/w/baru" {
		t.Errorf("user Google baru = owner workspace baru → redirect ke ruang kerjanya, got %q", loc)
	}
	// User baru + oauth_account terbuat.
	u, err := env.q.GetUserByEmail(t.Context(), "baru@gmail.com")
	if err != nil {
		t.Fatalf("user Google tidak terbuat: %v", err)
	}
	if u.PassHash != nil {
		t.Errorf("user Google tak boleh punya pass_hash, got %v", *u.PassHash)
	}
	if !u.EmailVerified {
		t.Errorf("user Google harus email_verified=true")
	}
	acc, err := env.q.GetOAuthAccount(t.Context(), db.GetOAuthAccountParams{Provider: "google", ProviderUid: sub})
	if err != nil {
		t.Fatalf("oauth_account tidak terbuat: %v", err)
	}
	if acc.UserID != u.ID {
		t.Errorf("oauth_account tertaut ke user salah: got %d want %d", acc.UserID, u.ID)
	}
}

func TestGoogleCallback_AutoLinkExistingEmail(t *testing.T) {
	env, _ := setupTest(t)
	// Buat dulu akun password (dev) dengan email sama.
	existing := env.seedMember(t, "dev@gmail.com", "member", 0)
	var err error
	if err != nil {
		t.Fatalf("seed existing: %v", err)
	}
	sub := "google-sub-link"
	SetGoogleOAuth(&stubProvider{claims: &oauth.Claims{Sub: sub, Email: "dev@gmail.com", EmailVerified: true}})
	t.Cleanup(func() { SetGoogleOAuth(nil) })

	rec := env.doCallback(t, "s1", "s1")
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("auto-link harus sukses 303, got %d: %s", rec.Code, rec.Body.String())
	}
	// TIDAK boleh buat user baru — oauth_account tertaut ke user yang ada.
	acc, err := env.q.GetOAuthAccount(t.Context(), db.GetOAuthAccountParams{Provider: "google", ProviderUid: sub})
	if err != nil {
		t.Fatalf("oauth_account tidak terbuat: %v", err)
	}
	if acc.UserID != existing.ID {
		t.Errorf("auto-link ke user salah: got %d want %d (existing)", acc.UserID, existing.ID)
	}
}

func TestGoogleCallback_ExistingOAuthNoDuplicate(t *testing.T) {
	env, _ := setupTest(t)
	sub := "google-sub-repeat"
	stub := &stubProvider{claims: &oauth.Claims{Sub: sub, Email: "repeat@gmail.com", EmailVerified: true}}
	SetGoogleOAuth(stub)
	t.Cleanup(func() { SetGoogleOAuth(nil) })

	// Login Google pertama → buat user.
	env.doCallback(t, "s1", "s1")
	first, err := env.q.GetUserByEmail(t.Context(), "repeat@gmail.com")
	if err != nil {
		t.Fatalf("login pertama gagal: %v", err)
	}
	// Login Google kedua (sub sama) → TIDAK boleh buat user ganda.
	rec := env.doCallback(t, "s2", "s2")
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("login kedua harus sukses, got %d", rec.Code)
	}
	accts, err := env.q.ListOAuthAccountsByUser(t.Context(), first.ID)
	if err != nil {
		t.Fatalf("list oauth: %v", err)
	}
	if len(accts) != 1 {
		t.Errorf("UNIQUE(provider,provider_uid) gagal cegah ganda: %d oauth_account", len(accts))
	}
}

func TestGoogleCallback_NonceForwarded(t *testing.T) {
	env, _ := setupTest(t)
	stub := &stubProvider{claims: &oauth.Claims{Sub: "g9", Email: "n@gmail.com", EmailVerified: true}}
	SetGoogleOAuth(stub)
	t.Cleanup(func() { SetGoogleOAuth(nil) })

	env.doCallback(t, "s1", "s1")
	// Handler harus meneruskan nonce tersimpan ke VerifyIDToken (anti-replay).
	if stub.gotNonce != "the-nonce" {
		t.Errorf("nonce tak diteruskan ke verifikasi: got %q want %q", stub.gotNonce, "the-nonce")
	}
}

func TestLogin_GoogleOnlyAccountRejected(t *testing.T) {
	env, _ := setupTest(t)
	// User Google-only (tanpa pass_hash).
	if _, err := env.q.CreateOAuthUser(t.Context(), db.CreateOAuthUserParams{Email: "googleonly@gmail.com"}); err != nil {
		t.Fatalf("seed google user: %v", err)
	}
	rec := env.do(postForm("/login", url.Values{
		"email": {"googleonly@gmail.com"}, "password": {"apa saja"},
	}), env.h.Login)

	// Kode generik anti-enumeration (sama dgn wrong password) — native PRG 303.
	if loc := rec.Header().Get("Location"); loc != "/login?err=invalid" {
		t.Errorf("login password akun Google-only harus ditolak generik ?err=invalid, got %q", loc)
	}
}
