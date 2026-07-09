package pages

import (
	"strings"
	"testing"

	g "maragu.dev/gomponents"
)

func render(t *testing.T, node g.Node) string {
	t.Helper()
	var sb strings.Builder
	if err := node.Render(&sb); err != nil {
		t.Fatalf("render: %v", err)
	}
	return sb.String()
}

func TestLogin_GoogleAlways_NoPasswordInProd(t *testing.T) {
	out := render(t, Login(false)) // production: tanpa form password
	if !strings.Contains(out, "/api/auth/google") || !strings.Contains(out, "Masuk dengan Google") {
		t.Errorf("tombol Google harus selalu ada:\n%s", out)
	}
	if strings.Contains(out, `type="password"`) {
		t.Errorf("form password tak boleh muncul saat showPassword=false:\n%s", out)
	}
}

func TestLogin_WithPassword_Dev(t *testing.T) {
	out := render(t, Login(true))
	for _, want := range []string{"Masuk dengan Google", `type="password"`, "@post(&#39;/login&#39;)", "/register"} {
		if !strings.Contains(out, want) {
			t.Errorf("login dev kurang %q:\n%s", want, out)
		}
	}
}

func TestRegister(t *testing.T) {
	out := render(t, Register())
	for _, want := range []string{"@post(&#39;/register&#39;)", `type="password"`, "/login"} {
		if !strings.Contains(out, want) {
			t.Errorf("register kurang %q:\n%s", want, out)
		}
	}
}

func TestLanding(t *testing.T) {
	// Login → CTA ke home per-role (homePath yang dioper), teks "Buka aplikasi".
	in := render(t, Landing(true, "/dev"))
	if !strings.Contains(in, `href="/dev"`) || !strings.Contains(in, "Buka aplikasi") {
		t.Errorf("landing login harus CTA ke homePath:\n%s", in)
	}
	// Anonim → CTA /login, homePath diabaikan.
	anon := render(t, Landing(false, "/dev"))
	if !strings.Contains(anon, `href="/login"`) {
		t.Errorf("landing anonim harus CTA /login:\n%s", anon)
	}
	if strings.Contains(anon, `href="/dev"`) {
		t.Errorf("landing anonim tak boleh link ke home:\n%s", anon)
	}
}
