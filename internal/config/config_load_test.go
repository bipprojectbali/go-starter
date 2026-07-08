package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGoogleEnabled(t *testing.T) {
	full := Config{GoogleClientID: "a", GoogleClientSecret: "b", GoogleRedirectURL: "c"}
	if !full.GoogleEnabled() {
		t.Error("ketiga kredensial terisi → enabled")
	}
	// Tiap satu kosong → disabled.
	for _, c := range []Config{
		{GoogleClientSecret: "b", GoogleRedirectURL: "c"},
		{GoogleClientID: "a", GoogleRedirectURL: "c"},
		{GoogleClientID: "a", GoogleClientSecret: "b"},
		{},
	} {
		if c.GoogleEnabled() {
			t.Errorf("kredensial tak lengkap harus disabled: %+v", c)
		}
	}
}

func TestIsProduction(t *testing.T) {
	if !(&Config{Env: "production"}).IsProduction() {
		t.Error("'production' → true")
	}
	// Exact-match (case-sensitive).
	for _, env := range []string{"dev", "", "Production", "PROD"} {
		if (&Config{Env: env}).IsProduction() {
			t.Errorf("%q tak boleh production", env)
		}
	}
}

func TestGetEnv(t *testing.T) {
	t.Setenv("GS_TEST_FOO", "bar")
	if getEnv("GS_TEST_FOO", "fallback") != "bar" {
		t.Error("env terisi harus dipakai")
	}
	if getEnv("GS_TEST_UNSET_XYZ", "fallback") != "fallback" {
		t.Error("env tak ada harus fallback")
	}
	// Set-tapi-kosong → "" (bukan fallback) — beda dgn mustEnv.
	t.Setenv("GS_TEST_EMPTY", "")
	if getEnv("GS_TEST_EMPTY", "fallback") != "" {
		t.Error("env set-kosong harus \"\", bukan fallback")
	}
}

// setMinimalEnv menyiapkan env dev minimal (isolasi via t.Setenv).
func setMinimalEnv(t *testing.T) {
	t.Helper()
	t.Setenv("DATABASE_URL", "postgres://x")
	t.Setenv("REDIS_ADDR", "localhost:6379")
	// bersihkan yang bisa mengganggu
	for _, k := range []string{"PORT", "ENV", "AUTO_MIGRATE", "SESSION_KEY",
		"GOOGLE_CLIENT_ID", "GOOGLE_CLIENT_SECRET", "GOOGLE_REDIRECT_URL", "SUPER_ADMIN_EMAILS"} {
		t.Setenv(k, "")
		os.Unsetenv(k)
	}
}

func TestMustLoad_DevDefaults(t *testing.T) {
	setMinimalEnv(t)
	c := MustLoad()
	if c.Port != "8080" || c.Env != "dev" || !c.AutoMigrate {
		t.Errorf("default dev salah: port=%q env=%q autoMigrate=%v", c.Port, c.Env, c.AutoMigrate)
	}
	if c.GoogleEnabled() {
		t.Error("dev tanpa kredensial Google → disabled")
	}
}

func TestMustLoad_MissingRequiredPanics(t *testing.T) {
	for _, missing := range []string{"DATABASE_URL", "REDIS_ADDR"} {
		t.Run(missing, func(t *testing.T) {
			setMinimalEnv(t)
			os.Unsetenv(missing)
			defer func() {
				r := recover()
				if r == nil {
					t.Errorf("%s hilang harus panic", missing)
				}
			}()
			MustLoad()
		})
	}
}

func TestMustLoad_ProductionRequiresSecrets(t *testing.T) {
	// Production tanpa SESSION_KEY → panic.
	setMinimalEnv(t)
	t.Setenv("ENV", "production")
	func() {
		defer func() {
			if recover() == nil {
				t.Error("production tanpa SESSION_KEY harus panic")
			}
		}()
		MustLoad()
	}()

	// Production dgn semua secret → tak panic.
	setMinimalEnv(t)
	t.Setenv("ENV", "production")
	t.Setenv("SESSION_KEY", "k")
	t.Setenv("GOOGLE_CLIENT_ID", "a")
	t.Setenv("GOOGLE_CLIENT_SECRET", "b")
	t.Setenv("GOOGLE_REDIRECT_URL", "c")
	c := MustLoad()
	if !c.IsProduction() || !c.GoogleEnabled() {
		t.Error("production lengkap harus load bersih")
	}
}

func TestMustLoad_AutoMigrateFlag(t *testing.T) {
	setMinimalEnv(t)
	t.Setenv("AUTO_MIGRATE", "false")
	if MustLoad().AutoMigrate {
		t.Error("AUTO_MIGRATE=false → false")
	}
	setMinimalEnv(t)
	t.Setenv("AUTO_MIGRATE", "yes") // hanya "true" literal yang true
	if MustLoad().AutoMigrate {
		t.Error("AUTO_MIGRATE=yes bukan true literal → false")
	}
}

func TestLoadDotEnv_MissingFileIsNil(t *testing.T) {
	if err := LoadDotEnv(filepath.Join(t.TempDir(), "nope.env")); err != nil {
		t.Errorf("file tak ada harus nil (opsional): %v", err)
	}
}

func TestLoadDotEnv_Parsing(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	content := "# komentar\n\nKEY_A = val_a \nKEY_B=val_b\nTANPA_SAMA_DENGAN\nKEY_C=c\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	// Pastikan bersih dulu.
	for _, k := range []string{"KEY_A", "KEY_B", "KEY_C"} {
		os.Unsetenv(k)
	}
	// KEY_C sudah di-set → TIDAK boleh ditimpa (precedence).
	t.Setenv("KEY_C", "sudah-ada")

	if err := LoadDotEnv(path); err != nil {
		t.Fatalf("LoadDotEnv: %v", err)
	}
	if os.Getenv("KEY_A") != "val_a" { // di-trim
		t.Errorf("KEY_A=%q, want val_a (ter-trim)", os.Getenv("KEY_A"))
	}
	if os.Getenv("KEY_B") != "val_b" {
		t.Errorf("KEY_B=%q", os.Getenv("KEY_B"))
	}
	if os.Getenv("KEY_C") != "sudah-ada" {
		t.Errorf("KEY_C tak boleh ditimpa, got %q", os.Getenv("KEY_C"))
	}
	os.Unsetenv("KEY_A")
	os.Unsetenv("KEY_B")
}
