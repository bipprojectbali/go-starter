package authz

import "testing"

func TestParseRoleAndString(t *testing.T) {
	cases := []struct {
		in   string
		want Role
	}{
		{"member", RoleMember},
		{"admin", RoleAdmin},
		{"owner", RoleOwner},
		{"staff", RoleStaff},
		{"super_admin", RoleSuperAdmin},
		{"", RoleMember},        // tak dikenal → aman ke member (terendah)
		{"garbage", RoleMember}, // idem
	}
	for _, c := range cases {
		if got := ParseRole(c.in); got != c.want {
			t.Errorf("ParseRole(%q)=%v, want %v", c.in, got, c.want)
		}
	}
	// Round-trip nama role (semua tingkat 2-bidang).
	for _, r := range []Role{RoleMember, RoleAdmin, RoleOwner, RoleStaff, RoleSuperAdmin} {
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
		{"super_admin", "/dev"}, // platform
		{"staff", "/dev"},       // platform
		{"owner", "/admin"},     // tenant puncak
		{"admin", "/admin"},     // tenant
		{"member", "/user"},     // tenant anggota
		{"", "/user"},           // default aman
	}
	for _, c := range cases {
		if got := HomePathFor(c.role); got != c.want {
			t.Errorf("HomePathFor(%q)=%q, want %q", c.role, got, c.want)
		}
	}
}
