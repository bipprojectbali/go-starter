package handler

import (
	"testing"

	"go_stater/internal/db"
)

// TestPromoteSuperAdmins menjaga aturan reconcile boot: email env dinaikkan ke
// super_admin (promote), tapi super-admin DB tier-2 (tak di env) & user lain
// TIDAK berubah (promote-only, tak pernah demote).
func TestPromoteSuperAdmins(t *testing.T) {
	env, _ := setupTest(t)
	q := env.h.DB

	// Seed: env-user (role user), tier-2 (super_admin di DB), user biasa.
	envUser, _ := q.CreateUser(t.Context(), db.CreateUserParams{Email: "env@local", PassHash: ptr("x")})
	tier2, _ := q.CreateUser(t.Context(), db.CreateUserParams{Email: "tier2@local", PassHash: ptr("x")})
	if err := q.UpdateUserRole(t.Context(), db.UpdateUserRoleParams{ID: tier2.ID, Role: "super_admin"}); err != nil {
		t.Fatalf("seed tier2: %v", err)
	}
	biasa, _ := q.CreateUser(t.Context(), db.CreateUserParams{Email: "biasa@local", PassHash: ptr("x")})

	// Reconcile: hanya env@local di daftar.
	if err := q.PromoteSuperAdmins(t.Context(), []string{"env@local"}); err != nil {
		t.Fatalf("PromoteSuperAdmins: %v", err)
	}

	// env-user dipromosikan.
	if u, _ := q.GetUser(t.Context(), envUser.ID); u.Role != "super_admin" {
		t.Errorf("email env harus dipromosikan ke super_admin, got %q", u.Role)
	}
	// tier-2 tetap (tak diturunkan).
	if u, _ := q.GetUser(t.Context(), tier2.ID); u.Role != "super_admin" {
		t.Errorf("super-admin DB tier-2 harus tetap, got %q", u.Role)
	}
	// user biasa tak tersentuh.
	if u, _ := q.GetUser(t.Context(), biasa.ID); u.Role != "user" {
		t.Errorf("user biasa tak boleh berubah, got %q", u.Role)
	}
}
