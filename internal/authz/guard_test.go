package authz

import (
	"errors"
	"testing"
)

func TestGuardSetRole(t *testing.T) {
	superA := Actor{ID: 1, Role: RoleSuperAdmin}
	admin := Actor{ID: 2, Role: RoleAdmin}
	rootEnv := Actor{ID: 3, Role: RoleUser, IsRoot: true} // env override

	cases := []struct {
		name    string
		actor   Actor
		target  Target
		newRole Role
		wantErr error
	}{
		{"super angkat user->admin", superA, Target{ID: 10, Role: RoleUser}, RoleAdmin, nil},
		{"super angkat user->super", superA, Target{ID: 10, Role: RoleUser}, RoleSuperAdmin, nil},
		{"admin angkat user->admin", admin, Target{ID: 10, Role: RoleUser}, RoleAdmin, nil},
		{"admin TAK boleh angkat super", admin, Target{ID: 10, Role: RoleUser}, RoleSuperAdmin, ErrForbidden},
		{"admin TAK boleh sentuh super", admin, Target{ID: 10, Role: RoleSuperAdmin}, RoleAdmin, ErrForbidden},
		{"target root env kebal", superA, Target{ID: 10, Role: RoleSuperAdmin, IsEnvSuperA: true}, RoleUser, ErrProtectedRoot},
		{"root env aktor bisa angkat super", rootEnv, Target{ID: 10, Role: RoleUser}, RoleSuperAdmin, nil},
		{"self-demote ditolak", superA, Target{ID: 1, Role: RoleSuperAdmin}, RoleAdmin, ErrSelfLockout},
	}
	for _, c := range cases {
		err := GuardSetRole(c.actor, c.target, c.newRole)
		if !errors.Is(err, c.wantErr) {
			t.Errorf("%s: got %v, want %v", c.name, err, c.wantErr)
		}
	}
}

func TestGuardMutateStatus(t *testing.T) {
	superA := Actor{ID: 1, Role: RoleSuperAdmin}
	admin := Actor{ID: 2, Role: RoleAdmin}

	cases := []struct {
		name      string
		actor     Actor
		target    Target
		newStatus string
		wantErr   error
	}{
		{"admin disable user", admin, Target{ID: 10, Role: RoleUser}, "disabled", nil},
		{"admin block user butuh super", admin, Target{ID: 10, Role: RoleUser}, "blocked", ErrForbidden},
		{"super block user", superA, Target{ID: 10, Role: RoleUser}, "blocked", nil},
		{"admin TAK sentuh admin lain", admin, Target{ID: 10, Role: RoleAdmin}, "disabled", ErrForbidden},
		{"root env kebal", superA, Target{ID: 10, Role: RoleUser, IsEnvSuperA: true}, "blocked", ErrProtectedRoot},
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
	superA := Actor{ID: 1, Role: RoleSuperAdmin}
	admin := Actor{ID: 2, Role: RoleAdmin}

	if err := GuardDelete(admin, Target{ID: 10, Role: RoleUser}); err != nil {
		t.Errorf("admin hapus user: %v", err)
	}
	if err := GuardDelete(admin, Target{ID: 10, Role: RoleAdmin}); !errors.Is(err, ErrForbidden) {
		t.Errorf("admin hapus admin lain harus forbidden: %v", err)
	}
	if err := GuardDelete(superA, Target{ID: 10, IsEnvSuperA: true}); !errors.Is(err, ErrProtectedRoot) {
		t.Errorf("hapus root env harus protected: %v", err)
	}
	if err := GuardDelete(superA, Target{ID: 1, Role: RoleSuperAdmin}); !errors.Is(err, ErrSelfLockout) {
		t.Errorf("hapus diri sendiri harus self-lockout: %v", err)
	}
}
