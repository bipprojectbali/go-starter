// Command go_stater — entry point: config, wiring, start server.
package main

import (
	"context"
	"embed"
	"errors"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
	_ "time/tzdata" // embed database tzdata: LoadLocation gagal di container minimal (CGO_ENABLED=0) tanpa ini

	"go_stater/internal/assets"
	"go_stater/internal/authz"
	"go_stater/internal/config"
	"go_stater/internal/database"
	"go_stater/internal/handler"
	"go_stater/internal/oauth"
	"go_stater/internal/session"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/rueidis"
)

//go:embed static
var staticEmbed embed.FS

//go:embed migrations/*.sql
var migrationsEmbed embed.FS

func main() {
	// Dispatch subcommand SEBELUM run(). `./app migrate` = jalankan migrasi lalu
	// exit — dipakai container migrate one-shot (compose service_completed_successfully)
	// SEBELUM app start. Pola stdlib (switch os.Args), tanpa framework CLI.
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "migrate":
			if err := runMigrate(); err != nil {
				slog.Error("migrate: fatal", "err", err)
				os.Exit(1)
			}
			os.Exit(0)
		default:
			slog.Error("unknown subcommand", "arg", os.Args[1])
			os.Exit(2)
		}
	}
	if err := run(); err != nil {
		slog.Error("fatal", "err", err)
		os.Exit(1)
	}
}

// runMigrate menjalankan HANYA migrasi lalu keluar. Dipakai container
// `./app migrate` (compose: condition service_completed_successfully) SEBELUM app
// start. Sengaja TIDAK memanggil config.MustLoad(): migrate hanya butuh
// DATABASE_URL, sedang MustLoad mewajibkan SESSION_KEY/Google di production.
// Tak buka Redis/OAuth/HTTP — reuse migrationsEmbed + MigrateWithLock (advisory lock).
func runMigrate() error {
	_ = config.LoadDotEnv(".env")
	cfg, err := config.LoadMigrateConfig()
	if err != nil {
		return err
	}
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	if err := database.MigrateWithLock(ctx, pool, migrationsEmbed); err != nil {
		return err
	}
	slog.Info("migrations applied")
	return nil
}

func run() error {
	// Config (dev: muat .env dulu bila ada).
	_ = config.LoadDotEnv(".env")
	cfg := config.MustLoad()

	log := newLogger(cfg)
	slog.SetDefault(log)

	// ctx dibatalkan saat SIGINT/SIGTERM → memicu graceful shutdown.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Postgres — DUA pool (defense-in-depth RLS):
	//   migratePool (DATABASE_URL, role owner)  → migrasi + boot, BYPASS RLS.
	//   pool (APP_DATABASE_URL, role app_rw)     → runtime handler, RLS MENGIKAT.
	// Di dev keduanya bisa DSN sama (owner) — RLS tak mengikat, tapi isolasi tetap
	// benar via GUC+WHERE. Di prod, app_rw NOBYPASSRLS = jaring pengaman terakhir.
	migratePool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer migratePool.Close()

	pool, err := pgxpool.New(ctx, cfg.AppDatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	// Redis (rueidis) — satu klien untuk session store.
	rc, err := rueidis.NewClient(rueidis.ClientOption{
		InitAddress: []string{cfg.RedisAddr},
	})
	if err != nil {
		return err
	}
	defer rc.Close()

	// Session manager (scs + rueidis store).
	sm := session.NewManager(rc)
	session.Init(sm)

	// Auto-migrate dengan advisory lock (aman multi-instance). Pakai migratePool
	// (owner) — migrasi ALTER/CREATE POLICY butuh privilege owner, bukan app_rw.
	if cfg.AutoMigrate {
		if err := database.MigrateWithLock(ctx, migratePool, migrationsEmbed); err != nil {
			return err
		}
		log.Info("migrations applied")
	}

	// Model role 2-bidang: super_admin = ENV-ONLY (SUPER_ADMIN_EMAILS), nol baris
	// DB, di-overlay RefreshIdentity per-request. Tak ada reconcile boot ke DB —
	// role users.role hanya owner/admin/member (CHECK). Kehadiran super_admin tak
	// pernah ditulis; sepenuhnya diturunkan dari env saat identity di-resolve.

	// Static server dengan cache-busting untuk app.css (berubah tiap `make css`).
	staticSub, err := fs.Sub(staticEmbed, "static")
	if err != nil {
		return err
	}
	assetSrv, err := assets.New(staticSub, "app.css")
	if err != nil {
		return err
	}
	handler.SetCSSPath(assetSrv.Path("app.css")) // inject path ber-hash ke Layout
	handler.SetDevMode(!cfg.IsProduction())      // password auth = dev-only
	handler.SetSuperAdminChecker(cfg.IsSuperAdminEmail)
	handler.SetAppTimezone(cfg.Location()) // TZ agregasi panel logs (tampilan jam lokal)

	// Authz (Casbin) — enforcer in-memory dari model+policy embed.
	enforcer, err := authz.New(authz.Model, authz.Policy)
	if err != nil {
		return err
	}
	authz.Init(enforcer)

	// Google OAuth — di-wire bila kredensial tersedia. Di dev tanpa kredensial,
	// app tetap start (tombol Google membalas 503 saat diklik).
	if cfg.GoogleEnabled() {
		gp, err := oauth.New(ctx, cfg.GoogleClientID, cfg.GoogleClientSecret, cfg.GoogleRedirectURL)
		if err != nil {
			return err
		}
		handler.SetGoogleOAuth(gp)
		log.Info("google oauth enabled")
	} else {
		log.Warn("google oauth disabled: kredensial GOOGLE_* tidak lengkap")
	}

	// Wiring handler + router.
	h := handler.New(pool, log)
	r := chi.NewRouter()
	registerRoutes(r, h, assetSrv.Handler(), log, !cfg.IsProduction())

	// Bungkus: CSRF (terluar) → session LoadAndSave → router.
	// CrossOriginProtection butuh Go ≥1.25.1 (CVE-2025-47910 di 1.25.0).
	csrf := http.NewCrossOriginProtection()
	handlerChain := csrf.Handler(sm.LoadAndSave(r))

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           handlerChain,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      0, // 0 = tanpa batas; SSE butuh koneksi panjang
		IdleTimeout:       60 * time.Second,
	}

	// Jalankan server di goroutine; error dikirim ke channel.
	serverErr := make(chan error, 1)
	go func() {
		log.Info("listening", "addr", srv.Addr, "env", cfg.Env)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
	}()

	// Tunggu: error server ATAU sinyal shutdown.
	select {
	case err := <-serverErr:
		return err
	case <-ctx.Done():
		stop() // berhenti tangkap sinyal (SIGTERM kedua = kill paksa)
		log.Info("shutdown: draining connections")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			return err
		}
		log.Info("shutdown: done")
		return nil
	}
}

func newLogger(cfg *config.Config) *slog.Logger {
	if cfg.IsProduction() {
		return slog.New(slog.NewJSONHandler(os.Stdout, nil))
	}
	return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
}
