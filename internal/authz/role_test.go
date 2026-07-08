package authz

import "testing"

func TestParseRoleAndString(t *testing.T) {
	cases := []struct {
		in   string
		want Role
	}{
		{"user", RoleUser},
		{"admin", RoleAdmin},
		{"super_admin", RoleSuperAdmin},
		{"", RoleUser},        // tak dikenal → aman ke user
		{"garbage", RoleUser}, // idem
	}
	for _, c := range cases {
		if got := ParseRole(c.in); got != c.want {
			t.Errorf("ParseRole(%q)=%v, want %v", c.in, got, c.want)
		}
	}
	// Round-trip nama role.
	for _, r := range []Role{RoleUser, RoleAdmin, RoleSuperAdmin} {
		if ParseRole(r.String()) != r {
			t.Errorf("round-trip gagal untuk %v (%q)", r, r.String())
		}
	}
}

func TestHomePath(t *testing.T) {
	cases := []struct {
		role string
		want string
	}{
		{"super_admin", "/dev"},
		{"admin", "/admin"},
		{"user", "/user"},
		{"", "/user"}, // default aman
	}
	for _, c := range cases {
		if got := HomePathFor(c.role); got != c.want {
			t.Errorf("HomePathFor(%q)=%q, want %q", c.role, got, c.want)
		}
	}
}
