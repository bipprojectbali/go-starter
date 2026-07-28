package handler

import (
	"context"
	"testing"

	"go_starter/internal/authz"
	"go_starter/internal/session"

	"github.com/alexedwards/scs/v2"
	"github.com/alexedwards/scs/v2/memstore"
)

// ctxWithRole membuat context ber-session dengan role tertentu (untuk authz.Can).
func ctxWithRole(t *testing.T, role string) context.Context {
	t.Helper()
	e, err := authz.New(authz.Model, authz.Policy)
	if err != nil {
		t.Fatalf("authz.New: %v", err)
	}
	authz.Init(e)
	sm := scs.New()
	sm.Store = memstore.New()
	session.Init(sm)
	ctx, err := sm.Load(context.Background(), "")
	if err != nil {
		t.Fatalf("session load: %v", err)
	}
	session.SetIdentity(ctx, 1, "u@x.com", role, false, 1, "Acme", "acme", "")
	return ctx
}

// Sejak 0004 hanya tersisa DUA tujuan lintas-panel: platform (/dev) dan ruang
// kerja aktif. Pintasan "Admin"+"User" yang lama menunjuk dua alamat yang kini
// satu dan sama — menampilkan keduanya berarti dua tombol ke tempat identik.
func TestQuickLinksFor(t *testing.T) {
	cases := []struct {
		role      string
		wantHrefs []string
	}{
		{"member", []string{"/w/acme"}},                    // anggota: ruang kerjanya
		{"admin", []string{"/w/acme"}},                     // admin: sama — beda AKSI, bukan ALAMAT
		{"super_admin", []string{"/dev/users", "/w/acme"}}, // super: platform + ruang kerja
	}
	for _, c := range cases {
		links := quickLinksFor(ctxWithRole(t, c.role))
		if len(links) != len(c.wantHrefs) {
			t.Errorf("role %s: %d pintasan, want %d (%v)", c.role, len(links), len(c.wantHrefs), c.wantHrefs)
			continue
		}
		for i, want := range c.wantHrefs {
			if links[i].Href != want {
				t.Errorf("role %s pintasan[%d].Href=%q, want %q", c.role, i, links[i].Href, want)
			}
		}
	}
}

func TestQuickLinksFor_NeverLinksToBareDevRoute(t *testing.T) {
	// /dev telanjang tak punya handler (404) — pintasan HARUS ke /dev/users.
	links := quickLinksFor(ctxWithRole(t, "super_admin"))
	for _, l := range links {
		if l.Href == "/dev" {
			t.Errorf("pintasan tak boleh menunjuk /dev telanjang (404), harus /dev/users")
		}
	}
}
