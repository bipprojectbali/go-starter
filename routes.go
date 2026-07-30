package main

import (
	"log/slog"
	"net/http"

	"go_starter/internal/handler"
	"go_starter/internal/mw"

	"github.com/go-chi/chi/v5"
)

// registerRoutes mendaftarkan SEMUA route — single source of truth (§4.1).
// devMode (=!production) menentukan apakah auth password diaktifkan; di produksi
// hanya Google yang jadi jalur login.
func registerRoutes(r chi.Router, h *handler.Handler, staticFS http.Handler, log *slog.Logger, devMode bool) {
	// Urutan middleware penting: request-id dulu (dipakai log/recover),
	// lalu recover (tangkap panic downstream), log, security headers.
	r.Use(mw.RequestID)
	r.Use(mw.Recover(log))
	r.Use(mw.RequestLog(log))
	r.Use(mw.SecurityHeaders)

	// Health (tanpa auth)
	r.Get("/healthz", h.Liveness)
	r.Get("/readyz", h.Readiness)

	// Static (embedded, tanpa auth)
	r.Handle("/static/*", staticFS)

	// Browser tetap auto-minta /favicon.ico di root (bookmark, tab lama) walau
	// <head> menunjuk favicon.svg — redirect permanen agar tak jadi 404 di log.
	r.Get("/favicon.ico", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/static/favicon.svg", http.StatusMovedPermanently)
	})

	// Publik: landing page (TIDAK redirect ke /login). Scope buka tx ber-tenant
	// bila login (no-op anonim) — RefreshIdentity/TrackPresence butuh h.q(ctx).
	// RefreshIdentity agar redirect per-role pakai role SEGAR dari DB (self-heal
	// session lama). TrackPresence: rekam kehadiran bila login (no-op anonim).
	r.With(h.Scope, h.RefreshIdentity, h.TrackPresence).Get("/", h.Home)

	// Login Google — SELALU aktif (jalur login utama di produksi). Path pakai
	// prefix /api/auth/ agar exact-match dengan redirect URI di Google Console.
	r.Get("/api/auth/google", h.GoogleLogin)
	r.Get(handler.PathGoogleCallback, h.GoogleCallback)

	// GET /login selalu ada (target redirect RequireAuth); rendernya adaptif
	// (form password hanya muncul di dev — lihat handler.SetDevMode).
	r.Get("/login", h.LoginPage)
	r.Post("/logout", h.Logout)

	// Auth password — DEV-ONLY. Di produksi permukaan serang ini tidak ada;
	// hanya untuk mempermudah agen/dev masuk ke runtime.
	if devMode {
		r.Post("/login", h.Login)
		r.Get("/register", h.RegisterPage)
		r.Post("/register", h.Register)
	}

	// Semua route terproteksi memakai urutan middleware sama:
	// RequireAuth (authn) → RefreshIdentity (role/status SEGAR dari DB, self-heal
	// session lama + enforcement real-time) → RequireEnforce (authz per resource).

	// Panel /dev — owner/developer. Hanya super_admin/root lolos "dev:users".
	r.Route("/dev", func(r chi.Router) {
		r.Use(mw.RequireAuth)
		r.Use(h.Scope) // buka tx ber-tenant (RLS) SEBELUM RefreshIdentity butuh h.q
		r.Use(h.RefreshIdentity)
		r.Use(h.TrackPresence)
		r.Use(mw.RequireEnforce("dev:users", "read"))
		// /dev telanjang → arahkan ke halaman default panel (/dev/users). Tanpa
		// ini, /dev 404 (dan HomePath super_admin menuju /dev).
		r.Get("/", func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, "/dev/users", http.StatusSeeOther)
		})
		r.Get("/users", h.DevUsersList)
		r.Post("/users/{id}/role", h.DevUserSetRole)
		r.Post("/users/{id}/status", h.DevUserSetStatus)
		r.Post("/users/{id}/delete", h.DevUserDelete)

		// Workspace lintas-platform (0005): tangguhkan/aktifkan/pulihkan. Suspend
		// PLATFORM-ONLY dan sengaja tak punya padanan di sisi owner — kalau owner
		// bisa membatalkannya sendiri, gunanya hilang.
		//
		// Di mode single daftar ini berisi tepat satu baris — tak berguna sebagai
		// DAFTAR, tapi suspend/restore-nya tetap dibutuhkan sebagai maintenance
		// mode (satu-satunya cara menutup aplikasi sementara tanpa restart).
		r.Get("/workspaces", h.DevWorkspaces)
		r.Post("/workspaces/{id}/suspend", h.DevWorkspaceSuspend)
		r.Post("/workspaces/{id}/unsuspend", h.DevWorkspaceUnsuspend)
		r.Post("/workspaces/{id}/restore", h.DevWorkspaceRestore)

		// Pengaturan platform — berlaku SEKETIKA (tabel platform_settings + cache),
		// bukan env yang butuh restart. Hak khusus per-user diatur di /dev/users
		// karena di sanalah orangnya terlihat; dua tempat mengelola hal yang sama
		// hanya bikin salah satunya basi.
		//
		// Gate SENDIRI (platform:settings), BUKAN dev:users yang menjaga grup ini:
		// aturan di sini berlaku untuk SETIAP user di platform, jadi staff — yang
		// aksesnya sengaja dibatasi — tak boleh mengubahnya. Deny-default di
		// policy.csv; hanya super_admin (root) yang lolos.
		r.Group(func(r chi.Router) {
			r.Use(mw.RequireEnforce("platform:settings", "write"))
			r.Get("/settings", h.DevSettings)
			r.Post("/settings/quota", h.DevSettingsQuota)
			// Kenaikan mode tenancy: single → multi, SEKALI JALAN (0007). Di-gate
			// platform:settings yang sama — ini keputusan paling fundamental di
			// halaman ini, jadi tak boleh lebih longgar dari kuota.
			r.Post("/settings/tenancy", h.DevSettingsTenancy)
			r.Post("/users/{id}/quota", h.DevUserQuota)
			r.Post("/users/{id}/quota/reset", h.DevUserQuotaReset)
		})

		// Panel aktivitas user — TERSEDIA di produksi (data-driven, super_admin
		// butuh pantau aktivitas nyata). Beda dari /health & /erd yang dev-only.
		r.Get("/logs", h.DevLogs)

		// File Health — DEV-ONLY. Butuh source .go di disk (tak ada di
		// single-binary produksi). Tak didaftarkan di prod → menu pun tak muncul.
		if devMode {
			r.Get("/health", h.DevHealth)
			r.Get("/erd", h.DevERD)
		}
	})

	// Ruang kerja — /w/{workspace}/… (keputusan 0004). SATU ruang per workspace:
	// /admin & /user dilebur karena keduanya membedakan ROLE, bukan RESOURCE —
	// halaman anggotanya sama, yang beda hanya aksi yang boleh dilakukan.
	// Beda role = beda AKSI di halaman yang sama, BUKAN beda ALAMAT (satu orang
	// bisa admin di A & member di B; alamat yang ikut berubah = tautan rusak).
	registerWorkspaceRoutes(r, h)

	// Workspace: pindah & buat baru. SELALU didaftarkan (0007) — dulu bersyarat
	// mode, dan itu sudah tak sah begitu mode bisa NAIK saat aplikasi berjalan:
	// route yang dipasang sekali saat boot akan telanjur salah sesudahnya, dan
	// gejalanya adalah 404 pada menu yang baru saja muncul.
	//
	// Yang menahannya kini middleware `mw.RequireMulti`, dibaca PER-REQUEST. Itu
	// tetap memenuhi kaidah 0006 §4 (bukan cuma menu yang disembunyikan — jalurnya
	// benar-benar tertutup), sekaligus mengikuti mode tanpa restart.
	r.Group(func(r chi.Router) {
		r.Use(mw.RequireAuth)
		r.Use(mw.RequireMulti)
		r.Use(h.Scope)
		r.Use(h.RefreshIdentity)
		r.Post("/workspace/switch", h.WorkspaceSwitch)
	})
	r.Group(func(r chi.Router) {
		r.Use(mw.RequireAuth)
		r.Use(mw.RequireMulti)
		// TANPA Scope: user tanpa workspace tak punya tenant untuk di-scope.
		// Handler pakai db.WithSuper langsung (membership belum ada).
		r.Get("/workspace/new", h.WorkspaceNewPage)
		r.Post("/workspace/new", h.WorkspaceCreate)
	})
	r.Group(func(r chi.Router) {
		r.Use(mw.RequireAuth)
		// Unarchive SENGAJA di luar /w/{workspace} (0005 §4): gerbang read-only
		// workspace terarsip memblokir SEMUA POST di dalamnya, jadi pintu keluarnya
		// harus berada di luar ruangan yang ia buka. Konsekuensinya handler ini
		// memvalidasi keanggotaan & otoritasnya sendiri (isOwnerOf).
		//
		// TANPA RequireMulti: workspace primer memang tak bisa diarsipkan, tapi di
		// mode multi ada workspace lain yang bisa — dan pintu keluar tak boleh
		// bergantung pada mode.
		r.Post("/workspace/{workspace}/unarchive", h.WorkspaceUnarchive)
	})

	// Undangan — PUBLIK (penerima belum tentu punya akun). Tanpa Scope: penerima
	// belum jadi anggota workspace mana pun saat membuka tautan.
	r.Get("/invite/{token}", h.InvitePage)
	r.Post("/invite/{token}/accept", h.InviteAccept)

	// Notifikasi — LINTAS-PANEL (bukan milik /admin maupun /user): undangan bisa
	// datang dari workspace yang belum jadi milik user. Scope tetap dipasang agar
	// shell (switcher workspace) punya Queries ber-scope; data notifikasinya
	// sendiri dibaca via db.WithSuper karena lintas-workspace.
	r.Route("/notifications", func(r chi.Router) {
		r.Use(mw.RequireAuth)
		r.Use(h.Scope)
		r.Use(h.RefreshIdentity)
		r.Use(h.TrackPresence)
		r.Use(mw.RequireEnforce("notif:home", "read"))
		r.Get("/", h.NotificationsPage)
		r.Post("/invite/{token}/accept", h.NotificationAccept)
		r.Post("/invite/{token}/decline", h.NotificationDecline)
	})

}

// registerWorkspaceRoutes mendaftarkan grup /w/{workspace} — semua halaman yang
// cakupannya SATU workspace. Dipisah dari registerRoutes agar file tetap terbaca;
// tetap di routes.go supaya "semua route di satu tempat" tak dilanggar.
//
// Yang SENGAJA di luar grup ini (lihat 0004): /dev (platform, lintas-workspace),
// /notifications (milik user — undangan datang dari workspace yang belum jadi
// miliknya), /invite/{token} (publik), /workspace/new (belum punya workspace).
// Cakupan data menentukan bentuk path, bukan sebaliknya.
func registerWorkspaceRoutes(r chi.Router, h *handler.Handler) {
	// SATU pola untuk kedua mode (0007). Dulu mode single memakai "/app" telanjang,
	// sehingga menaikkan aplikasi ke multi mengubah setiap alamat yang sudah
	// tersebar — dan route harus didaftarkan ulang, yang berarti restart.
	r.Route(handler.WorkspacePrefix+"/{workspace}", func(r chi.Router) {
		r.Use(mw.RequireAuth)
		// Scope menerjemahkan {workspace} → tenant_id + memvalidasi keanggotaan;
		// bukan anggota → 404 (bukan 403: itu mengonfirmasi workspace-nya ada).
		r.Use(h.Scope)
		r.Use(h.RefreshIdentity)
		r.Use(h.TrackPresence)
		// Gerbang = user:home (semua anggota). Pembatasan per-aksi ada di handler
		// (canEditWorkspace/canManageMembers) — member tetap boleh MEMBUKA halaman.
		r.Use(mw.RequireEnforce("user:home", "read"))

		r.Get("/", h.WorkspaceHome)

		// Pengaturan workspace: admin+ boleh LIHAT, ganti nama = owner/platform
		// (di-guard handler, bukan route, agar admin tetap bisa membuka read-only).
		r.Get("/settings", h.WorkspaceSettings)
		r.Post("/settings", h.WorkspaceUpdate)

		// Siklus hidup oleh OWNER (0005). Keduanya POST di dalam workspace, jadi
		// otomatis tertolak saat workspace sudah diarsipkan — kecuali unarchive,
		// yang justru karena itu diletakkan di luar prefix ini.
		//
		// SELALU didaftarkan, tak lagi bergantung mode (0007). Yang menahannya
		// adalah `is_primary`: rumah aplikasi tak bisa diarsipkan/dihapus, dijaga
		// di HANDLER dan di SQL sekaligus. Itu penjagaan yang lebih baik daripada
		// "route-nya tak ada di mode single" — sebab mode bisa naik saat jalan,
		// dan route yang terdaftar bersyarat akan telanjur salah setelahnya.
		r.Post("/archive", h.WorkspaceArchive)
		r.Post("/delete", h.WorkspaceDelete)

		// Anggota (model membership). Lihat = semua anggota; ubah/keluarkan/undang
		// = owner/admin (di-guard handler via canManageMembers).
		r.Get("/members", h.MembersPage)
		r.Post("/members/{id}/role", h.MemberSetRole)
		r.Post("/members/{id}/remove", h.MemberRemove)
		r.Post("/members/invite", h.InviteCreate)
		r.Post("/members/invite/{id}/delete", h.InviteDelete)
	})
}
