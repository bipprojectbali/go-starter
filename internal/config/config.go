// Package config memuat konfigurasi dari environment ke struct typed.
// Semua env dibaca HANYA di sini — tidak tersebar ke package lain.
package config

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config menampung seluruh konfigurasi runtime aplikasi.
type Config struct {
	Port           string // PORT, default "8080"
	DatabaseURL    string // DATABASE_URL (wajib) — role OWNER: migrate + boot (BYPASS RLS)
	AppDatabaseURL string // APP_DATABASE_URL — role app_rw NOBYPASSRLS: pool runtime (RLS mengikat). Default = DatabaseURL bila kosong (dev).
	RedisAddr      string // REDIS_ADDR (wajib)
	Env            string // ENV: "dev" | "production"
	AutoMigrate    bool   // AUTO_MIGRATE, default true
	SessionKey     string // SESSION_KEY (wajib di production)

	// Google OAuth. Opsional di dev (tombol Google nonaktif bila kosong),
	// WAJIB di production (Google = jalur login utama).
	GoogleClientID     string // GOOGLE_CLIENT_ID
	GoogleClientSecret string // GOOGLE_CLIENT_SECRET
	GoogleRedirectURL  string // GOOGLE_REDIRECT_URL (exact-match Google Console)

	// SuperAdminEmails = super-admin "sejati" (root immutable). Email di sini
	// selalu super_admin & kebal demote/block/delete lewat app. SUPER_ADMIN_EMAILS
	// dipisah koma. Disimpan lower-case untuk perbandingan case-insensitive.
	SuperAdminEmails []string

	// AppTimezone = zona waktu untuk agregasi tampilan (mis. "jam berapa user
	// aktif" di panel logs). Data disimpan UTC; TZ ini hanya untuk konversi saat
	// baca. APP_TIMEZONE (IANA, mis. "Asia/Jakarta"). Divalidasi saat load.
	AppTimezone string

	// MaxWorkspacesPerUser = kuota DEFAULT workspace yang boleh DIMILIKI (role
	// owner) tiap user; dipakai saat user dibuat. Kolom users.workspace_quota
	// menyimpannya per-user agar platform bisa override tanpa ubah env (lever
	// monetisasi: free/pro/enterprise). MAX_WORKSPACES_PER_USER, default 3.
	MaxWorkspacesPerUser int
}

// Location mem-parse AppTimezone jadi *time.Location. Sudah divalidasi di
// MustLoad, jadi aman; fallback UTC bila entah bagaimana gagal.
func (c *Config) Location() *time.Location {
	loc, err := time.LoadLocation(c.AppTimezone)
	if err != nil {
		return time.UTC
	}
	return loc
}

// IsSuperAdminEmail melaporkan apakah email termasuk root super-admin dari env.
func (c *Config) IsSuperAdminEmail(email string) bool {
	e := strings.ToLower(strings.TrimSpace(email))
	for _, s := range c.SuperAdminEmails {
		if s == e {
			return true
		}
	}
	return false
}

// GoogleEnabled melaporkan apakah OAuth Google terkonfigurasi (kredensial ada).
func (c *Config) GoogleEnabled() bool {
	return c.GoogleClientID != "" && c.GoogleClientSecret != "" && c.GoogleRedirectURL != ""
}

// IsProduction melaporkan apakah aplikasi berjalan di mode production.
func (c *Config) IsProduction() bool { return c.Env == "production" }

// MustLoad memuat konfigurasi dengan fail-fast: env wajib yang kosong
// menyebabkan panic saat startup, bukan error senyap di request pertama.
func MustLoad() *Config {
	c := &Config{
		Port:                 getEnv("PORT", "8080"),
		DatabaseURL:          mustEnv("DATABASE_URL"),
		AppDatabaseURL:       getEnv("APP_DATABASE_URL", ""),
		RedisAddr:            mustEnv("REDIS_ADDR"),
		Env:                  getEnv("ENV", "dev"),
		AutoMigrate:          getEnv("AUTO_MIGRATE", "true") == "true",
		GoogleClientID:       getEnv("GOOGLE_CLIENT_ID", ""),
		GoogleClientSecret:   getEnv("GOOGLE_CLIENT_SECRET", ""),
		GoogleRedirectURL:    getEnv("GOOGLE_REDIRECT_URL", ""),
		SuperAdminEmails:     parseEmailList(getEnv("SUPER_ADMIN_EMAILS", "")),
		AppTimezone:          getEnv("APP_TIMEZONE", "Asia/Jakarta"),
		MaxWorkspacesPerUser: getEnvInt("MAX_WORKSPACES_PER_USER", 3),
	}
	// Validasi TZ fail-fast: nama IANA salah → panic saat startup, bukan error
	// senyap saat render panel logs. (tzdata di-embed via import di main.)
	if _, err := time.LoadLocation(c.AppTimezone); err != nil {
		panic(fmt.Sprintf("config: APP_TIMEZONE %q tidak valid: %v", c.AppTimezone, err))
	}
	// Dual-DSN: pool runtime pakai APP_DATABASE_URL (role app_rw NOBYPASSRLS) agar
	// RLS mengikat. Kosong → fallback DATABASE_URL (dev: owner, RLS TAK mengikat —
	// isolasi tetap benar via WHERE, tapi tak ada jaring RLS). Di production SANGAT
	// disarankan set APP_DATABASE_URL ke role non-owner (defense-in-depth).
	if c.AppDatabaseURL == "" {
		c.AppDatabaseURL = c.DatabaseURL
	}
	if c.IsProduction() {
		c.SessionKey = mustEnv("SESSION_KEY")
		// Di production Google adalah jalur login utama — wajib ada.
		c.GoogleClientID = mustEnv("GOOGLE_CLIENT_ID")
		c.GoogleClientSecret = mustEnv("GOOGLE_CLIENT_SECRET")
		c.GoogleRedirectURL = mustEnv("GOOGLE_REDIRECT_URL")
	}
	return c
}

// LoadMigrateConfig memuat konfigurasi MINIMAL untuk subcommand `migrate`:
// HANYA DATABASE_URL. Sengaja TIDAK memakai MustLoad — migrate cuma butuh
// koneksi DB, sedang MustLoad mewajibkan SESSION_KEY/Google/REDIS di production
// (yang tak relevan untuk container migrate-only). Return error (bukan panic)
// agar caller (runMigrate) bisa exit(1) rapi. Env tetap dibaca HANYA di sini.
func LoadMigrateConfig() (*Config, error) {
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		return nil, fmt.Errorf("config: env %q wajib di-set untuk migrate", "DATABASE_URL")
	}
	return &Config{DatabaseURL: url}, nil
}

// LoadDotEnv memuat pasangan key=value dari file .env ke environment
// bila belum di-set. Tanpa dependency — parser sederhana untuk dev.
// Baris kosong dan yang diawali '#' diabaikan. Aman bila file tidak ada.
func LoadDotEnv(path string) error {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // .env opsional (mis. di production pakai env asli)
		}
		return fmt.Errorf("open %s: %w", path, err)
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key, val = strings.TrimSpace(key), unquote(strings.TrimSpace(val))
		if _, exists := os.LookupEnv(key); !exists {
			_ = os.Setenv(key, val)
		}
	}
	if err := sc.Err(); err != nil {
		return fmt.Errorf("scan %s: %w", path, err)
	}
	return nil
}

// unquote melepas SATU pasang kutip pembungkus ("..." atau '...') dari nilai .env
// — perilaku dotenv standar. Tanpa ini, DATABASE_URL="postgres://..." dibaca
// LITERAL berikut kutipnya → pgx gagal parse (database kosong). Kutip di tengah
// nilai tak tersentuh; hanya pasangan pembungkus persis yang dilepas.
func unquote(s string) string {
	if len(s) >= 2 {
		if c := s[0]; (c == '"' || c == '\'') && s[len(s)-1] == c {
			return s[1 : len(s)-1]
		}
	}
	return s
}

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok {
		return v
	}
	return fallback
}

// getEnvInt membaca env sebagai int positif. Kosong / tak valid / <=0 → fallback
// (jangan biarkan salah ketik jadi kuota 0 yang mengunci semua user).
func getEnvInt(key string, fallback int) int {
	v, ok := os.LookupEnv(key)
	if !ok {
		return fallback
	}
	n, err := strconv.Atoi(strings.TrimSpace(v))
	if err != nil || n <= 0 {
		return fallback
	}
	return n
}

// parseEmailList memecah string comma-separated jadi slice email lower-case,
// membuang entri kosong. Dipakai untuk SUPER_ADMIN_EMAILS.
func parseEmailList(raw string) []string {
	var out []string
	for _, part := range strings.Split(raw, ",") {
		e := strings.ToLower(strings.TrimSpace(part))
		if e != "" {
			out = append(out, e)
		}
	}
	return out
}

func mustEnv(key string) string {
	v, ok := os.LookupEnv(key)
	if !ok || v == "" {
		panic(fmt.Sprintf("config: env %q wajib di-set", key))
	}
	return v
}
