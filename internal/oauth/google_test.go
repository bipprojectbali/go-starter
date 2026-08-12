package oauth

import (
	"net/url"
	"regexp"
	"strings"
	"testing"

	"golang.org/x/oauth2"
)

// newAuthURLProvider membangun Provider tanpa jaringan (tak lewat New/discovery
// OIDC): AuthURL hanya menyentuh field oauth, jadi verifier boleh nil di sini.
// (newTestProvider di verify_test.go menyetel verifier tapi bukan oauth — beda tujuan.)
func newAuthURLProvider() *Provider {
	return &Provider{
		oauth: &oauth2.Config{
			ClientID:    "test-client",
			RedirectURL: "https://app.example.com/cb",
			Endpoint:    oauth2.Endpoint{AuthURL: "https://accounts.google.com/o/oauth2/v2/auth"},
			Scopes:      []string{"openid", "email", "profile"},
		},
	}
}

// promptParam mengekstrak nilai query "prompt" dari URL consent yang dihasilkan.
func promptParam(t *testing.T, raw string) string {
	t.Helper()
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("parse AuthURL: %v", err)
	}
	return u.Query().Get("prompt")
}

func TestAuthURL_SelectAccount(t *testing.T) {
	p := newAuthURLProvider()

	// Login normal: TANPA prompt=select_account (flow mulus, tak ada klik ekstra).
	if got := promptParam(t, p.AuthURL("s", "n", "v", false)); got != "" {
		t.Errorf("login normal tak boleh set prompt, got %q", got)
	}

	// Ganti akun: WAJIB prompt=select_account (Google tampilkan pemilih akun).
	got := p.AuthURL("s", "n", "v", true)
	if p := promptParam(t, got); p != "select_account" {
		t.Errorf("ganti akun harus prompt=select_account, got %q", p)
	}
	// State & PKCE challenge tetap ada di kedua mode (proteksi tak boleh hilang).
	if !strings.Contains(got, "state=s") || !strings.Contains(got, "code_challenge=") {
		t.Errorf("AuthURL kehilangan state/PKCE: %s", got)
	}
}

func TestNormalizeAvatarURL(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want *string // nil = expect nil
	}{
		{"kosong", "", nil},
		{"spasi saja", "   ", nil},
		{"strip =s96-c", "https://lh3.googleusercontent.com/a/abc=s96-c", ptr("https://lh3.googleusercontent.com/a/abc")},
		{"strip =s256 tanpa -c", "https://lh3.googleusercontent.com/a/x=s256", ptr("https://lh3.googleusercontent.com/a/x")},
		{"tanpa suffix ukuran", "https://lh3.googleusercontent.com/a/plain", ptr("https://lh3.googleusercontent.com/a/plain")},
		{"suffix di tengah tak di-strip (anchored)", "https://x/=s96-c/y", ptr("https://x/=s96-c/y")},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := NormalizeAvatarURL(c.in)
			switch {
			case c.want == nil && got != nil:
				t.Errorf("want nil, got %q", *got)
			case c.want != nil && got == nil:
				t.Errorf("want %q, got nil", *c.want)
			case c.want != nil && got != nil && *got != *c.want:
				t.Errorf("got %q, want %q", *got, *c.want)
			}
		})
	}
}

func ptr(s string) *string { return &s }

func TestNewState(t *testing.T) {
	s, err := NewState()
	if err != nil {
		t.Fatalf("NewState: %v", err)
	}
	// 32 byte → 64 char hex.
	if len(s) != 64 {
		t.Errorf("state harus 64 char hex, got %d", len(s))
	}
	if !regexp.MustCompile(`^[0-9a-f]{64}$`).MatchString(s) {
		t.Errorf("state harus hex lowercase: %q", s)
	}
	// Entropi: dua panggilan berbeda.
	s2, _ := NewState()
	if s == s2 {
		t.Error("dua NewState harus berbeda (entropi)")
	}
}

func TestNewVerifier(t *testing.T) {
	v := NewVerifier()
	if v == "" {
		t.Fatal("verifier tak boleh kosong")
	}
	// PKCE spec: 43–128 char.
	if len(v) < 43 || len(v) > 128 {
		t.Errorf("verifier di luar rentang PKCE (43-128): %d", len(v))
	}
	if v == NewVerifier() {
		t.Error("dua verifier harus berbeda")
	}
}
