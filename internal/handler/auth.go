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

// LoginPage — GET /login (full page).
func (h *Handler) LoginPage(w http.ResponseWriter, r *http.Request) {
	h.renderPage(w, r, "Masuk", pages.Login())
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

	user, err := h.DB.CreateUser(r.Context(), db.CreateUserParams{Email: email, PassHash: hash})
	if err != nil {
		// Email unik: pelanggaran constraint = email sudah terpakai.
		h.Log.Warn("register: create user", "err", err)
		h.authError(w, r, "Email sudah terdaftar")
		return
	}

	if err := h.startSession(r, user.ID); err != nil {
		h.Log.Error("register: start session", "err", err)
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

	ok, err := auth.VerifyPassword(in.Password, user.PassHash)
	if err != nil {
		h.Log.Error("login: verify", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if !ok {
		h.authError(w, r, "Email atau password salah")
		return
	}

	if err := h.startSession(r, user.ID); err != nil {
		h.Log.Error("login: start session", "err", err)
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

// startSession memutar token (anti fixation) lalu menandai user login.
func (h *Handler) startSession(r *http.Request, userID int64) error {
	if err := session.Renew(r.Context()); err != nil {
		return err
	}
	session.SetUserID(r.Context(), userID)
	return nil
}
