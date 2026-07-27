package handler

import (
	"context"
	"errors"
	"net/http"
	"strings"

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
	h.renderPage(w, r, "Daftar", pages.Register(authErrMsg(r.URL.Query().Get("err"))))
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
	workspace := strings.TrimSpace(r.FormValue("workspace"))
	if workspace == "" || len(workspace) > maxWorkspaceNameLen {
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

	// Register = buat WORKSPACE (tenant) baru + user OWNER-nya (1 user = 1 workspace).
	// Pre-identity: tenant belum ada → WithSuper (bypass RLS). Nama = input user;
	// slug = unik auto-suffix (nama boleh duplikat, slug tidak). Tenant + user dalam
	// SATU tx → atomic (gagal di tengah tak meninggalkan tenant yatim).
	var user db.User
	err = db.WithSuper(r.Context(), h.Pool, func(q *db.Queries) error {
		slug, e := uniqueSlug(r.Context(), q, slugify(workspace))
		if e != nil {
			return e
		}
		t, e := q.CreateTenant(r.Context(), db.CreateTenantParams{Name: workspace, Slug: slug})
		if e != nil {
			return e
		}
		user, e = q.CreateUser(r.Context(), db.CreateUserParams{
			Email: email, PassHash: &hash,
			TenantID: t.ID, Role: authz.RoleNameOwner,
		})
		return e
	})
	if err != nil {
		// Email/slug unik: pelanggaran constraint = email sudah terpakai.
		h.Log.Warn("register: create tenant+user", "err", err)
		http.Redirect(w, r, "/register?err=exists", http.StatusSeeOther)
		return
	}

	if err := h.startIdentity(r, user, "password"); err != nil {
		h.Log.Error("register: start identity", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	// Native redirect (bukan SSE): scs LoadAndSave commit cookie otomatis — tak
	// perlu WriteCookie manual (itu hanya utk jalur NewSSE yang bypass scs).
	http.Redirect(w, r, authz.HomePathFor(session.Role(r.Context())), http.StatusSeeOther)
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

	if err := h.startIdentity(r, user, "password"); err != nil {
		if errors.Is(err, errAccountBlocked) || errors.Is(err, errAccountDisabled) {
			http.Redirect(w, r, "/login?err=inactive", http.StatusSeeOther)
			return
		}
		h.Log.Error("login: start identity", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, authz.HomePathFor(session.Role(r.Context())), http.StatusSeeOther)
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
// lengkap: role EFEKTIF (env super-admin override kolom DB) + gate status.
// Root env kebal gate status (owner tak bisa dikunci). Dipakai password login
// & Google callback. method ("password"/"google") dicatat di audit auth.login.
func (h *Handler) startIdentity(r *http.Request, u db.User, method string) error {
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

	// Role efektif 2-bidang: env super_admin > staff (platform_staff) > role tenant.
	// Lookup staff via WithSuper (pre-identity, platform_staff tanpa RLS). Fail-soft.
	role := u.Role
	if isRoot {
		role = authz.RoleNameSuperAdmin // env override apa pun nilai kolom DB
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
	// Nama workspace untuk cache session (brand sidebar). Pre-identity → WithSuper.
	// Fail-soft: gagal load → nama kosong (RefreshIdentity mengisi ulang tiap request).
	tenantName := ""
	if err := db.WithSuper(r.Context(), h.Pool, func(q *db.Queries) error {
		t, e := q.GetTenant(r.Context(), u.TenantID)
		if e == nil {
			tenantName = t.Name
		}
		return e
	}); err != nil {
		h.Log.Warn("startIdentity: load tenant name", "err", err)
	}
	session.SetIdentity(r.Context(), u.ID, u.Email, role, isRoot, u.TenantID, tenantName, avatar)
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
