package handler

import (
	"context"
	"crypto/subtle"
	"errors"
	"net/http"

	"go_stater/internal/db"
	"go_stater/internal/oauth"
	"go_stater/internal/session"

	"github.com/jackc/pgx/v5"
)

// googleProvider adalah kontrak minimal yang dibutuhkan handler dari OAuth
// provider. Interface (bukan *oauth.Provider konkret) agar test bisa menyuntik
// stub tanpa memanggil Google. *oauth.Provider memenuhi ini.
type googleProvider interface {
	AuthURL(state, nonce, verifier string) string
	Exchange(ctx context.Context, code, verifier string) (rawIDToken string, err error)
	oauth.Verifier // VerifyIDToken(ctx, rawIDToken, wantNonce) (*Claims, error)
}

// googleOAuth di-inject saat startup via SetGoogleOAuth (pola setter global,
// konsisten dgn SetCSSPath/session.Init). nil bila Google tak terkonfigurasi.
var googleOAuth googleProvider

// SetGoogleOAuth menyuntik provider Google (dipanggil dari main bila kredensial
// tersedia). Bila tak dipanggil, handler membalas 503 (Google nonaktif).
func SetGoogleOAuth(p googleProvider) { googleOAuth = p }

// GoogleLogin — GET /api/auth/google. Mulai authorization-code flow: generate
// state/nonce/verifier, simpan di session, redirect ke consent Google.
func (h *Handler) GoogleLogin(w http.ResponseWriter, r *http.Request) {
	if googleOAuth == nil {
		http.Error(w, "login Google tidak tersedia", http.StatusServiceUnavailable)
		return
	}
	state, err := oauth.NewState()
	if err != nil {
		h.Log.Error("google login: state", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	nonce, err := oauth.NewState()
	if err != nil {
		h.Log.Error("google login: nonce", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	verifier := oauth.NewVerifier()

	session.PutOAuthFlow(r.Context(), state, nonce, verifier)
	// Commit session + tulis cookie SEBELUM redirect (state harus persist agar
	// bisa diverifikasi saat callback).
	if err := session.WriteCookie(r.Context(), w); err != nil {
		h.Log.Error("google login: write cookie", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, googleOAuth.AuthURL(state, nonce, verifier), http.StatusFound)
}

// GoogleCallback — GET /api/auth/callback/google. Verifikasi state (anti-CSRF),
// tukar code (PKCE), verifikasi id_token (+nonce, +email_verified), lalu
// find-or-link user dalam satu transaksi dan mulai session.
func (h *Handler) GoogleCallback(w http.ResponseWriter, r *http.Request) {
	if googleOAuth == nil {
		http.Error(w, "login Google tidak tersedia", http.StatusServiceUnavailable)
		return
	}
	ctx := r.Context()

	// Ambil state/nonce/verifier tersimpan lalu HAPUS (one-time), apa pun hasilnya.
	wantState, nonce, verifier := session.OAuthFlow(ctx)
	session.ClearOAuthFlow(ctx)

	// Anti-CSRF: state dari Google harus cocok dengan yang kita simpan.
	gotState := r.URL.Query().Get("state")
	if wantState == "" || subtle.ConstantTimeCompare([]byte(gotState), []byte(wantState)) != 1 {
		h.Log.Warn("google callback: state mismatch")
		http.Error(w, "state tidak valid", http.StatusBadRequest)
		return
	}
	// Google mengirim error (mis. user membatalkan consent).
	if e := r.URL.Query().Get("error"); e != "" {
		h.Log.Warn("google callback: provider error", "error", e)
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	code := r.URL.Query().Get("code")
	rawIDToken, err := googleOAuth.Exchange(ctx, code, verifier)
	if err != nil {
		h.Log.Error("google callback: exchange", "err", err)
		http.Error(w, "gagal menukar kode", http.StatusBadGateway)
		return
	}
	claims, err := googleOAuth.VerifyIDToken(ctx, rawIDToken, nonce)
	if err != nil {
		// email_verified=false, nonce mismatch, signature invalid → tolak.
		h.Log.Warn("google callback: verify id_token", "err", err)
		http.Error(w, "verifikasi Google gagal", http.StatusUnauthorized)
		return
	}

	userID, err := h.findOrLinkGoogleUser(ctx, claims)
	if err != nil {
		h.Log.Error("google callback: find-or-link", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	// Mulai session: putar token (anti-fixation) lalu tandai login.
	if err := session.Renew(ctx); err != nil {
		h.Log.Error("google callback: renew", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	session.SetUserID(ctx, userID)
	if err := session.WriteCookie(ctx, w); err != nil {
		h.Log.Error("google callback: write cookie", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/todos", http.StatusSeeOther)
}

// findOrLinkGoogleUser memetakan claim Google ke user lokal dalam SATU transaksi:
//  1. oauth_account untuk (google, sub) ada → login user itu.
//  2. belum ada → user dgn email itu ada → AUTO-LINK (buat oauth_account).
//  3. tak ada → buat user OAuth baru + oauth_account.
//
// Auto-link aman di model ini: email_verified sudah dipastikan true oleh
// VerifyIDToken, dan password auth hanya dev-only (bukan permukaan takeover di prod).
func (h *Handler) findOrLinkGoogleUser(ctx context.Context, claims *oauth.Claims) (int64, error) {
	const provider = "google"

	tx, err := h.Pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx) //nolint — no-op bila sudah Commit
	q := h.DB.WithTx(tx)

	// 1. Identitas Google sudah tertaut?
	acc, err := q.GetOAuthAccount(ctx, db.GetOAuthAccountParams{Provider: provider, ProviderUid: claims.Sub})
	switch {
	case err == nil:
		return acc.UserID, tx.Commit(ctx)
	case !errors.Is(err, pgx.ErrNoRows):
		return 0, err
	}

	// 2. User dgn email itu sudah ada (mis. akun password dev) → auto-link.
	user, err := q.GetUserByEmail(ctx, claims.Email)
	switch {
	case err == nil:
		if _, err := q.CreateOAuthAccount(ctx, db.CreateOAuthAccountParams{
			UserID: user.ID, Provider: provider, ProviderUid: claims.Sub,
		}); err != nil {
			return 0, err
		}
		return user.ID, tx.Commit(ctx)
	case !errors.Is(err, pgx.ErrNoRows):
		return 0, err
	}

	// 3. User baru sepenuhnya dari Google.
	newUser, err := q.CreateOAuthUser(ctx, claims.Email)
	if err != nil {
		return 0, err
	}
	if _, err := q.CreateOAuthAccount(ctx, db.CreateOAuthAccountParams{
		UserID: newUser.ID, Provider: provider, ProviderUid: claims.Sub,
	}); err != nil {
		return 0, err
	}
	return newUser.ID, tx.Commit(ctx)
}
