package handler

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"go_starter/internal/db"
)

// slug.go — turunkan slug workspace (tenant) yang URL-safe & UNIK dari nama.
// slug = identitas mesin (immutable, unik); nama = tampilan (boleh duplikat).
// Pola GitHub/Slack: dua workspace boleh sama nama, dibedakan slug (acme, acme-2).

// maxSlugAttempts membatasi loop auto-suffix (jaring pengaman, bukan batas nyata —
// tabrakan slug puluhan kali praktis mustahil untuk satu nama).
const maxSlugAttempts = 100

// slugify menurunkan basis slug dari nama: lowercase, non-alnum → '-', rangkap '-'
// diringkas, trim '-' di ujung. Kosong/simbol-saja → "workspace" (fallback aman).
func slugify(name string) string {
	var b strings.Builder
	prevDash := false
	for _, r := range strings.TrimSpace(name) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			prevDash = false
		case r >= 'A' && r <= 'Z':
			b.WriteRune(r + 32) // ke lowercase
			prevDash = false
		default:
			if !prevDash && b.Len() > 0 {
				b.WriteByte('-')
				prevDash = true
			}
		}
	}
	s := strings.Trim(b.String(), "-")
	if s == "" {
		return "workspace"
	}
	return s
}

// uniqueSlug mengembalikan slug bebas-tabrakan berbasis `base`: coba base, lalu
// base-2, base-3, ... hingga TenantSlugExists=false. Dipanggil DALAM tx (register/
// oauth WithSuper) SEBELUM CreateTenant — cek+insert di tx sama menutup race
// (UNIQUE constraint slug tetap jaring terakhir bila dua tx bersaing).
func uniqueSlug(ctx context.Context, q *db.Queries, base string) (string, error) {
	for i := 1; i <= maxSlugAttempts; i++ {
		candidate := base
		if i > 1 {
			candidate = base + "-" + strconv.Itoa(i)
		}
		exists, err := q.TenantSlugExists(ctx, candidate)
		if err != nil {
			return "", err
		}
		if !exists {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("slug tak tersedia setelah %d percobaan untuk %q", maxSlugAttempts, base)
}

// emailLocal mengembalikan bagian sebelum '@' (dipakai sebagai nama workspace
// default untuk user OAuth — dirapikan user via pengaturan). "" → "workspace".
func emailLocal(email string) string {
	if i := strings.IndexByte(email, '@'); i >= 0 {
		email = email[:i]
	}
	if email == "" {
		return "workspace"
	}
	return email
}
