// Package config memuat konfigurasi dari environment ke struct typed.
// Semua env dibaca HANYA di sini — tidak tersebar ke package lain.
package config

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// Config menampung seluruh konfigurasi runtime aplikasi.
type Config struct {
	Port        string // PORT, default "8080"
	DatabaseURL string // DATABASE_URL (wajib)
	RedisAddr   string // REDIS_ADDR (wajib)
	Env         string // ENV: "dev" | "production"
	AutoMigrate bool   // AUTO_MIGRATE, default true
	SessionKey  string // SESSION_KEY (wajib di production)

	// Google OAuth. Opsional di dev (tombol Google nonaktif bila kosong),
	// WAJIB di production (Google = jalur login utama).
	GoogleClientID     string // GOOGLE_CLIENT_ID
	GoogleClientSecret string // GOOGLE_CLIENT_SECRET
	GoogleRedirectURL  string // GOOGLE_REDIRECT_URL (exact-match Google Console)
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
		Port:               getEnv("PORT", "8080"),
		DatabaseURL:        mustEnv("DATABASE_URL"),
		RedisAddr:          mustEnv("REDIS_ADDR"),
		Env:                getEnv("ENV", "dev"),
		AutoMigrate:        getEnv("AUTO_MIGRATE", "true") == "true",
		GoogleClientID:     getEnv("GOOGLE_CLIENT_ID", ""),
		GoogleClientSecret: getEnv("GOOGLE_CLIENT_SECRET", ""),
		GoogleRedirectURL:  getEnv("GOOGLE_REDIRECT_URL", ""),
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
		key, val = strings.TrimSpace(key), strings.TrimSpace(val)
		if _, exists := os.LookupEnv(key); !exists {
			_ = os.Setenv(key, val)
		}
	}
	if err := sc.Err(); err != nil {
		return fmt.Errorf("scan %s: %w", path, err)
	}
	return nil
}

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok {
		return v
	}
	return fallback
}

func mustEnv(key string) string {
	v, ok := os.LookupEnv(key)
	if !ok || v == "" {
		panic(fmt.Sprintf("config: env %q wajib di-set", key))
	}
	return v
}
