package ui

import (
	"strings"
	"testing"

	lucide "github.com/eduardolat/gomponents-lucide"
	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

func renderShell(currentPath string) string {
	d := ShellData{
		Title:       "Dev",
		BrandLabel:  "go_starter /dev",
		CurrentPath: currentPath,
		UserEmail:   "owner@x.com",
		Nav: []NavItem{
			{Label: "Users", Href: "/dev/users", Icon: lucide.Users(h.Class("size-4"))},
		},
	}
	var sb strings.Builder
	AppShell(d, g.Text("konten")).Render(&sb)
	return sb.String()
}

func TestAppShell_ActiveLink(t *testing.T) {
	out := renderShell("/dev/users")
	// Menu Users harus aktif (aria-current) saat path cocok.
	if !strings.Contains(out, `aria-current="page"`) {
		t.Errorf("menu aktif harus punya aria-current:\n%s", out)
	}
	if !strings.Contains(out, `href="/dev/users"`) {
		t.Errorf("menu Users harus ada:\n%s", out)
	}
	// Konten & shell dasar.
	for _, want := range []string{"konten", "go_starter /dev", "sidebarOpen", "Keluar"} {
		if !strings.Contains(out, want) {
			t.Errorf("shell kurang %q", want)
		}
	}
	// REGRESI: data-class key ber-hyphen HARUS ter-quote (kalau tidak, ekspresi
	// JS invalid → Datastar mati: "Unexpected token '-'").
	if !strings.Contains(out, `{&#39;translate-x-0&#39;: $sidebarOpen}`) {
		t.Errorf("data-class key harus ter-quote (hyphen tak valid tanpa kutip):\n%s", out)
	}
}

// TestAppShell_SwitchAccount: footer sidebar menawarkan "Ganti akun" (pindah akun
// tanpa logout dulu) berdampingan dengan "Keluar". Link GET ke /account/switch
// (dijaga RequireAuth — /api/auth/google?switch=1 dijaga RequireGuest, memantul
// balik untuk user yang sudah masuk).
func TestAppShell_SwitchAccount(t *testing.T) {
	out := renderShell("/dev/users")
	for _, want := range []string{"Ganti akun", `href="/account/switch"`} {
		if !strings.Contains(out, want) {
			t.Errorf("footer sidebar kurang %q:\n%s", want, out)
		}
	}
}

// TestUserMenu_Consolidated: "Ganti akun" + "Keluar" hidup dalam SATU dropdown
// (details/summary) yang dipicu avatar user — bukan lagi dua tombol terpisah.
// Keluar tetap lewat modal konfirmasi (signal logoutConfirm), bukan POST langsung.
func TestUserMenu_Consolidated(t *testing.T) {
	out := renderShell("/dev/users")
	for _, want := range []string{
		"<details", "<summary", // dropdown CSS-only (CSP-safe)
		`href="/account/switch"`, "Ganti akun",
		"Keluar", "$logoutConfirm = true", // Keluar → buka modal, bukan logout langsung
	} {
		if !strings.Contains(out, want) {
			t.Errorf("menu akun kurang %q:\n%s", want, out)
		}
	}
}

// TestSidebar_ThemeBlockSeparate: pemilih tema hadir di sidebar sebagai baris
// menu tersendiri (dropdown-top, theme-controller) — dipisah dari footer identitas
// agar email panjang tak menumpuk ikon tema. Regresi keluhan "numpuk dengan tema".
func TestSidebar_ThemeBlockSeparate(t *testing.T) {
	out := renderShell("/dev/users")
	for _, want := range []string{"data-theme-dropdown", "theme-controller", "dropdown-top"} {
		if !strings.Contains(out, want) {
			t.Errorf("blok tema sidebar kurang %q:\n%s", want, out)
		}
	}
}

func TestAppShell_InactiveWhenPathDiffers(t *testing.T) {
	out := renderShell("/dev/other")
	if strings.Contains(out, `aria-current="page"`) {
		t.Errorf("menu tak boleh aktif saat path beda:\n%s", out)
	}
}

// TestActiveNavHref: longest-match menang — sub-route TIDAK mengaktifkan parent
// (regresi bug "/admin/workspace" bikin Dashboard + Workspace dua-duanya menyala).
func TestActiveNavHref(t *testing.T) {
	nav := []NavItem{
		{Label: "Dashboard", Href: "/admin"},
		{Label: "Workspace", Href: "/admin/workspace"},
	}
	cases := []struct{ path, want string }{
		{"/admin", "/admin"},                       // exact → Dashboard
		{"/admin/workspace", "/admin/workspace"},   // sub-route → Workspace SAJA (bukan /admin)
		{"/admin/workspace/x", "/admin/workspace"}, // lebih dalam → tetap Workspace
		{"/adminX", ""},                            // batas segmen: /adminX BUKAN di bawah /admin
		{"/other", ""},                             // tak cocok
	}
	for _, c := range cases {
		if got := activeNavHref(nav, c.path); got != c.want {
			t.Errorf("activeNavHref(%q)=%q, want %q", c.path, got, c.want)
		}
	}
}

// TestActiveNavHref_Root: item "/" hanya aktif pada path persis "/" (tak cocok semua).
func TestActiveNavHref_Root(t *testing.T) {
	nav := []NavItem{{Label: "Home", Href: "/"}}
	if got := activeNavHref(nav, "/"); got != "/" {
		t.Errorf("root exact harus aktif, got %q", got)
	}
	if got := activeNavHref(nav, "/dev"); got != "" {
		t.Errorf("root TAK boleh aktif di /dev, got %q", got)
	}
}

func TestWhen(t *testing.T) {
	var yes, no strings.Builder
	When(true, g.Text("TAMPIL")).Render(&yes)
	When(false, g.Text("TAMPIL")).Render(&no)
	if !strings.Contains(yes.String(), "TAMPIL") {
		t.Error("When(true) harus render node")
	}
	if strings.Contains(no.String(), "TAMPIL") {
		t.Error("When(false) tak boleh render node")
	}
}
