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
		BrandLabel:  "go_stater /dev",
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
	for _, want := range []string{"konten", "go_stater /dev", "sidebarOpen", "Keluar"} {
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

func TestAppShell_InactiveWhenPathDiffers(t *testing.T) {
	out := renderShell("/dev/other")
	if strings.Contains(out, `aria-current="page"`) {
		t.Errorf("menu tak boleh aktif saat path beda:\n%s", out)
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
