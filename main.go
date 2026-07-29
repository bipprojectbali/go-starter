// Command go_starter — entry point: config, wiring, start server.
package main

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"
	_ "time/tzdata" // embed database tzdata: LoadLocation gagal di container minimal (CGO_ENABLED=0) tanpa ini

	"go_starter/internal/appmode"
	"go_starter/internal/assets"
	"go_starter/internal/authz"
	"go_starter/internal/config"
	"go_starter/internal/database"
	"go_starter/internal/db"
	"go_starter/internal/handler"
	"go_starter/internal/oauth"
	"go_starter/internal/session"
	"go_starter/internal/settings"

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

	// Session manager (scs + rueidis store). Opsi keamanan DITURUNKAN dari
	// ENV=production — bukan env tersendiri yang bisa lupa diisi.
	sm := session.NewManager(rc, session.Options{
		Secure:     cfg.IsProduction(),
		CookieName: cfg.SessionCookieName(),
	})
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
	// Mode aplikasi di-set SEBELUM apa pun yang membentuk URL atau mendaftarkan
	// route (0006): mode menentukan bentuk keduanya, dan route dipasang sekali.
	appmode.Set(cfg.AppMode)
	// Mode single butuh TEPAT SATU tenant. Sengaja fail-fast: DB berisi >1
	// workspace + APP_MODE=single = app menolak start, sebab memilih diam-diam
	// salah satu membuat sisanya lenyap dari pandangan tanpa jejak.
	if err := handler.BootstrapSingleApp(ctx, migratePool, cfg.AppName); err != nil {
		return err
	}
	log.Info("mode aplikasi", "mode", cfg.AppMode.String())

	// BUKTIKAN isolasi tenant mengikat pada koneksi runtime — jangan percaya env.
	// Urutannya penting: SETELAH migrasi (tabel & policy harus sudah ada) dan
	// SETELAH appmode.Set (ketatnya bergantung mode), SEBELUM melayani request.
	//
	// Kenapa di sini, bukan di config: string DSN tak membuktikan apa pun. Mengisi
	// APP_DATABASE_URL dengan nilai yang SAMA seperti DATABASE_URL lolos setiap
	// pemeriksaan env sambil tetap menjalankan pool sebagai owner — lalu satu
	// query yang lupa `WHERE tenant_id` membocorkan data lintas-pelanggan tanpa
	// error. Yang bisa ditanyakan ke database jawabannya pasti.
	if err := verifyTenantIsolation(ctx, pool, cfg, log); err != nil {
		return err
	}

	// Alamat publik aplikasi — sumber redirect_uri OAuth & tautan undangan.
	// Di-set SEBELUM wiring OAuth di bawah (yang merakit redirect_uri darinya).
	handler.SetAppBaseURL(cfg.AppBaseURL)
	handler.SetCSSPath(assetSrv.Path("app.css")) // inject path ber-hash ke Layout
	handler.SetDevMode(!cfg.IsProduction())      // password auth = dev-only
	handler.SetSuperAdminChecker(cfg.IsSuperAdminEmail)
	handler.SetAppTimezone(cfg.Location()) // TZ agregasi panel logs (tampilan jam lokal)

	// Pengaturan platform: env jadi FALLBACK (dipakai bila baris DB belum ada,
	// mis. deployment baru), DB jadi sumber kebenaran yang bisa diubah operator
	// saat jalan. Fail-soft: gagal baca → jalan dgn fallback env, sebab kuota
	// yang sedikit basi jauh lebih baik daripada aplikasi menolak start.
	settings.SetFallback(settings.KeyWorkspaceQuotaDefault, strconv.Itoa(cfg.MaxWorkspacesPerUser))
	if kv, err := loadSettings(ctx, pool); err != nil {
		log.Error("settings: gagal memuat, pakai fallback env", "err", err)
	} else {
		settings.Load(kv)
	}

	// Authz (Casbin) — enforcer in-memory dari model+policy embed.
	enforcer, err := authz.New(authz.Model, authz.Policy)
	if err != nil {
		return err
	}
	authz.Init(enforcer)

	// Google OAuth — di-wire bila kredensial tersedia. Di dev tanpa kredensial,
	// app tetap start (tombol Google membalas 503 saat diklik).
	if cfg.GoogleEnabled() {
		// redirect_uri DIRAKIT dari base URL + path route (handler.PathGoogleCallback),
		// bukan dibaca dari env tersendiri: path yang sama tak boleh hidup di dua
		// tempat, sebab salah ketik di salah satunya hanya muncul sebagai
		// `redirect_uri_mismatch` dari Google — pesan yang tak menyebut sebabnya.
		gp, err := oauth.New(ctx, cfg.GoogleClientID, cfg.GoogleClientSecret, handler.GoogleRedirectURL())
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

// verifyTenantIsolation memastikan pool runtime benar-benar terikat RLS.
//
// Keras di MULTI-tenant (menolak start), sekadar catatan di single-app: di sana
// hanya ada satu tenant, jadi tak ada yang bisa bocor ke siapa pun — memaksa
// operator menyiapkan role terpisah untuk perlindungan yang tak ia butuhkan
// hanyalah gesekan. Aturannya menyesuaikan diri sendiri, tanpa env tambahan.
//
// Dev tetap boleh longgar: menjalankan Postgres dengan role terpisah hanya untuk
// `make dev` tak sepadan, dan isolasi di sana tetap benar via GUC + WHERE.
func verifyTenantIsolation(ctx context.Context, pool *pgxpool.Pool, cfg *config.Config, log *slog.Logger) error {
	st, err := db.CheckRLS(ctx, pool, "audit_logs")
	if err != nil {
		return err
	}
	if st.Binds() {
		log.Info("isolasi tenant: RLS mengikat", "role", st.User)
		return nil
	}
	// Tidak mengikat. Seberapa keras responsnya bergantung apakah ada yang bisa
	// bocor sama sekali.
	if cfg.IsProduction() && appmode.IsMulti() {
		// Pesan memuat SATU perintah siap-tempel. Role app_rw beserta seluruh
		// GRANT-nya sudah dibuat migrasi 00007 (termasuk ALTER DEFAULT PRIVILEGES,
		// sehingga tabel dari migrasi berikutnya ikut terjangkau otomatis) — yang
		// belum hanya kredensial login, sebab password tak boleh ada di repo.
		// Menyuruh orang menjalankan ulang GRANT yang sudah ada hanya menambah
		// friksi, dan friksi itulah yang mendorong mereka menyerah lalu
		// menyamakan DSN dengan DATABASE_URL.
		return fmt.Errorf("isolasi tenant TIDAK mengikat: %s.\n"+
			"Pool runtime (APP_DATABASE_URL) harus memakai role non-owner tanpa BYPASSRLS, "+
			"agar satu query yang lupa WHERE tenant_id mengembalikan 0 baris — bukan data "+
			"pelanggan lain.\n"+
			"Role app_rw & hak aksesnya SUDAH dibuat migrasi 00007; tinggal beri kredensial:\n"+
			"  ALTER ROLE app_rw LOGIN PASSWORD '<password>';\n"+
			"lalu set APP_DATABASE_URL=postgres://app_rw:<password>@host:port/db",
			st.Reason())
	}
	log.Warn("isolasi tenant TIDAK mengikat — RLS tak jadi jaring pengaman",
		"sebab", st.Reason(),
		"catatan", "dapat diterima di dev/single-app; WAJIB diperbaiki sebelum production multi-tenant")
	return nil
}

// loadSettings membaca seluruh pengaturan platform jadi map siap-cache.
// platform_settings TANPA RLS (pengaturan berlaku lintas-workspace), jadi
// WithSuper aman — dan memang perlu: saat boot belum ada tenant untuk di-scope.
func loadSettings(ctx context.Context, pool *pgxpool.Pool) (map[string]string, error) {
	kv := map[string]string{}
	err := db.WithSuper(ctx, pool, func(q *db.Queries) error {
		rows, e := q.ListSettings(ctx)
		if e != nil {
			return e
		}
		for _, s := range rows {
			kv[s.Key] = s.Value
		}
		return nil
	})
	return kv, err
}

func newLogger(cfg *config.Config) *slog.Logger {
	if cfg.IsProduction() {
		return slog.New(slog.NewJSONHandler(os.Stdout, nil))
	}
	return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
}
