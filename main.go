// Command go_stater — entry point: config, wiring, start server.
package main

import (
	"context"
	"embed"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"time"

	"go_stater/internal/config"
	"go_stater/internal/database"
	"go_stater/internal/handler"
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
	if err := run(); err != nil {
		slog.Error("fatal", "err", err)
		os.Exit(1)
	}
}

func run() error {
	// Config (dev: muat .env dulu bila ada).
	_ = config.LoadDotEnv(".env")
	cfg := config.MustLoad()

	log := newLogger(cfg)
	slog.SetDefault(log)

	ctx := context.Background()

	// Postgres pool.
	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
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

	// Auto-migrate dengan advisory lock (aman multi-instance).
	if cfg.AutoMigrate {
		if err := database.MigrateWithLock(ctx, pool, migrationsEmbed); err != nil {
			return err
		}
		log.Info("migrations applied")
	}

	// Static file server dari embed.FS (sub agar path bersih: /static/app.css).
	staticSub, err := fs.Sub(staticEmbed, "static")
	if err != nil {
		return err
	}
	staticFS := http.StripPrefix("/static/", http.FileServerFS(staticSub))

	// Wiring handler + router.
	h := handler.New(pool, log)
	r := chi.NewRouter()
	registerRoutes(r, h, staticFS)

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

	log.Info("listening", "addr", srv.Addr, "env", cfg.Env)
	return srv.ListenAndServe()
}

func newLogger(cfg *config.Config) *slog.Logger {
	if cfg.IsProduction() {
		return slog.New(slog.NewJSONHandler(os.Stdout, nil))
	}
	return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
}
