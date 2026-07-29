package authz

import (
	"errors"
	"testing"
)

// Guard beroperasi pada tangga TENANT (member < admin < owner). Root env
// (IsRoot) selalu efektif super_admin (> owner). Owner = puncak tenant yang
// dilindungi (dulu super_admin di model lama).
func TestGuardSetRole(t *testing.T) {
	owner := Actor{ID: 1, Role: RoleOwner}
	admin := Actor{ID: 2, Role: RoleAdmin}
	rootEnv := Actor{ID: 3, Role: RoleMember, IsRoot: true} // env override → super_admin

	cases := []struct {
		name    string
		actor   Actor
		target  Target
		newRole Role
		wantErr error
	}{
		{"owner angkat member->admin", owner, Target{ID: 10, Role: RoleMember}, RoleAdmin, nil},
		// Admin TAK boleh mengangkat sesama admin: itu memperbanyak dirinya sendiri
		// dan mencabut kendali atasannya atas siapa yang memegang wewenang itu.
		// Kritis di mode single-app (0006), tempat admin adalah jabatan tertinggi
		// yang bisa diangkat — tanpa aturan ini, satu admin melahirkan semuanya.
		{"admin TAK boleh angkat sesama admin", admin, Target{ID: 10, Role: RoleMember}, RoleAdmin, ErrForbidden},
		{"admin boleh atur member (di bawahnya)", admin, Target{ID: 10, Role: RoleAdmin}, RoleMember, nil},
		{"owner angkat sesama owner ditolak", owner, Target{ID: 10, Role: RoleMember}, RoleOwner, ErrForbidden},
		{"admin TAK boleh angkat owner", admin, Target{ID: 10, Role: RoleMember}, RoleOwner, ErrForbidden},
		{"admin TAK boleh sentuh owner", admin, Target{ID: 10, Role: RoleOwner}, RoleAdmin, ErrForbidden},
		{"target root env kebal", owner, Target{ID: 10, Role: RoleOwner, IsEnvSuperA: true}, RoleMember, ErrProtectedRoot},
		{"root env aktor bisa angkat owner", rootEnv, Target{ID: 10, Role: RoleMember}, RoleOwner, nil},
		{"self-demote ditolak", owner, Target{ID: 1, Role: RoleOwner}, RoleAdmin, ErrSelfLockout},
	}
	for _, c := range cases {
		err := GuardSetRole(c.actor, c.target, c.newRole)
		if !errors.Is(err, c.wantErr) {
			t.Errorf("%s: got %v, want %v", c.name, err, c.wantErr)
		}
	}
}

func TestGuardMutateStatus(t *testing.T) {
	owner := Actor{ID: 1, Role: RoleOwner}
	admin := Actor{ID: 2, Role: RoleAdmin}

	cases := []struct {
		name      string
		actor     Actor
		target    Target
		newStatus string
		wantErr   error
	}{
		{"admin disable member", admin, Target{ID: 10, Role: RoleMember}, "disabled", nil},
		{"admin block member butuh owner", admin, Target{ID: 10, Role: RoleMember}, "blocked", ErrForbidden},
		{"owner block member", owner, Target{ID: 10, Role: RoleMember}, "blocked", nil},
		{"admin TAK sentuh admin lain", admin, Target{ID: 10, Role: RoleAdmin}, "disabled", ErrForbidden},
		{"root env kebal", owner, Target{ID: 10, Role: RoleMember, IsEnvSuperA: true}, "blocked", ErrProtectedRoot},
		{"self ditolak", admin, Target{ID: 2, Role: RoleAdmin}, "disabled", ErrSelfLockout},
	}
	for _, c := range cases {
		err := GuardMutateStatus(c.actor, c.target, c.newStatus)
		if !errors.Is(err, c.wantErr) {
			t.Errorf("%s: got %v, want %v", c.name, err, c.wantErr)
		}
	}
}

func TestGuardDelete(t *testing.T) {
	owner := Actor{ID: 1, Role: RoleOwner}
	admin := Actor{ID: 2, Role: RoleAdmin}

	if err := GuardDelete(admin, Target{ID: 10, Role: RoleMember}); err != nil {
		t.Errorf("admin hapus member: %v", err)
	}
	if err := GuardDelete(admin, Target{ID: 10, Role: RoleAdmin}); !errors.Is(err, ErrForbidden) {
		t.Errorf("admin hapus admin lain harus forbidden: %v", err)
	}
	if err := GuardDelete(owner, Target{ID: 10, IsEnvSuperA: true}); !errors.Is(err, ErrProtectedRoot) {
		t.Errorf("hapus root env harus protected: %v", err)
	}
	if err := GuardDelete(owner, Target{ID: 1, Role: RoleOwner}); !errors.Is(err, ErrSelfLockout) {
		t.Errorf("hapus diri sendiri harus self-lockout: %v", err)
	}
}
