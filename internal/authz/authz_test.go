package authz

import "testing"

// buildEnforcer memuat model + policy embed asli — menguji policy nyata, bukan fixture.
func buildEnforcer(t *testing.T) *enforcerWrap {
	t.Helper()
	e, err := New(Model, Policy)
	if err != nil {
		t.Fatalf("New enforcer: %v", err)
	}
	return &enforcerWrap{e}
}

type enforcerWrap struct {
	e interface {
		Enforce(...interface{}) (bool, error)
	}
}

func (w *enforcerWrap) can(t *testing.T, role, obj, act string) bool {
	t.Helper()
	ok, err := w.e.Enforce(role, obj, act)
	if err != nil {
		t.Fatalf("Enforce(%s,%s,%s): %v", role, obj, act, err)
	}
	return ok
}

func TestPolicy_Enforcement(t *testing.T) {
	e := buildEnforcer(t)

	cases := []struct {
		role, obj, act string
		want           bool
		why            string
	}{
		// Hierarki TENANT: admin mewarisi member (user:home), owner mewarisi admin.
		{"member", "user:home", "read", true, "member boleh beranda"},
		{"admin", "user:home", "read", true, "admin warisi user:home"},
		{"owner", "user:home", "read", true, "owner warisi member"},
		{"super_admin", "user:home", "read", true, "super_admin warisi semua"},

		// Notifikasi = milik SETIAP user login. Undangan bisa datang dari workspace
		// mana pun, jadi tak boleh terbatas satu tingkat role — semua tingkat wajib
		// lolos lewat rantai g dari member.
		{"member", "notif:home", "read", true, "member boleh notifikasi"},
		{"admin", "notif:home", "read", true, "admin warisi notifikasi"},
		{"owner", "notif:home", "read", true, "owner warisi notifikasi"},
		{"staff", "notif:home", "read", true, "staff warisi notifikasi (via owner)"},
		{"super_admin", "notif:home", "read", true, "super_admin lolos notifikasi"},

		// admin panel (tenant).
		{"admin", "admin:users", "read", true, "admin baca users"},
		{"admin", "admin:users:btn-delete", "delete", true, "admin hapus (glob keyMatch)"},
		{"member", "admin:users", "read", false, "member tak boleh panel admin"},

		// /dev = PLATFORM (staff + super_admin). Tenant role ditolak.
		{"member", "dev:users", "read", false, "member tak boleh /dev"},
		{"admin", "dev:users", "read", false, "admin tak boleh /dev"},
		{"owner", "dev:users", "read", false, "owner (tenant) tak boleh /dev"},
		{"staff", "dev:users", "read", true, "staff baca panel /dev"},
		{"super_admin", "dev:users", "read", true, "super_admin god-mode via root"},
		{"super_admin", "dev:apa-pun:xyz", "manage", true, "super_admin bypass total"},

		// staff TERBATAS: bukan god-mode (bukan root). Obj platform sensitif ditolak.
		{"staff", "platform:tenants:delete", "delete", false, "staff TAK boleh hapus tenant"},
		{"staff", "platform:staff:manage", "manage", false, "staff TAK boleh kelola staff"},

		// Default deny: role tak dikenal / obj tak ada policy.
		{"member", "dev:secret", "manage", false, "deny default"},
		{"stranger", "user:home", "read", false, "role tak dikenal ditolak"},
	}
	for _, c := range cases {
		if got := e.can(t, c.role, c.obj, c.act); got != c.want {
			t.Errorf("%s: Enforce(%s,%s,%s)=%v, want %v",
				c.why, c.role, c.obj, c.act, got, c.want)
		}
	}
}

func TestNew_InvalidPolicyLine(t *testing.T) {
	_, err := New(Model, "x, bad, line")
	if err == nil {
		t.Error("baris policy tak dikenal harus error")
	}
}

// TestPlatformSettings_StaffDitolak menjaga pemisahan yang mudah hilang: SELURUH
// panel /dev di-gate dev:users, dan staff memilikinya. Tanpa objek tersendiri
// untuk platform:settings, staff — yang aksesnya sengaja dibatasi (tak boleh
// suspend tenant, tak boleh kelola staff lain) — bisa mengubah aturan yang
// berlaku bagi SETIAP user di platform.
func TestPlatformSettings_StaffDitolak(t *testing.T) {
	e, err := New(Model, Policy)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if ok, _ := e.Enforce("staff", "platform:settings", "write"); ok {
		t.Error("staff TAK BOLEH mengubah pengaturan platform")
	}
	if ok, _ := e.Enforce("super_admin", "platform:settings", "write"); !ok {
		t.Error("super_admin harus boleh mengubah pengaturan platform")
	}
	// Staff tetap boleh melihat panel /dev — yang dibatasi hanya pengaturannya.
	if ok, _ := e.Enforce("staff", "dev:users", "read"); !ok {
		t.Error("staff harus tetap bisa membuka panel /dev")
	}
	for _, role := range []string{"owner", "admin", "member"} {
		if ok, _ := e.Enforce(role, "platform:settings", "write"); ok {
			t.Errorf("role tenant %q tak boleh menyentuh pengaturan platform", role)
		}
	}
}
