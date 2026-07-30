package handler

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"go_starter/internal/appmode"
	"go_starter/internal/auth"
	"go_starter/internal/authz"
	"go_starter/internal/db"
	"go_starter/internal/session"
	"go_starter/internal/ui/pages"

	"github.com/jackc/pgx/v5"
)

// maxWorkspaceNameLen membatasi panjang nama workspace (validasi input —
// user-modifiable, WAJIB divalidasi backend).
const maxWorkspaceNameLen = 60

// LoginPage — GET /login (full page). Form password hanya di dev (devMode).
// ?err= (dari redirect PRG) → alert. Menutup juga jalur /login?err=inactive dari
// RefreshIdentity/OAuth yang dulu tak pernah dirender (pesan hilang senyap).
func (h *Handler) LoginPage(w http.ResponseWriter, r *http.Request) {
	h.renderPage(w, r, "Masuk", pages.Login(devMode, authErrMsg(r.URL.Query().Get("err"))))
}

// RegisterPage — GET /register (full page). ?err= → alert (pola PRG).
func (h *Handler) RegisterPage(w http.ResponseWriter, r *http.Request) {
	h.renderPage(w, r, "Daftar", pages.Register(authErrMsg(r.URL.Query().Get("err")), appmode.IsMulti()))
}

// authErrMsg memetakan kode error auth (query ?err=) ke pesan ramah. Kode ringkas
// di URL (bukan pesan penuh) agar rapi + tak bocor detail. "" → tak ada alert.
func authErrMsg(code string) string {
	switch code {
	case "invalid":
		return "Email atau password salah"
	case "fields":
		return "Email dan password wajib diisi"
	case "short":
		return "Password minimal 8 karakter"
	case "exists":
		return "Email sudah terdaftar"
	case "workspace":
		return "Nama workspace wajib diisi"
	case "inactive":
		return "Akun tidak aktif. Hubungi administrator."
	default:
		return ""
	}
}

// Register — POST /register (native form, PRG). Buat workspace+owner, lalu login
// otomatis & redirect 303 ke home. Gagal → redirect /register?err=CODE.
func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	email := strings.TrimSpace(strings.ToLower(r.FormValue("email")))
	password := r.FormValue("password")
	// Nama workspace hanya relevan di mode MULTI (pendaftar membuat workspace-nya
	// sendiri). Di mode single ia bergabung ke aplikasi yang sudah ada & sudah
	// bernama — mewajibkannya di sana membuat pendaftaran mustahil diselesaikan,
	// sebab formnya pun tak menanyakannya.
	workspace := strings.TrimSpace(r.FormValue("workspace"))
	if appmode.IsMulti() && (workspace == "" || len(workspace) > maxWorkspaceNameLen) {
		http.Redirect(w, r, "/register?err=workspace", http.StatusSeeOther)
		return
	}
	if email == "" || password == "" {
		http.Redirect(w, r, "/register?err=fields", http.StatusSeeOther)
		return
	}
	if len(password) < 8 {
		http.Redirect(w, r, "/register?err=short", http.StatusSeeOther)
		return
	}

	hash, err := auth.HashPassword(password)
	if err != nil {
		h.Log.Error("hash password", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	// Register = buat USER (identitas global) + WORKSPACE pertama + MEMBERSHIP owner.
	// Pre-identity: tenant belum ada → WithSuper (bypass RLS). Nama = input user;
	// slug = unik auto-suffix (nama boleh duplikat, slug tidak). Ketiganya dalam
	// SATU tx → atomic (gagal di tengah tak meninggalkan user/tenant yatim).
	var user db.User
	var tenantID int64
	err = db.WithSuper(r.Context(), h.Pool, func(q *db.Queries) error {
		u, e := q.CreateUser(r.Context(), db.CreateUserParams{Email: email, PassHash: &hash})
		if e != nil {
			return e
		}
		user = u
		// Penempatan berbeda per mode (0006 §6): multi = workspace baru + owner,
		// single = gabung ke aplikasi tunggal sebagai MEMBER. Dipusatkan agar
		// jalur ini & OAuth tak bisa menyimpang satu sama lain.
		t, e := placeNewUser(r.Context(), q, u.ID, workspace)
		if e != nil {
			return e
		}
		tenantID = t.ID
		return nil
	})
	if err != nil {
		// Email/slug unik: pelanggaran constraint = email sudah terpakai.
		h.Log.Warn("register: create tenant+user", "err", err)
		http.Redirect(w, r, "/register?err=exists", http.StatusSeeOther)
		return
	}

	if err := h.startIdentity(r, user, "password", tenantID); err != nil {
		h.Log.Error("register: start identity", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	h.acceptPendingInvite(r) // datang lewat tautan undangan? langsung gabung
	// Native redirect (bukan SSE): scs LoadAndSave commit cookie otomatis — tak
	// perlu WriteCookie manual (itu hanya utk jalur NewSSE yang bypass scs).
	http.Redirect(w, r, homeFor(r.Context()), http.StatusSeeOther)
}

// Login — POST /login (native form, PRG). Verifikasi argon2id, mulai session,
// redirect 303 ke home. Gagal → /login?err=CODE (pesan generik anti-enumeration).
func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	email := strings.TrimSpace(strings.ToLower(r.FormValue("email")))
	password := r.FormValue("password")
	if email == "" || password == "" {
		http.Redirect(w, r, "/login?err=fields", http.StatusSeeOther)
		return
	}

	// Pre-identity: tenant user belum diketahui (login MEMUTUSKANNYA dari hasil ini).
	// email UNIQUE global → WithSuper (bypass RLS) untuk mencari lintas-tenant.
	var user db.User
	err := db.WithSuper(r.Context(), h.Pool, func(q *db.Queries) error {
		u, e := q.GetUserByEmail(r.Context(), email)
		user = u
		return e
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// Kode generik — jangan bocorkan email terdaftar/tidak (anti user-enumeration).
			http.Redirect(w, r, "/login?err=invalid", http.StatusSeeOther)
			return
		}
		h.Log.Error("login: get user", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	// Akun Google-only (pass_hash NULL) tak bisa login password. Kode generik
	// yang sama (anti user-enumeration) — jangan ungkap bahwa akun ini OAuth.
	if user.PassHash == nil {
		http.Redirect(w, r, "/login?err=invalid", http.StatusSeeOther)
		return
	}

	ok, err := auth.VerifyPassword(password, *user.PassHash)
	if err != nil {
		h.Log.Error("login: verify", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if !ok {
		http.Redirect(w, r, "/login?err=invalid", http.StatusSeeOther)
		return
	}

	if err := h.startIdentity(r, user, "password", 0); err != nil {
		if errors.Is(err, errAccountBlocked) || errors.Is(err, errAccountDisabled) {
			http.Redirect(w, r, "/login?err=inactive", http.StatusSeeOther)
			return
		}
		h.Log.Error("login: start identity", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	h.acceptPendingInvite(r) // datang lewat tautan undangan? langsung gabung
	http.Redirect(w, r, homeFor(r.Context()), http.StatusSeeOther)
}

// Logout — POST /logout (native form, PRG). Hancurkan session, redirect 303.
// SELALU HTTP redirect (bukan SSE): redirect via SSE menyuntik <script> yang
// diblokir CSP proyek — logout tak akan berpindah halaman (lihat gotcha).
func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	// Jejak logout SEBELUM Clear (uid hilang setelahnya). Fail-soft.
	if uid := session.UserID(r.Context()); uid != 0 {
		h.auditAuth(r.Context(), uid, "auth.logout", "")
	}
	if err := session.Clear(r.Context()); err != nil {
		h.Log.Error("logout", "err", err)
	}
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

// errAccountBlocked & errAccountDisabled = status yang menolak login.
var (
	errAccountBlocked  = errors.New("akun diblokir")
	errAccountDisabled = errors.New("akun dinonaktifkan")
)

// startIdentity memutar token session (anti fixation), lalu menyimpan identitas
// lengkap: workspace AKTIF + role EFEKTIF di workspace itu + gate status. Root env
// kebal gate status. Dipakai password login & Google callback. method
// ("password"/"google") dicatat di audit auth.login.
//
// preferTenant: workspace yang ingin diaktifkan (mis. baru dibuat saat register).
// 0 = pilih workspace pertama user. Bila user tak punya workspace sama sekali,
// session tetap dibuat dgn tenant 0 — middleware Scope yang mengarahkan ke
// /workspace/new (keadaan sah di model membership).
func (h *Handler) startIdentity(r *http.Request, u db.User, method string, preferTenant int64) error {
	isRoot := isSuperAdminEmail(u.Email)

	// Gate status — root lolos (tak bisa dikunci lewat status DB).
	if !isRoot {
		switch u.Status {
		case "blocked":
			return errAccountBlocked
		case "disabled":
			return errAccountDisabled
		}
	}

	// Workspace aktif + role tenant di dalamnya. Pre-identity (belum ada tx ber-
	// scope) → WithSuper; memberships/platform_staff memang tanpa RLS.
	var (
		tenantID   int64
		tenantName string
		tenantSlug string
		tenantRole = authz.RoleNameMember
	)
	if err := db.WithSuper(r.Context(), h.Pool, func(q *db.Queries) error {
		// super_admin = OWNER workspace primer (0007). Dilakukan SEBELUM membaca
		// keanggotaan, supaya baris yang baru dibuat ikut terpilih di login yang
		// SAMA — kalau sesudahnya, super_admin pertama mendarat tanpa workspace
		// dan diarahkan ke /workspace/new padahal rumahnya sudah ada.
		//
		// Fail-soft: kehilangan kepemilikan sementara jauh lebih ringan daripada
		// super_admin yang tak bisa masuk sama sekali.
		if isRoot {
			if e := h.ensurePrimaryOwner(r.Context(), q, u.ID); e != nil {
				h.Log.Warn("startIdentity: pasang owner workspace primer", "uid", u.ID, "err", e)
			}
		}
		ms, e := q.ListMembershipsByUser(r.Context(), u.ID)
		if e != nil || len(ms) == 0 {
			return e // tanpa workspace → tenant tetap 0 (Scope arahkan buat baru)
		}
		pick := ms[0] // default: workspace pertama (terlama)
		if preferTenant != 0 {
			for _, m := range ms {
				if m.TenantID == preferTenant {
					pick = m
					break
				}
			}
		}
		tenantID, tenantName, tenantSlug, tenantRole = pick.TenantID, pick.Name, pick.Slug, pick.Role
		return nil
	}); err != nil {
		h.Log.Warn("startIdentity: load memberships", "err", err)
	}

	// Role efektif 2-bidang: env super_admin > staff (platform_staff) > role tenant
	// di workspace aktif. Lookup staff fail-soft.
	role := tenantRole
	if isRoot {
		role = authz.RoleNameSuperAdmin // env override menang atas semua
	} else if staff, err := h.isStaff(r.Context(), u.Email); err == nil && staff {
		role = authz.RoleNameStaff
	}

	if err := session.Renew(r.Context()); err != nil {
		return err
	}
	avatar := ""
	if u.AvatarUrl != nil {
		avatar = *u.AvatarUrl
	}
	session.SetIdentity(r.Context(), u.ID, u.Email, role, isRoot, tenantID, tenantName, tenantSlug, avatar)
	// Jejak login (fail-soft) — untuk panel aktivitas. actor = user sendiri.
	h.auditAuth(r.Context(), u.ID, "auth.login", method)
	return nil
}

// isStaff melaporkan apakah email = operator platform (platform_staff). Dipakai
// jalur pre-identity (login/oauth) — platform_staff TANPA RLS, WithSuper aman.
func (h *Handler) isStaff(ctx context.Context, email string) (bool, error) {
	var ok bool
	err := db.WithSuper(ctx, h.Pool, func(q *db.Queries) error {
		v, e := q.IsPlatformStaff(ctx, email)
		ok = v
		return e
	})
	return ok, err
}
