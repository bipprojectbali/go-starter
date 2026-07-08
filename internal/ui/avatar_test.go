package ui

import (
	"strings"
	"testing"

	h "maragu.dev/gomponents/html"
)

func TestInitials(t *testing.T) {
	cases := []struct{ name, email, want string }{
		{"Budi Santoso", "b@x.com", "BS"},
		{"Budi", "b@x.com", "B"},
		{"", "kurosaki@gmail.com", "K"}, // fallback ke email
		{"", "", "?"},                   // kosong total
		{"agus.dwi.p", "a@x.com", "AP"}, // titik → pemisah
	}
	for _, c := range cases {
		if got := initials(c.name, c.email); got != c.want {
			t.Errorf("initials(%q,%q)=%q, want %q", c.name, c.email, got, c.want)
		}
	}
}

func TestDeterministicColor(t *testing.T) {
	// Deterministik: input sama → warna sama.
	a := deterministicColor("kurosaki@gmail.com")
	b := deterministicColor("kurosaki@gmail.com")
	if a != b {
		t.Errorf("warna harus stabil untuk email sama: %q vs %q", a, b)
	}
	if !strings.HasPrefix(a, "#") {
		t.Errorf("warna harus hex, got %q", a)
	}
}

func TestSizedAvatar(t *testing.T) {
	if got := sizedAvatar("", 40); got != "" {
		t.Errorf("URL kosong harus tetap kosong, got %q", got)
	}
	got := sizedAvatar("https://lh3.googleusercontent.com/a/abc", 40)
	if !strings.Contains(got, "=s80-c") { // 40*2 retina
		t.Errorf("harus tambah suffix ukuran retina, got %q", got)
	}
}

func TestAvatar_RendersImgWithSafeguards(t *testing.T) {
	var sb strings.Builder
	Avatar("https://lh3.googleusercontent.com/a/x", "Budi", "b@x.com", 32).Render(&sb)
	out := sb.String()
	// Wajib: inisial server-side + img dengan referrerpolicy + onerror fallback.
	for _, want := range []string{"avatar-initials", "referrerpolicy", "no-referrer", "this.remove()", "=s64-c"} {
		if !strings.Contains(out, want) {
			t.Errorf("avatar HTML kurang %q:\n%s", want, out)
		}
	}
}

func TestAvatar_NoURLShowsInitialsOnly(t *testing.T) {
	var sb strings.Builder
	Avatar("", "Budi Santoso", "b@x.com", 32).Render(&sb)
	out := sb.String()
	if strings.Contains(out, "<img") {
		t.Errorf("tanpa URL tak boleh render <img>:\n%s", out)
	}
	if !strings.Contains(out, "BS") {
		t.Errorf("harus tampil inisial BS:\n%s", out)
	}
	_ = h.Div // keep import stable
}
