package handler

import (
	"context"
	"net/http"
	"strings"

	"go_starter/internal/appmode"
	"go_starter/internal/db"
	"go_starter/internal/session"
)

// dev_tenancy.go — kenaikan mode tenancy dari panel platform (keputusan 0007).
//
// Dipisah dari dev_settings.go karena sifatnya berbeda dari pengaturan lain di
// sana: yang lain bisa diubah bolak-balik, yang ini SEKALI JALAN. Menaruhnya di
// file yang sama akan membuatnya terbaca seperti setelan biasa.

// DevSettingsTenancy — POST /dev/settings/tenancy. Naikkan aplikasi dari satu
// aplikasi ke multi-workspace.
//
// Tak ada pasangan "turun", dan itu bukan kelalaian: menurunkan mode setelah ada
// banyak workspace akan menyembunyikan sisanya — kehilangan data yang tampak
// seperti bug UI. Database menolaknya lewat trigger, jadi menambahkan handler
// penurunan pun hanya akan menghasilkan tombol yang selalu gagal.
//
// Berlaku SEKETIKA tanpa restart: seluruh route sudah berbentuk /w/{slug} sejak
// mode single, dan gerbang route khas-multi dibaca per-request (mw.RequireMulti).
func (h *Handler) DevSettingsTenancy(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Sudah multi → tak ada yang perlu dikerjakan. Bukan error: operator mungkin
	// menekan tombol lama di tab yang belum disegarkan, dan memarahinya untuk itu
	// tak membantu siapa pun.
	if appmode.IsMulti() {
		http.Redirect(w, r, "/dev/settings", http.StatusSeeOther)
		return
	}

	prim, err := h.primaryTenant(ctx)
	if err != nil {
		h.Log.Error("tenancy: baca workspace primer", "err", err)
		http.Redirect(w, r, "/dev/settings?err=failed", http.StatusSeeOther)
		return
	}

	// Konfirmasi diketik, bukan dicentang. Aksi ini permanen, dan checkbox bisa
	// dicentang tanpa dibaca — menyalin nama menuntut orangnya melihat objek yang
	// terlibat. Dibandingkan case-insensitive & trim: yang diuji adalah PERHATIAN,
	// bukan ketelitian mengetik.
	if !strings.EqualFold(strings.TrimSpace(r.FormValue("confirm")), prim.Name) {
		http.Redirect(w, r, "/dev/settings?err=confirm", http.StatusSeeOther)
		return
	}

	uid := session.UserID(ctx)
	if err := db.WithSuper(ctx, h.Pool, func(q *db.Queries) error {
		return UpgradeToMulti(ctx, q, uid)
	}); err != nil {
		h.Log.Error("tenancy: naikkan ke multi", "err", err)
		http.Redirect(w, r, "/dev/settings?err=failed", http.StatusSeeOther)
		return
	}

	// Audit WAJIB di sini, bukan sekadar bagus: ini perubahan bentuk aplikasi yang
	// tak bisa dibatalkan, jadi "siapa dan kapan" harus terekam. target = workspace
	// primer (satu-satunya objek yang ada saat kenaikan).
	h.auditPlatform(ctx, uid, "platform.tenancy.upgrade", prim.ID,
		map[string]string{"to": appmode.NameMulti})
	http.Redirect(w, r, "/dev/settings?ok=tenancy", http.StatusSeeOther)
}

// primaryTenant membaca workspace primer lewat jalur platform. WithSuper: route
// /dev lintas-workspace, jadi tak ada tenant untuk di-scope.
func (h *Handler) primaryTenant(ctx context.Context) (db.Tenant, error) {
	var t db.Tenant
	err := db.WithSuper(ctx, h.Pool, func(q *db.Queries) error {
		var e error
		t, e = q.GetPrimaryTenant(ctx)
		return e
	})
	return t, err
}
