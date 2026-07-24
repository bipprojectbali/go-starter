package handler

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"go_stater/internal/auth"
	"go_stater/internal/authz"
	"go_stater/internal/db"
	"go_stater/internal/session"
	"go_stater/internal/ui"
	"go_stater/internal/ui/pages"

	"github.com/jackc/pgx/v5"
	"github.com/starfederation/datastar-go/datastar"
	g "maragu.dev/gomponents"
)

// credentials adalah bentuk signals yang dikirim form auth Datastar. Workspace
// hanya diisi di register (nama workspace baru); login mengabaikannya.
type credentials struct {
	Workspace string `json:"workspace"`
	Email     string `json:"email"`
	Password  string `json:"password"`
}

// maxWorkspaceNameLen membatasi panjang nama workspace (validasi input signal —
// signal user-modifiable, WAJIB divalidasi backend, gotcha #15b).
const maxWorkspaceNameLen = 60

// LoginPage — GET /login (full page). Form password hanya di dev (devMode).
func (h *Handler) LoginPage(w http.ResponseWriter, r *http.Request) {
	h.renderPage(w, r, "Masuk", pages.Login(devMode))
}

// RegisterPage — GET /register (full page).
func (h *Handler) RegisterPage(w http.ResponseWriter, r *http.Request) {
	h.renderPage(w, r, "Daftar", pages.Register())
}

// authError mem-patch alert error auth via Datastar (id "auth-error").
func (h *Handler) authError(w http.ResponseWriter, r *http.Request, msg string) {
	patch(w, r, h.Log, ui.Alert(ui.VariantDestructive, "auth-error", g.Text(msg)))
}

// Register — POST /register. Buat user (argon2id), lalu login otomatis.
func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	var in credentials
	if err := datastar.ReadSignals(r, &in); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	email := strings.TrimSpace(strings.ToLower(in.Email))
	workspace := strings.TrimSpace(in.Workspace)
	if workspace == "" {
		h.authError(w, r, "Nama workspace wajib diisi")
		return
	}
	if len(workspace) > maxWorkspaceNameLen {
		h.authError(w, r, "Nama workspace maksimal 60 karakter")
		return
	}
	if email == "" || in.Password == "" {
		h.authError(w, r, "Email dan password wajib diisi")
		return
	}
	if len(in.Password) < 8 {
		h.authError(w, r, "Password minimal 8 karakter")
		return
	}

	hash, err := auth.HashPassword(in.Password)
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
		h.authError(w, r, "Email sudah terdaftar")
		return
	}

	if err := h.startIdentity(r, user, "password"); err != nil {
		h.Log.Error("register: start identity", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	// Tulis cookie SEBELUM NewSSE (NewSSE flush header & bypass scs — lihat auth.go doc).
	if err := session.WriteCookie(r.Context(), w); err != nil {
		h.Log.Error("register: write cookie", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	sse := datastar.NewSSE(w, r)
	_ = sse.Redirect(authz.HomePathFor(session.Role(r.Context())))
}

// Login — POST /login. Verifikasi argon2id, mulai session.
func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var in credentials
	if err := datastar.ReadSignals(r, &in); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	email := strings.TrimSpace(strings.ToLower(in.Email))
	if email == "" || in.Password == "" {
		h.authError(w, r, "Email dan password wajib diisi")
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
			// Pesan generik — jangan bocorkan email terdaftar/tidak (anti user-enumeration).
			h.authError(w, r, "Email atau password salah")
			return
		}
		h.Log.Error("login: get user", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	// Akun Google-only (pass_hash NULL) tak bisa login password. Pesan generik
	// yang sama (anti user-enumeration) — jangan ungkap bahwa akun ini OAuth.
	if user.PassHash == nil {
		h.authError(w, r, "Email atau password salah")
		return
	}

	ok, err := auth.VerifyPassword(in.Password, *user.PassHash)
	if err != nil {
		h.Log.Error("login: verify", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if !ok {
		h.authError(w, r, "Email atau password salah")
		return
	}

	if err := h.startIdentity(r, user, "password"); err != nil {
		if errors.Is(err, errAccountBlocked) || errors.Is(err, errAccountDisabled) {
			h.authError(w, r, "Akun tidak aktif. Hubungi administrator.")
			return
		}
		h.Log.Error("login: start identity", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if err := session.WriteCookie(r.Context(), w); err != nil {
		h.Log.Error("login: write cookie", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	sse := datastar.NewSSE(w, r)
	_ = sse.Redirect(authz.HomePathFor(session.Role(r.Context())))
}

// Logout — POST /logout. Hancurkan session.
func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	// Jejak logout SEBELUM Clear (uid hilang setelahnya). Fail-soft.
	if uid := session.UserID(r.Context()); uid != 0 {
		h.auditAuth(r.Context(), uid, "auth.logout", "")
	}
	if err := session.Clear(r.Context()); err != nil {
		h.Log.Error("logout", "err", err)
	}
	// Logout bisa dari form biasa atau Datastar; tangani keduanya.
	if r.Header.Get("Datastar-Request") == "true" {
		// Tulis cookie (Destroy → cookie kadaluwarsa) sebelum NewSSE.
		if err := session.WriteCookie(r.Context(), w); err != nil {
			h.Log.Error("logout: write cookie", "err", err)
		}
		sse := datastar.NewSSE(w, r)
		_ = sse.Redirect("/login")
		return
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
