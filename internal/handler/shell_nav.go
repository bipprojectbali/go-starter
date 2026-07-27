package handler

import (
	"context"

	"go_starter/internal/authz"
	"go_starter/internal/db"
	"go_starter/internal/session"
	"go_starter/internal/ui"

	lucide "github.com/eduardolat/gomponents-lucide"
	"maragu.dev/gomponents/html"
)

// shell_nav.go — pemilihan menu & badge untuk halaman LINTAS-PANEL. Dipisah dari
// render.go (yang sudah di ambang file-health) — bukan sekadar demi ukuran:
// isinya memang concern berbeda, yaitu halaman yang tak dimiliki panel manapun.

// navFor memilih menu sidebar untuk halaman lintas-panel (mis. /notifications).
// Semua pemanggil renderShell lain terikat satu panel dan mengoper nav-nya
// langsung; halaman ini tidak, jadi menu ditentukan dari ROLE — pola precompute
// yang sama dengan quickLinksFor (view tak boleh memanggil authz sendiri).
func navFor(ctx context.Context) []ui.NavItem {
	role := session.Role(ctx)
	switch {
	case isPlatformRole(role) || session.IsRoot(ctx):
		return devNav()
	case role == authz.RoleNameOwner || role == authz.RoleNameAdmin:
		return adminNav
	default:
		return userNav
	}
}

// brandFor = sub-label panel, dipasangkan dengan navFor agar konteks di sidebar
// cocok dengan menu yang ditampilkan.
func brandFor(ctx context.Context) string {
	role := session.Role(ctx)
	switch {
	case isPlatformRole(role) || session.IsRoot(ctx):
		return "go_starter /dev"
	case role == authz.RoleNameOwner || role == authz.RoleNameAdmin:
		return "go_starter /admin"
	default:
		return "go_starter"
	}
}

// notifBadge menghitung yang perlu perhatian: peristiwa belum dibaca + undangan
// pending. Undangan sengaja ikut walau tak pernah ter-auto-read — ia tugas yang
// hanya hilang bila benar-benar ditindak.
//
// FAIL-SOFT 3 lapis (pola workspaceOptions): tanpa uid → nil; gagal query → log
// + nil. Badge hilang jauh lebih baik daripada halaman gagal render.
func (h *Handler) notifBadge(ctx context.Context) *ui.NavBadge {
	uid := session.UserID(ctx)
	if uid == 0 {
		return nil
	}
	item := ui.NavItem{
		Label: "Notifikasi", Href: "/notifications",
		Icon: lucide.Bell(html.Class("size-4")),
	}
	var total int64
	if err := db.WithSuper(ctx, h.Pool, func(q *db.Queries) error {
		n, err := q.CountUnreadNotifications(ctx, uid)
		if err != nil {
			return err
		}
		inv, err := q.CountPendingInvitesByEmail(ctx, normalizeEmail(session.Email(ctx)))
		if err != nil {
			return err
		}
		total = n + inv
		return nil
	}); err != nil {
		h.Log.Error("shell: notif badge", "err", err)
		return &ui.NavBadge{Item: item} // menu tetap ada, hanya angkanya absen
	}
	return &ui.NavBadge{Item: item, Count: total}
}
