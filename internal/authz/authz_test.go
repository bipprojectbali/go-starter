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
		// Hierarki: admin mewarisi user.
		{"user", "app:todos", "read", true, "user boleh todos"},
		{"admin", "app:todos", "read", true, "admin warisi user"},
		{"super_admin", "app:todos", "read", true, "super_admin warisi semua"},

		// admin panel.
		{"admin", "admin:users", "read", true, "admin baca users"},
		{"admin", "admin:users:btn-delete", "delete", true, "admin hapus (glob keyMatch)"},
		{"user", "admin:users", "read", false, "user tak boleh panel admin"},

		// /dev = super_admin/root saja (via bypass root), user & admin ditolak.
		{"user", "dev:users", "read", false, "user tak boleh /dev"},
		{"admin", "dev:users", "read", false, "admin tak boleh /dev"},
		{"super_admin", "dev:users", "read", true, "super_admin god-mode via root"},
		{"super_admin", "dev:apa-pun:xyz", "manage", true, "super_admin bypass total"},

		// Default deny: role tak dikenal / obj tak ada policy.
		{"user", "dev:secret", "manage", false, "deny default"},
		{"stranger", "app:todos", "read", false, "role tak dikenal ditolak"},
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
