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

// TestIsPlatformHome menjaga pembagian 0004: HANYA role platform yang punya home
// tetap (/dev). Role tenant sengaja tidak — alamatnya /w/{slug}, bergantung
// workspace aktif, bukan role. Menambahkan owner/admin ke sini akan menghidupkan
// kembali pemetaan role→path yang justru dihapus.
func TestIsPlatformHome(t *testing.T) {
	cases := []struct {
		role string
		want bool
	}{
		{"super_admin", true}, // platform
		{"staff", true},       // platform
		{"owner", false},      // tenant puncak — tetap ber-workspace
		{"admin", false},      // tenant
		{"member", false},     // tenant anggota
		{"", false},           // default aman: bukan platform
		{"ngawur", false},     // tak dikenal → member → bukan platform
	}
	for _, c := range cases {
		if got := IsPlatformHome(ParseRole(c.role)); got != c.want {
			t.Errorf("IsPlatformHome(%q)=%v, want %v", c.role, got, c.want)
		}
	}
	if PlatformHomePath != "/dev" {
		t.Errorf("PlatformHomePath=%q, want /dev", PlatformHomePath)
	}
}
