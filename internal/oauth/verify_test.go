package oauth

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"strings"
	"testing"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	jose "github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
)

const (
	testIssuer   = "https://test-issuer.example.com"
	testClientID = "test-client-id"
	testKeyID    = "test-key-1"
)

// newTestProvider membangun Provider dengan verifier lokal (JWKS in-memory),
// tanpa discovery HTTP ke Google. Mengembalikan provider + fungsi sign untuk
// membuat id_token uji.
func newTestProvider(t *testing.T) (*Provider, func(claims map[string]any) string) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("gen key: %v", err)
	}
	pubKeySet := &oidc.StaticKeySet{PublicKeys: []crypto.PublicKey{&key.PublicKey}}
	verifier := oidc.NewVerifier(testIssuer, pubKeySet, &oidc.Config{ClientID: testClientID})
	p := &Provider{verifier: verifier}

	signer, err := jose.NewSigner(
		jose.SigningKey{Algorithm: jose.RS256, Key: key},
		(&jose.SignerOptions{}).WithType("JWT").WithHeader("kid", testKeyID),
	)
	if err != nil {
		t.Fatalf("signer: %v", err)
	}
	sign := func(claims map[string]any) string {
		raw, err := jwt.Signed(signer).Claims(claims).Serialize()
		if err != nil {
			t.Fatalf("sign: %v", err)
		}
		return raw
	}
	return p, sign
}

// baseClaims = id_token valid dasar (issuer/aud/exp benar).
func baseClaims() map[string]any {
	now := time.Now()
	return map[string]any{
		"iss":            testIssuer,
		"aud":            testClientID,
		"sub":            "google-sub-123",
		"exp":            now.Add(time.Hour).Unix(),
		"iat":            now.Unix(),
		"email":          "user@gmail.com",
		"email_verified": true,
		"nonce":          "the-nonce",
		"picture":        "https://lh3.googleusercontent.com/a/x=s96-c",
		"name":           "Test User",
	}
}

func TestVerifyIDToken_Success(t *testing.T) {
	p, sign := newTestProvider(t)
	claims, err := p.VerifyIDToken(context.Background(), sign(baseClaims()), "the-nonce")
	if err != nil {
		t.Fatalf("VerifyIDToken: %v", err)
	}
	if claims.Sub != "google-sub-123" || claims.Email != "user@gmail.com" {
		t.Errorf("claim salah: %+v", claims)
	}
	if !claims.EmailVerified || claims.Name != "Test User" {
		t.Errorf("claim tambahan salah: %+v", claims)
	}
}

func TestVerifyIDToken_NonceMismatch(t *testing.T) {
	p, sign := newTestProvider(t)
	// Anti-replay: nonce tak cocok → ditolak (jebakan paling umum, dicek manual).
	_, err := p.VerifyIDToken(context.Background(), sign(baseClaims()), "nonce-lain")
	if err == nil || !strings.Contains(err.Error(), "nonce") {
		t.Errorf("nonce mismatch harus ditolak, got %v", err)
	}
}

func TestVerifyIDToken_EmailNotVerified(t *testing.T) {
	p, sign := newTestProvider(t)
	c := baseClaims()
	c["email_verified"] = false
	_, err := p.VerifyIDToken(context.Background(), sign(c), "the-nonce")
	if err == nil || !strings.Contains(err.Error(), "terverifikasi") {
		t.Errorf("email_verified=false harus ditolak (nOAuth), got %v", err)
	}
}

func TestVerifyIDToken_EmptyEmail(t *testing.T) {
	p, sign := newTestProvider(t)
	c := baseClaims()
	delete(c, "email")
	_, err := p.VerifyIDToken(context.Background(), sign(c), "the-nonce")
	if err == nil || !strings.Contains(err.Error(), "email") {
		t.Errorf("email kosong harus ditolak, got %v", err)
	}
}

func TestVerifyIDToken_WrongAudience(t *testing.T) {
	p, sign := newTestProvider(t)
	c := baseClaims()
	c["aud"] = "penyerang-client-id"
	_, err := p.VerifyIDToken(context.Background(), sign(c), "the-nonce")
	if err == nil {
		t.Error("audience salah harus ditolak verifier")
	}
}
