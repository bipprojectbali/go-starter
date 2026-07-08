// Package oauth membungkus alur Google OAuth 2.0 / OIDC: authorization-code
// flow dengan PKCE, verifikasi id_token, dan ekstraksi claim.
//
// Hanya *flow* — session & penyimpanan user ditangani lapisan lain (scs, sqlc).
// Ini padanan idiomatik Go untuk "Login with Google" (bukan framework auth):
// x/oauth2 (flow + PKCE) + go-oidc (verifikasi id_token).
package oauth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

// googleIssuer adalah issuer OIDC Google (dipakai discovery + verifikasi iss).
const googleIssuer = "https://accounts.google.com"

// Claims adalah subset claim id_token yang kita butuhkan untuk login.
type Claims struct {
	Sub           string // OIDC subject — id user Google yang immutable (kunci tautan)
	Email         string
	EmailVerified bool
}

// Verifier memverifikasi raw id_token dan mengembalikan claim tepercaya.
// Diabstraksi jadi interface agar handler bisa di-test tanpa memanggil Google.
type Verifier interface {
	// VerifyIDToken memverifikasi signature + iss/aud, mencocokkan nonce, lalu
	// mengembalikan claim. Menolak bila email belum terverifikasi provider.
	VerifyIDToken(ctx context.Context, rawIDToken, wantNonce string) (*Claims, error)
}

// Provider menyatukan konfigurasi oauth2 + verifier OIDC untuk Google.
// Mengimplementasikan Verifier.
type Provider struct {
	oauth    *oauth2.Config
	verifier *oidc.IDTokenVerifier
}

// New membangun Provider: discovery OIDC Google + konfigurasi oauth2.
// clientID/secret/redirectURL dari config (env). scope openid+email+profile.
func New(ctx context.Context, clientID, clientSecret, redirectURL string) (*Provider, error) {
	oidcProvider, err := oidc.NewProvider(ctx, googleIssuer)
	if err != nil {
		return nil, fmt.Errorf("oauth: discovery Google OIDC: %w", err)
	}
	return &Provider{
		oauth: &oauth2.Config{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			RedirectURL:  redirectURL,
			Endpoint:     google.Endpoint,
			Scopes:       []string{oidc.ScopeOpenID, "email", "profile"},
		},
		verifier: oidcProvider.Verifier(&oidc.Config{ClientID: clientID}),
	}, nil
}

// AuthURL membangun URL consent Google dengan proteksi lengkap:
//   - state: anti-CSRF (dicek constant-time di callback)
//   - nonce: anti-replay id_token
//   - PKCE S256: anti authorization-code injection
func (p *Provider) AuthURL(state, nonce, verifier string) string {
	return p.oauth.AuthCodeURL(state,
		oidc.Nonce(nonce),
		oauth2.S256ChallengeOption(verifier),
	)
}

// Exchange menukar authorization code jadi token, membuktikan kepemilikan PKCE
// verifier. Mengembalikan raw id_token dari field extra.
func (p *Provider) Exchange(ctx context.Context, code, verifier string) (string, error) {
	tok, err := p.oauth.Exchange(ctx, code, oauth2.VerifierOption(verifier))
	if err != nil {
		return "", fmt.Errorf("oauth: exchange code: %w", err)
	}
	rawIDToken, ok := tok.Extra("id_token").(string)
	if !ok || rawIDToken == "" {
		return "", fmt.Errorf("oauth: id_token tidak ada di respons token")
	}
	return rawIDToken, nil
}

// VerifyIDToken mengimplementasikan Verifier: verifikasi kriptografis id_token,
// cek nonce MANUAL (verifier.Verify TIDAK cek nonce), lalu tolak bila email
// belum terverifikasi provider (email_verified=false → kelas nOAuth).
func (p *Provider) VerifyIDToken(ctx context.Context, rawIDToken, wantNonce string) (*Claims, error) {
	idToken, err := p.verifier.Verify(ctx, rawIDToken)
	if err != nil {
		return nil, fmt.Errorf("oauth: verifikasi id_token: %w", err)
	}
	// verifier.Verify TIDAK memeriksa nonce — wajib manual (jebakan paling umum).
	if idToken.Nonce != wantNonce {
		return nil, fmt.Errorf("oauth: nonce tidak cocok")
	}
	var claims struct {
		Email         string `json:"email"`
		EmailVerified bool   `json:"email_verified"`
	}
	if err := idToken.Claims(&claims); err != nil {
		return nil, fmt.Errorf("oauth: baca claim: %w", err)
	}
	if claims.Email == "" {
		return nil, fmt.Errorf("oauth: id_token tanpa email (scope email hilang?)")
	}
	if !claims.EmailVerified {
		return nil, fmt.Errorf("oauth: email belum terverifikasi provider")
	}
	return &Claims{
		Sub:           idToken.Subject,
		Email:         claims.Email,
		EmailVerified: claims.EmailVerified,
	}, nil
}

// NewState menghasilkan token acak (state / nonce) 32 byte hex dari crypto/rand.
func NewState() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("oauth: generate random token: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// NewVerifier menghasilkan PKCE code verifier (native x/oauth2).
func NewVerifier() string { return oauth2.GenerateVerifier() }
