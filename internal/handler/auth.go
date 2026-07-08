package handler

import (
	"errors"
	"net/http"
	"strings"

	"go_stater/internal/auth"
	"go_stater/internal/db"
	"go_stater/internal/session"
	"go_stater/internal/ui"
	"go_stater/internal/ui/pages"

	"github.com/jackc/pgx/v5"
	"github.com/starfederation/datastar-go/datastar"
	g "maragu.dev/gomponents"
)

// credentials adalah bentuk signals yang dikirim form auth Datastar.
type credentials struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

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

	user, err := h.DB.CreateUser(r.Context(), db.CreateUserParams{Email: email, PassHash: &hash})
	if err != nil {
		// Email unik: pelanggaran constraint = email sudah terpakai.
		h.Log.Warn("register: create user", "err", err)
		h.authError(w, r, "Email sudah terdaftar")
		return
	}

	if err := h.startIdentity(r, user); err != nil {
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
	_ = sse.Redirect("/todos")
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

	user, err := h.DB.GetUserByEmail(r.Context(), email)
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

	if err := h.startIdentity(r, user); err != nil {
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
	_ = sse.Redirect("/todos")
}

// Logout — POST /logout. Hancurkan session.
func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
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
// & Google callback.
func (h *Handler) startIdentity(r *http.Request, u db.User) error {
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

	role := u.Role
	if isRoot {
		role = "super_admin" // env override apa pun nilai kolom DB
	}

	if err := session.Renew(r.Context()); err != nil {
		return err
	}
	avatar := ""
	if u.AvatarUrl != nil {
		avatar = *u.AvatarUrl
	}
	session.SetIdentity(r.Context(), u.ID, u.Email, role, isRoot, avatar)
	return nil
}
