package handler

import (
	"strings"
	"testing"
	"time"

	"go_starter/internal/session"
	"go_starter/internal/ui"
)

// shell_nav_test.go — /notifications tak dimiliki panel manapun, jadi menunya
// dipilih dari role. Kalau salah pilih, user melihat menu yang tak boleh ia
// akses (atau kehilangan menu panelnya saat membuka notifikasi).

func TestNavFor_MenuIkutRole(t *testing.T) {
	cases := []struct {
		role      string
		wantFirst string // href item pertama = penanda panel yang dipilih
	}{
		{"member", "/user"},
		{"admin", "/admin"},
		{"owner", "/admin"},
		{"staff", "/dev/users"},
		{"super_admin", "/dev/users"},
	}
	for _, c := range cases {
		nav := navFor(ctxWithRole(t, c.role))
		if len(nav) == 0 {
			t.Errorf("role %s: menu kosong — halaman notifikasi akan tanpa navigasi", c.role)
			continue
		}
		if nav[0].Href != c.wantFirst {
			t.Errorf("role %s: menu mulai dari %q, want %q", c.role, nav[0].Href, c.wantFirst)
		}
	}
}

// TestNavFor_MemberTakDapatMenuAdmin: member membuka /notifications tak boleh
// mendapati menu panel yang route-nya akan menolaknya (menu hantu).
func TestNavFor_MemberTakDapatMenuAdmin(t *testing.T) {
	for _, it := range navFor(ctxWithRole(t, "member")) {
		if strings.HasPrefix(it.Href, "/admin") || strings.HasPrefix(it.Href, "/dev") {
			t.Errorf("member tak boleh melihat menu %q di halaman notifikasi", it.Href)
		}
	}
}

// TestBrandFor_SelarasDenganNav: sub-label sidebar harus cocok dengan menu yang
// tampil — kalau tidak, user melihat menu /admin berlabel "/dev".
func TestBrandFor_SelarasDenganNav(t *testing.T) {
	cases := map[string]string{
		"member":      "go_starter",
		"admin":       "go_starter /admin",
		"super_admin": "go_starter /dev",
	}
	for role, want := range cases {
		if got := brandFor(ctxWithRole(t, role)); got != want {
			t.Errorf("role %s: brand %q, want %q", role, got, want)
		}
	}
}

// TestNotifBadge_TanpaUserNil: fail-soft lapis pertama — shell dirender juga di
// jalur tanpa user; badge tak boleh memaksa query atau panic.
func TestNotifBadge_TanpaUserNil(t *testing.T) {
	env, _ := setupTest(t)
	var got *ui.NavBadge
	env.withSession(t, 0, func(sc sessionCtx) {
		got = env.h.notifBadge(sc.ctx)
	})
	if got != nil {
		t.Errorf("tanpa user login badge harus nil, got %+v", got)
	}
}

// TestNotifBadge_JumlahGabungan: badge = peristiwa belum dibaca + undangan
// pending. Undangan ikut karena ia tugas yang belum ditindak.
func TestNotifBadge_JumlahGabungan(t *testing.T) {
	env, uid := setupTest(t)
	env.mkNotif(t, uid, "member.role.changed")
	env.mkNotif(t, uid, "member.removed")
	env.mkInvite(t, "tok-badge", "test@local", "member", time.Hour)

	var got *ui.NavBadge
	env.withSession(t, uid, func(sc sessionCtx) {
		session.SetIdentity(sc.ctx, uid, "test@local", "owner", false, env.tenantID, "Test", "")
		got = env.h.notifBadge(sc.ctx)
	})
	if got == nil {
		t.Fatal("badge harus ada untuk user login")
	}
	if got.Count != 3 {
		t.Errorf("badge = 2 peristiwa + 1 undangan = 3, got %d", got.Count)
	}
	if got.Item.Href != "/notifications" {
		t.Errorf("href badge salah: %q", got.Item.Href)
	}
}
