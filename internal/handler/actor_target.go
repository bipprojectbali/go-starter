package handler

import (
	"context"
	"net/http"
	"strconv"

	"go_starter/internal/authz"
	"go_starter/internal/db"
	"go_starter/internal/session"

	"github.com/go-chi/chi/v5"
)

// actor_target.go — menyusun pasangan (aktor, target) yang dibaca guard authz.
// Dipakai DUA panel: /dev/users (lintas-workspace) & /w/{slug}/members (satu
// workspace). Keduanya wajib memakai penyusun yang sama — kalau salah satunya
// menilai role target dengan caranya sendiri, aturan wewenang akan berbeda
// tergantung pintu yang dipakai, dan itu persis bentuk celah yang sulit terlihat.

func (h *Handler) parseTargetID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, "id tidak valid", http.StatusBadRequest)
		return 0, false
	}
	return id, true
}

// loadActorTarget membangun Actor (dari session) & Target (dari DB) untuk guard.
// tenantID = workspace tempat role target dinilai (role kini per-workspace).
func (h *Handler) loadActorTarget(ctx context.Context, targetID, tenantID int64) (authz.Actor, authz.Target, error) {
	actor := authz.Actor{
		ID:     session.UserID(ctx),
		Role:   authz.ParseRole(session.Role(ctx)),
		IsRoot: session.IsRoot(ctx),
	}
	tu, err := h.q(ctx).GetUser(ctx, targetID)
	if err != nil {
		return actor, authz.Target{}, err
	}
	// Role target = role di WORKSPACE tsb (kini per-workspace, bukan properti user).
	// Tak punya membership di sana → RoleMember.
	//
	// Perhatikan arahnya: guard membandingkan `target.Role >= actorRole`, jadi role
	// terendah membuat target LEBIH mudah disentuh, bukan lebih terlindungi. Itu
	// benar di sini justru karena orang tanpa keanggotaan tak punya wewenang apa
	// pun di workspace itu — tak ada yang perlu dilindungi dari penurunan. Yang
	// MENAHAN aksi liar tetap gerbang di depan (route dev:users) & IsEnvSuperA.
	role := authz.RoleMember
	if m, e := h.q(ctx).GetMembership(ctx, db.GetMembershipParams{UserID: targetID, TenantID: tenantID}); e == nil {
		role = authz.ParseRole(m.Role)
	}
	target := authz.Target{
		ID:          tu.ID,
		Role:        role,
		IsEnvSuperA: isSuperAdminEmail(tu.Email),
	}
	return actor, target, nil
}
