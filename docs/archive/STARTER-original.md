
> **⚠️ ARSIP — dokumen historis, jangan dijadikan acuan.** Ini spesifikasi desain
> ASLI (`STARTER.md`), diarsipkan 2026-08-11. Sebagian sudah **usang** vs kode kini
> (mis. Basecoat→daisyUI, contoh `todos`, narasi pengganti-HTMX, River, argon2id).
> Acuan yang benar: [`CLAUDE.md`](../../CLAUDE.md) (konvensi + gotcha),
> [`README.md`](../../README.md) (cara pakai), `docs/decisions/` (keputusan
> arsitektur mutakhir). Disimpan untuk konteks "kenapa awalnya begini", bukan "begini
> sekarang".

# STARTER.md — go_starter

Spesifikasi starter template **`go_starter`** — web base super cepat, super ringan, modern (2026), agent-friendly, dengan output **single binary**.

> Dokumen ini adalah single source of truth untuk membangun starter template.
> Prinsip desain: **satu bahasa, satu binary, satu perintah build, error ditangkap compile-time semaksimal mungkin.**
>
> **Pemakai:** dipakai sendiri (bukan publik). Server sudah punya PostgreSQL, Redis, dan MinIO berjalan.
> Konsekuensi desain: **Postgres jadi default sejak awal** (dev = prod, tanpa kejutan dialect). MinIO, job queue, dan realtime bersifat **add-on opsional** — core bisa jalan tanpa mereka.
>
> **Arsitektur berlapis:**
> - **Core (wajib):** Go + Chi + gomponents + **Datastar** + Postgres (pgx/sqlc) + goose + session (scs + rueidis) + Tailwind + Basecoat CSS. `make dev` langsung hidup.
> - **Add-on (opsional, di-enable saat butuh):** Job queue (River), File storage (MinIO), WebAuthn/passkey.
>
> **Keputusan arsitektur kunci (hasil audit stack Juli 2026):**
> - **Interaktivitas + realtime = Datastar** (satu file ~11.8 KiB), menggantikan trio HTMX + Alpine + SSE-manual. SSE-native, SDK Go bertipe, jembatan resmi ke gomponents (`maragu.dev/gomponents-datastar`). Menghindari utang migrasi HTMX 4 (yang menghapus `sse-swap`).
> - **Satu klien Redis: rueidis** — untuk session store (scs Store kustom) & realtime bus. Jangan campur dua klien Redis.
> - **Password: argon2id** (`golang.org/x/crypto/argon2`, pure-Go) — OWASP #1 preferred 2026.
> - **CSRF: `http.CrossOriginProtection`** (stdlib, Go 1.25.1+) — sebelumnya absen total.
> - **UI: Basecoat 1.0 vendored CSS** (bukan port manual) + wrapper gomponents tipis.

---

## 1. Prinsip Desain

1. **Satu bahasa** — seluruh stack (backend, view, query) ditulis dalam Go + SQL. Tanpa Node.js, tanpa bundler, tanpa runtime kedua.
2. **Satu binary** — static assets, migrations, dan template di-embed via `embed.FS`. Binary = satu file; *runtime* tetap butuh Postgres + Redis (bukan "deploy nol-infra", tapi "deploy nol-artefak-tambahan").
3. **Compile-time feedback** — kesalahan (query salah kolom, komponen salah variant, event SSE salah bentuk) ketahuan saat `go build` / `sqlc generate`, bukan runtime. Datastar SDK Go bertipe menjaga produksi event SSE tetap lolos type-checker.
4. **Agent-friendly** — zero magic, zero konvensi tersembunyi, semua wiring eksplisit dan grep-able. Workflow tunggal via `Makefile`. **Satu paradigma interaktivitas (Datastar)**, bukan tiga — lebih sedikit konsep untuk agent nalar.
5. **Vendored assets** — tidak ada CDN. Semua JS/CSS disimpan lokal dan di-embed, dengan **versi + checksum tercatat** (lihat §13).

---

## 2. Stack Resmi

### 2.1 Core (wajib — template tidak jalan tanpa ini)

| # | Layer | Teknologi | Versi/Catatan (terverifikasi Juli 2026) |
|---|-------|-----------|---------------|
| 1 | Bahasa | Go | 1.26+ (latest 1.26.5). Butuh **≥1.25.1** untuk `http.CrossOriginProtection` (CSRF). Method routing sejak 1.22 |
| 2 | HTTP Server | `net/http` (stdlib) | `http.Server` dengan timeout eksplisit. HTTP/2 aktif di belakang Traefik (wajib untuk SSE Datastar) |
| 3 | Router | Chi (`go-chi/chi/v5`) | v5.x, aktif. Dipakai untuk `r.Group`/`r.Use`/sub-router mount — celah nyata stdlib. Pure-Go, zero transitive deps, bisa dilepas |
| 4 | Middleware | Chi middleware + custom | `func(http.Handler) http.Handler` standar |
| 5 | View/HTML | gomponents (`maragu.dev/gomponents`) | **v1.3.0** (terverifikasi spike), zero-dep. HTML sebagai fungsi Go — satu sumber kebenaran, no codegen, paling agent-friendly. ⚠ `g.Attr` panic bila >1 value; `g.If` eager (pakai `g.Iff` untuk lazy) |
| 6 | Interaktivitas + Realtime | **Datastar** (`data-star.dev`, vendored) | **JS runtime v1.0.2 (~11.8 KiB) + Go SDK `starfederation/datastar-go` v1.2.2** (versioning TERPISAH, kompatibel di jalur API v1). Satu file gantikan HTMX+Alpine+SSE-ext. Jembatan gomponents resmi (`maragu.dev/gomponents-datastar` v0.3.3) |
| 7 | Styling | Tailwind CSS v4 standalone CLI | **v4.3.2, binary musl** untuk distroless terverifikasi. Auto content-detection baca class di `.go`. Tanpa Node |
| 8 | Design system | **Basecoat 1.0** (`basecoatui.com`, vendored CSS) | v1.0.1 kini **file CSS matang** (root-class API: `btn`, `card`, `field`). Vendor `basecoat.css`, tulis wrapper gomponents tipis. **Bukan** port manual 20 komponen |
| 9 | Database | PostgreSQL + `pgx/v5` | Pool via `pgxpool`. **Default sejak awal** (dev = prod) |
| 10 | Query layer | sqlc | SQL murni → Go type-safe. **Bukan ORM**. Query dinamis kompleks → tulis manual di layer repo |
| 11 | Migration | goose | SQL polos, embed via `embed.FS`. Auto-migrate + **advisory lock** (§4.7) — aman multi-instance |
| 12 | Session | scs (`alexedwards/scs/v2`) | Store: **rueidis** via Store kustom (~3 method). Accessor typed di satu file (§4.9) |
| 13 | Password | **argon2id** (`golang.org/x/crypto/argon2`) | OWASP #1 preferred 2026. Pure-Go. (bcrypt masih acceptable tapi legacy) |
| 14 | CSRF | `http.CrossOriginProtection` (stdlib) | **Go 1.25.1+.** Nol dependency. Di dev tanpa TLS → fallback cek Origin/Host (§12) |
| 15 | Redis client | **rueidis** | **Satu klien** untuk session store & realtime bus. Client-side caching + auto-pipelining |
| 16 | Validation | manual + go-playground/validator | Form sederhana → validasi manual di boundary (lebih selaras). validator/v10 untuk struct kompleks (opsional) |
| 17 | Logging | `log/slog` (stdlib) | Structured, JSON handler di production. Request ID via middleware |
| 18 | Dev live-reload | air | Satu config `.air.toml` |

### 2.2 Add-on (opsional — di-enable saat butuh, jangan pasang kalau belum perlu)

| # | Layer | Teknologi | Kapan dipakai |
|---|-------|-----------|---------------|
| A | Background job | **River** (`riverqueue/river`) | Job queue **Postgres-based** (bukan Redis). **v0.40.0** (masih pra-v1, tapi API matang & typed generics), migration di-embed. ⚠ `rivermigrate.New` return `(*Migrator, error)`; `AddWorker` panic pada duplikat (pakai `AddWorkerSafely`) |
| B | File storage | minio-go | MinIO existing. Upload: `MaxBytesReader` + streaming (jaga RAM), validasi content-type (§12) |
| C | Passkey / WebAuthn | `go-webauthn/webauthn` | Login tanpa password. Butuh sedikit JS — dokumentasikan, jangan paksakan ke core |
| D | Obfuscation | garble | `garble -tiny -literals build`. Verifikasi kompat Go 1.26 sebelum pakai |

### 2.3 Larangan Eksplisit (untuk manusia dan agent)

- ❌ ORM (GORM, ent) — pakai sqlc.
- ❌ `html/template` untuk view — pakai gomponents (error runtime, gugur prinsip compile-time).
- ❌ React/Vue/Svelte/bundler apa pun.
- ❌ **HTMX / Alpine.js** — digantikan Datastar. Jangan campur paradigma.
- ❌ Asset dari CDN — semua vendored di `static/`, dengan checksum tercatat.
- ❌ Mendaftarkan route di luar `routes.go`.
- ❌ Mengedit manual file hasil generate sqlc di `internal/db/`.
- ❌ `map[string]string` untuk variant komponen — pakai typed const/enum (§4.5).
- ❌ Key session string mentah (`"userID"`) tersebar — pakai accessor typed (§4.9).
- ❌ **Dua klien Redis** — hanya rueidis, untuk session + bus.
- ❌ WebSocket di core — realtime pakai SSE (Datastar). WS hanya bila butuh dua-arah latency-rendah (chat, kolaborasi), tambah manual.
- ❌ asynq/Redis untuk job baru — pakai River (Postgres).
- ❌ Endpoint list tanpa pagination — keyset default (§4.3).

---

## 3. Struktur Folder

```
go_starter/
├── CLAUDE.md                  # aturan main agent (lihat §7)
├── STARTER.md                  # dokumen ini
├── Makefile                   # satu-satunya pintu workflow (lihat §6)
├── .air.toml                  # config live-reload dev
├── sqlc.yaml                  # config sqlc
├── go.mod / go.sum
├── main.go                    # entry point: config, wiring, start server
├── routes.go                  # SEMUA route terdaftar di sini (single source of truth)
│
├── internal/
│   ├── config/                # load env → struct Config (typed)
│   │   └── config.go
│   ├── handler/               # HTTP handlers, satu file per fitur
│   │   ├── handler.go         # struct Handler + dependencies
│   │   ├── auth.go            # login, logout, register
│   │   ├── home.go
│   │   └── todo.go            # contoh CRUD end-to-end
│   ├── ui/                    # gomponents
│   │   ├── layout.go          # Layout(), Head(), Nav()
│   │   ├── components.go      # komponen Basecoat-style: Button, Card, Input, ...
│   │   └── pages/             # satu file per halaman
│   │       ├── home.go
│   │       ├── login.go
│   │       └── todo.go
│   ├── db/                    # HASIL GENERATE sqlc — JANGAN DIEDIT MANUAL
│   ├── session/               # accessor session typed (§4.9) + rueidis Store kustom
│   │   ├── session.go
│   │   └── store.go           # scs.Store di atas rueidis (~3 method)
│   ├── job/                   # (add-on) River args + worker
│   │   ├── email.go           # contoh: EmailWelcomeArgs + Worker
│   │   └── worker.go
│   ├── rt/                    # realtime: bus untuk push Datastar SSE
│   │   └── bus.go             # Bus interface: channel (default) / rueidis Pub/Sub (multi-instance)
│   └── mw/                    # custom middleware
│       ├── auth.go            # RequireAuth (sadar Datastar redirect)
│       ├── recover.go        # panic recovery + log terstruktur (§12)
│       └── security.go        # CSRF, security headers, rate-limit, request ID (§12)
│
├── queries/                   # file .sql untuk sqlc
│   ├── users.sql
│   └── todos.sql
│
├── migrations/                # goose, embed via embed.FS
│   ├── 00001_users.sql
│   └── 00002_todos.sql
│
├── static/                    # embed via embed.FS
│   ├── datastar.min.js        # vendored, versi + checksum tercatat (§13)
│   ├── basecoat.css           # vendored design system (root-class API)
│   ├── app.css                # OUTPUT tailwind (hasil generate, @import basecoat)
│   └── input.css              # SOURCE tailwind
│   └── VENDOR.md              # versi + sha256 tiap aset vendored
│
├── tailwindcss                # binary standalone Tailwind CLI (gitignored, di-download via make setup)
│
└── deploy/
    ├── Dockerfile             # multi-stage → distroless/scratch, <20MB
    └── docker-compose.yml     # mem_limit + cpus di level service, label Traefik
```

---

## 4. Konvensi Kode

### 4.1 Routing — semua di `routes.go`

```go
func registerRoutes(r chi.Router, h *handler.Handler, sm *scs.SessionManager) {
    // Public
    r.Get("/", h.Home)
    r.Get("/login", h.LoginPage)
    r.Post("/login", h.LoginSubmit)
    r.Post("/logout", h.Logout)

    // Protected
    r.Group(func(r chi.Router) {
        r.Use(mw.RequireAuth(sm))
        r.Get("/todos", h.TodoList)
        r.Post("/todos", h.TodoCreate)
        r.Delete("/todos/{id}", h.TodoDelete)
    })

    // Static (embedded)
    r.Handle("/static/*", http.FileServerFS(staticFS))
}
```

### 4.2 Handler — pola standar

```go
type Handler struct {
    DB    *db.Queries         // hasil sqlc
    Pool  *pgxpool.Pool
    Jobs  *river.Client[pgx.Tx] // (add-on) nil kalau job tidak dipakai
    Bus   rt.Bus              // (add-on) nil kalau realtime tidak dipakai
    Log   *slog.Logger
}

func (h *Handler) TodoList(w http.ResponseWriter, r *http.Request) {
    userID := session.UserID(r.Context())   // typed accessor, bukan key string (§4.9)
    todos, err := h.DB.ListTodos(r.Context(), db.ListTodosParams{
        UserID: userID,
        Limit:  pageSize,          // const bernama, bisa override via config
        // cursor keyset dari query param (halaman pertama = sentinel max)
    })
    if err != nil {
        h.Log.Error("list todos", "err", err)
        http.Error(w, "internal error", http.StatusInternalServerError)
        return
    }
    renderPage(w, r, h.Log, pages.TodoList(todos)) // full page (§4.4)
}

// TodoCreate — aksi Datastar: balas fragment via SSE, bukan full page.
func (h *Handler) TodoCreate(w http.ResponseWriter, r *http.Request) {
    userID := session.UserID(r.Context())
    var in struct{ Title string `json:"title"` }
    if err := datastar.ReadSignals(r, &in); err != nil {
        http.Error(w, "bad request", http.StatusBadRequest)
        return
    }
    if strings.TrimSpace(in.Title) == "" {
        patch(w, r, h.Log, ui.Alert(ui.VariantDestructive, g.Text("Judul wajib diisi")))
        return
    }
    todo, err := h.DB.CreateTodo(r.Context(), db.CreateTodoParams{UserID: userID, Title: in.Title})
    if err != nil {
        h.Log.Error("create todo", "err", err)
        http.Error(w, "internal error", http.StatusInternalServerError)
        return
    }
    patch(w, r, h.Log, pages.TodoItem(todo)) // fragment baru di-morph ke list
}
```

### 4.3 Query — sqlc

`queries/todos.sql` — **list WAJIB paginate** (keyset, bukan offset — stabil & cepat pada tabel besar):

```sql
-- name: ListTodos :many
-- Keyset pagination: halaman pertama kirim created_at=now()+id=max.
-- Halaman berikutnya kirim (created_at, id) baris terakhir yang tampil.
SELECT * FROM todos
WHERE user_id = $1
  AND (created_at, id) < ($2, $3)
ORDER BY created_at DESC, id DESC
LIMIT $4;

-- name: CreateTodo :one
INSERT INTO todos (user_id, title) VALUES ($1, $2) RETURNING *;

-- name: DeleteTodo :exec
DELETE FROM todos WHERE id = $1 AND user_id = $2;
```

> **Kenapa keyset, bukan `LIMIT/OFFSET`?** OFFSET memindai lalu membuang N baris — makin dalam makin lambat, dan bisa lompat baris saat data berubah. Keyset (`WHERE (created_at, id) < (...)`) memakai indeks langsung, O(log n) konsisten. Ini pola rujukan agent — **jangan** contohkan OFFSET. Butuh indeks: `CREATE INDEX ON todos (user_id, created_at DESC, id DESC)`.
>
> ⚠️ **Gotcha sqlc (terverifikasi spike):** row-value comparison `(created_at, id) < ($2, $3)` membuat sqlc **salah infer** tipe `cursor_id` (mengikuti `created_at` → `pgtype.Timestamptz`, padahal `id` itu `bigint`). Wajib cast eksplisit: `< (sqlc.arg(cursor_created_at)::timestamptz, sqlc.arg(cursor_id)::bigint)`. Cast memaksa `cursor_id` jadi `int64`. Halaman pertama: cursor = `(pgtype.Timestamptz{InfinityModifier: pgtype.Infinity}, math.MaxInt64)`.

Setelah edit file `.sql` apa pun → **wajib** `make check` (yang menjalankan `sqlc generate`).

### 4.4 Render — dua jalur: full page (navigasi) & patch SSE (Datastar)

**Jalur 1 — full page** untuk GET halaman biasa (first load / navigasi). Selalu bungkus Layout:

```go
// renderPage mengirim halaman penuh. Error TIDAK ditelan — di-log.
func renderPage(w http.ResponseWriter, r *http.Request, log *slog.Logger, node g.Node) {
    w.Header().Set("Content-Type", "text/html; charset=utf-8")
    if err := ui.Layout(node).Render(w); err != nil {
        // Header mungkin sudah terkirim; tak bisa ubah status. Minimal catat.
        log.Error("render page", "path", r.URL.Path, "err", err)
    }
}
```

**Jalur 2 — patch via Datastar SSE** untuk update sebagian (setelah create/delete/live). Datastar SDK men-*morph* fragment ke DOM berdasarkan `id`. Fragment = node gomponents di-render ke string:

```go
// patch mengirim satu (atau lebih) fragment gomponents ke browser via Datastar.
// Elemen dicocokkan & di-morph berdasarkan atribut id-nya.
func patch(w http.ResponseWriter, r *http.Request, log *slog.Logger, nodes ...g.Node) {
    sse := datastar.NewSSE(w, r)
    for _, n := range nodes {
        var sb strings.Builder
        if err := n.Render(&sb); err != nil {
            log.Error("patch render", "path", r.URL.Path, "err", err)
            return
        }
        if err := sse.PatchElements(sb.String()); err != nil {
            log.Error("patch send", "err", err) // klien putus, dsb.
            return
        }
    }
}
```

> **Beda dari HTMX lama:** tidak ada lagi cek `HX-Request` untuk memutuskan fragment vs full. Datastar: GET halaman → full page; aksi (`@post`/`@get` via `data-on`) → handler balas **event SSE** berisi fragment. Dua jalur eksplisit, bukan satu helper bercabang header.

### 4.5 Komponen gomponents — wrapper tipis di atas Basecoat CSS

**Strategi baru (Basecoat 1.0):** jangan port class-soup Tailwind manual. Basecoat kini file CSS dengan **root-class API** (`btn`, `card`, `field`). Wrapper gomponents cuma emit class root + variant typed. Class kecil, konsisten shadcn, dan grep-able.

Variant tetap **typed const** — typo jadi compile error (menegakkan §1.3):

```go
// internal/ui/components.go
type Variant int

const (
    VariantDefault Variant = iota
    VariantDestructive
    VariantOutline
    VariantGhost
)

// Modifier Basecoat per variant — root class "btn" datang dari basecoat.css,
// wrapper cuma menambah modifier. Array indeks enum, bukan map string:
// menambah variant tanpa kelasnya = compile error (array literal tak lengkap).
var btnVariant = [...]string{
    VariantDefault:     "",              // btn default
    VariantDestructive: "btn-destructive",
    VariantOutline:     "btn-outline",
    VariantGhost:       "btn-ghost",
}

func Button(variant Variant, children ...g.Node) g.Node {
    return h.Button(h.Class("btn "+btnVariant[variant]), g.Group(children))
}

// Pemakaian: ui.Button(ui.VariantDestructive, g.Text("Hapus"))
```

Untuk interaktivitas Datastar, atribut `data-*` pakai helper resmi `maragu.dev/gomponents-datastar`:

```go
import ds "maragu.dev/gomponents-datastar"

// Tombol yang POST ke /todos saat klik, kirim signal $title:
h.Button(
    h.Class("btn"),
    ds.On("click", "@post('/todos')"),
    g.Text("Tambah"),
)
```

Komponen inti minimum (Fase 3) — **mulai 5 dulu, buktikan pola wrapper**:
`Button`, `Card`, `Input`, `Label`, `Alert`. (Karena Basecoat sudah sediakan CSS-nya, menambah komponen = wrapper tipis, bukan proyek besar.)

Komponen lanjutan (Fase 8): `Select`, `Checkbox`, `Table`, `Badge`, `Dialog` (`<dialog>` native), `Dropdown` (`<details>` native), `Tabs`, `FormField`, `Spinner`, `Toast`.

### 4.6 Middleware auth — sadar Datastar

Untuk request Datastar (aksi SSE), redirect via event `sse.Redirect`, bukan HTTP 303. Deteksi lewat header `Datastar-Request`:

```go
func RequireAuth(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        if session.UserID(r.Context()) == 0 { // typed accessor (§4.9)
            if r.Header.Get("Datastar-Request") == "true" {
                sse := datastar.NewSSE(w, r)
                _ = sse.Redirect("/login") // browser pindah lewat SSE
                return
            }
            http.Redirect(w, r, "/login", http.StatusSeeOther)
            return
        }
        next.ServeHTTP(w, r)
    })
}
```

### 4.7 Embed — semua masuk binary

```go
//go:embed static
var staticFS embed.FS

//go:embed migrations/*.sql
var migrationsFS embed.FS

// Saat startup:
goose.SetBaseFS(migrationsFS)
if err := goose.SetDialect("postgres"); err != nil {
    log.Fatal("goose dialect", "err", err)
}
if err := goose.Up(sqlDB, "migrations"); err != nil {
    log.Fatal("goose up", "err", err)
}
```

**Aman multi-instance dengan Postgres advisory lock** — bungkus migrate agar hanya satu instance yang jalan, sisanya menunggu:

```go
func migrateWithLock(ctx context.Context, pool *pgxpool.Pool, sqlDB *sql.DB) error {
    conn, err := pool.Acquire(ctx)
    if err != nil { return fmt.Errorf("acquire conn: %w", err) }
    defer conn.Release()

    // Lock global (angka arbitrer tetap). Instance lain BLOCK di sini sampai lock lepas.
    if _, err := conn.Exec(ctx, "SELECT pg_advisory_lock(4242)"); err != nil {
        return fmt.Errorf("advisory lock: %w", err)
    }
    defer conn.Exec(ctx, "SELECT pg_advisory_unlock(4242)")

    goose.SetBaseFS(migrationsFS)
    if err := goose.SetDialect("postgres"); err != nil { return err }
    return goose.Up(sqlDB, "migrations")
}
```

> ⚠️ **Urutan startup penting** karena ada **dua sumber migrasi**: goose (skema aplikasi) dan River (tabel job-nya sendiri). Jalankan berurutan di dalam lock yang sama:
> 1. `pg_advisory_lock` →
> 2. `goose.Up` (skema app) →
> 3. `rivermigrate.Migrate` (kalau add-on job aktif) →
> 4. `pg_advisory_unlock`.
>
> Flag `AUTO_MIGRATE=false` tetap disediakan untuk memindah migrate ke langkah pre-deploy terpisah bila diinginkan. Default **on** (aman berkat advisory lock).

### 4.8 Config — env → struct typed

```go
type Config struct {
    Port        string // PORT, default "8080"
    DatabaseURL string // DATABASE_URL (wajib)
    RedisAddr   string // REDIS_ADDR (wajib)
    Env         string // ENV: "dev" | "production"
    AutoMigrate bool   // AUTO_MIGRATE, default true (§4.7)
    SessionKey  string // SESSION_KEY untuk scs (wajib di production)
}

// MustLoad — fail-fast: env wajib yang kosong = panic saat startup,
// bukan error senyap di request pertama.
func MustLoad() *Config {
    c := &Config{
        Port:        getEnv("PORT", "8080"),
        DatabaseURL: mustEnv("DATABASE_URL"),
        RedisAddr:   mustEnv("REDIS_ADDR"),
        Env:         getEnv("ENV", "dev"),
        AutoMigrate: getEnv("AUTO_MIGRATE", "true") == "true",
    }
    if c.Env == "production" {
        c.SessionKey = mustEnv("SESSION_KEY")
    }
    return c
}
```

Semua env dibaca **hanya** di `internal/config`, tidak tersebar. **Manual `os.Getenv` + fail-fast**, bukan viper/envconfig — nol dependency, paling agent-friendly.

### 4.9 Session — accessor typed, key muncul sekali

Key session string mentah (`"userID"`) dilarang tersebar. Bungkus di satu file — string-nya muncul **tepat sekali**, typo jadi mustahil:

```go
// internal/session/session.go
package session

const keyUserID = "userID" // satu-satunya tempat string ini ada

var mgr *scs.SessionManager // di-inject saat startup

func Init(m *scs.SessionManager) { mgr = m }

// UserID mengembalikan 0 jika belum login.
func UserID(ctx context.Context) int64      { return mgr.GetInt64(ctx, keyUserID) }
func SetUserID(ctx context.Context, id int64) { mgr.Put(ctx, keyUserID, id) }
func Clear(ctx context.Context) error        { return mgr.Destroy(ctx) }
```

Handler & middleware memanggil `session.UserID(ctx)` / `session.SetUserID(ctx, id)` — tidak pernah menyentuh string key langsung.

**Store rueidis kustom** (`internal/session/store.go`): scs butuh `scs.Store` interface (3 method: `Find`, `Commit`, `Delete`). Store resmi `redisstore` menyeret klien lain (redigo) — untuk menjaga **satu klien Redis (rueidis)**, tulis Store tipis di atas rueidis:

```go
// Store mengimplementasikan scs.Store di atas rueidis — ~30 baris, satu klien Redis.
type Store struct {
    client rueidis.Client
    prefix string // "scs:"
}

func (s *Store) Find(token string) ([]byte, bool, error) { /* GET */ }
func (s *Store) Commit(token string, b []byte, expiry time.Time) error { /* SET PX */ }
func (s *Store) Delete(token string) error { /* DEL */ }
```

> Ini pengecualian sah dari "jangan tulis sendiri yang sudah ada lib-nya": alternatifnya = dua klien Redis di satu binary (bentrok prinsip minimal-deps). Store 3-method jauh lebih murah daripada dependency ganda.

> 🐞 **GOTCHA KRITIS (terverifikasi spike): scs + Datastar SSE → cookie tak terkirim.**
> `scs.LoadAndSave` menyisipkan `Set-Cookie` lewat pembungkus `ResponseWriter` yang commit saat `Write`/`WriteHeader` pertama. Tapi `datastar.NewSSE()` langsung `Flush()` header via `http.ResponseController`, yang meng-`Unwrap()` pembungkus scs → header 200 terkirim **sebelum** scs sempat menulis cookie. Akibat: login "sukses" tapi user tetap dianggap belum login (session di store, cookie tak pernah sampai browser). **Test unit lolos** (body benar) — hanya ketahuan saat run nyata.
>
> **Fix wajib:** commit + tulis cookie **manual SEBELUM `NewSSE`**, di setiap handler yang mengubah session lalu membalas via Datastar (login, register, logout):
> ```go
> // internal/session/session.go
> func WriteCookie(ctx context.Context, w http.ResponseWriter) error {
>     token, expiry, err := mgr.Commit(ctx) // commit ke store
>     if err != nil { return err }
>     mgr.WriteSessionCookie(ctx, w, token, expiry) // tulis Set-Cookie manual
>     return nil
> }
> // handler: startSession(...) → session.WriteCookie(ctx, w) → datastar.NewSSE(w, r)
> ```
> Test WAJIB assert `rec.Header().Get("Set-Cookie")` memuat `session=` — bukan cuma cek body.

---

## 5. Background Job — River (add-on, Postgres-based)

> Job queue bersifat **opsional**. Jangan pasang kalau project belum butuh async.
> Ganti dari asynq: River berbasis **Postgres** (bukan Redis), v1.x stabil, dan job masuk transaksi yang sama dengan data — tidak ada dependency antrean terpisah.

River memakai **struct args typed** — nama job & payload dicek compile-time, bukan string + JSON mentah:

```go
// internal/job/email.go
type EmailWelcomeArgs struct {
    UserID int64 `json:"user_id"`
}

// Kind = identitas job. Konstanta di satu tempat, bukan string tersebar.
func (EmailWelcomeArgs) Kind() string { return "email_welcome" }

// Worker mengimplementasikan pemrosesan. River mencocokkan args↔worker via tipe.
type EmailWelcomeWorker struct {
    river.WorkerDefaults[EmailWelcomeArgs]
    DB  *db.Queries
    Log *slog.Logger
}

func (w *EmailWelcomeWorker) Work(ctx context.Context, job *river.Job[EmailWelcomeArgs]) error {
    user, err := w.DB.GetUser(ctx, job.Args.UserID)
    if err != nil {
        return fmt.Errorf("get user %d: %w", job.Args.UserID, err) // context saat re-throw
    }
    // ... kirim email
    _ = user
    return nil
}
```

Enqueue dari handler — masuk transaksi Postgres yang sama dengan data:

```go
_, err := riverClient.Insert(ctx, job.EmailWelcomeArgs{UserID: user.ID}, nil)
```

Worker berjalan sebagai goroutine di binary yang sama (mode default), atau proses terpisah dengan flag `-worker` bila beban tinggi. Satu binary, dua mode.

---

## 6. Makefile — satu-satunya pintu workflow

```makefile
.PHONY: setup dev check build css migrate-new run clean

BINARY := app

## setup: download tools (sekali saja)
setup:
	go install github.com/air-verse/air@latest
	go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest
	go install github.com/pressly/goose/v3/cmd/goose@latest
	# download tailwind standalone CLI sesuai OS/arch ke ./tailwindcss
	# download + verifikasi checksum datastar.min.js & basecoat.css (lihat VENDOR.md)

## css: generate Tailwind output (input.css @import basecoat.css)
css:
	./tailwindcss -i static/input.css -o static/app.css --minify

## vendor-verify: cek sha256 aset vendored vs VENDOR.md
vendor-verify:
	# bandingkan checksum static/datastar.min.js & static/basecoat.css

## check: WAJIB dijalankan agent setelah setiap perubahan, sampai hijau
check:
	sqlc generate
	go vet ./...
	go build -o /tmp/$(BINARY)-check .
	go test ./...

## dev: live reload (air: pre-cmd css, lalu build + run — TIDAK test di loop)
dev:
	air

## build: production single binary
build: css
	sqlc generate
	CGO_ENABLED=0 go build -trimpath -buildvcs=false -ldflags="-s -w" -o $(BINARY) .

## build-obfuscated: production + garble (verifikasi kompat Go 1.26 dulu)
build-obfuscated: css
	sqlc generate
	CGO_ENABLED=0 garble -tiny -literals build -o $(BINARY) .

## migrate-new: buat file migration baru (make migrate-new name=add_users)
migrate-new:
	goose -dir migrations create $(name) sql

run: build
	./$(BINARY)
```

> **Catatan `air`:** jangan jalankan `go test` di loop reload (lambat). `.air.toml` cukup: pre-cmd `make css` → `go build` → run. `make check` (termasuk test) dijalankan manual/agent setelah perubahan.

---

## 7. CLAUDE.md — isi wajib

```markdown
# Aturan Project

## Workflow
- Setelah SETIAP perubahan kode, jalankan `make check` dan pastikan hijau sebelum lanjut.
- Setelah mengedit file di `queries/*.sql` atau `migrations/*.sql`, `make check` wajib (menjalankan sqlc generate).
- Setelah mengubah class Tailwind di file Go, jalankan `make css`.

## Peta Kode
- Semua route ada di `routes.go` — JANGAN daftarkan route di tempat lain.
- Handler di `internal/handler/`, satu file per fitur.
- View/komponen di `internal/ui/` (gomponents). Tiru pola wrapper Basecoat yang sudah ada.
- Query SQL di `queries/`. File `internal/db/` adalah HASIL GENERATE — jangan edit manual.
- Migration di `migrations/` via goose.

## Larangan
- Jangan pakai ORM, html/template, React, atau CDN.
- Jangan pakai HTMX atau Alpine.js — interaktivitas HANYA Datastar.
- Jangan tambah dependency baru tanpa konfirmasi. Jangan pakai dua klien Redis (hanya rueidis).
- Jangan pakai `deploy.resources` di compose — pakai `mem_limit`/`cpus` di level service.
- Jangan bikin endpoint list tanpa pagination (keyset).

## Pola Penting
- Dua jalur render: `renderPage()` (full page GET) vs `patch()` (fragment via Datastar SSE). Lihat §4.4.
- Interaktivitas: atribut `data-*` via `maragu.dev/gomponents-datastar`; aksi `@get`/`@post`.
- Middleware auth sadar Datastar (`sse.Redirect` untuk request Datastar).
- **Authz, bukan cuma authn:** tiap query yang menyentuh data milik user WAJIB filter `user_id` (ownership). Jangan andalkan hanya login. Contoh: `DeleteTodo WHERE id=$1 AND user_id=$2`.
- Password: argon2id (bukan bcrypt). Session: typed accessor (§4.9).
- Env hanya dibaca di `internal/config` (fail-fast `MustLoad`).
- Setiap handler di belakang middleware recover + CSRF (§11).
```

---

## 8. Deploy

### Dockerfile (multi-stage, <20MB)

```dockerfile
FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN ./tailwindcss -i static/input.css -o static/app.css --minify
# -trimpath: buang path absolut (reproducible). -buildvcs=false: jangan sisipkan info VCS.
RUN CGO_ENABLED=0 go build -trimpath -buildvcs=false -ldflags="-s -w" -o /app .

# nonroot: binary jalan sebagai UID non-root (65532). debian13: base terbaru.
FROM gcr.io/distroless/static-debian13:nonroot
COPY --from=build /app /app
EXPOSE 8080
# GOMEMLIMIT diset via env di compose (Go 1.26 belum baca cgroup mem otomatis).
ENTRYPOINT ["/app"]
```

> **Healthcheck di distroless (tanpa shell):** jangan pakai `CMD curl` (tak ada curl/shell). Pakai healthcheck via binary sendiri (`/app -healthcheck` yang hit `/readyz` lalu exit), atau andalkan Traefik/compose TCP check.

### docker-compose.yml (pola standar server)

```yaml
services:
  app:
    image: ghcr.io/<user>/go_starter:latest
    restart: unless-stopped
    mem_limit: 128m
    cpus: 0.5
    logging:
      driver: json-file
      options:
        max-size: "10m"
        max-file: "3"
    environment:
      - DATABASE_URL=${DATABASE_URL}
      - REDIS_ADDR=${REDIS_ADDR}
      - SESSION_KEY=${SESSION_KEY}
      - ENV=production
      - GOMEMLIMIT=115MiB          # ~90% dari mem_limit 128m; beri GC batas soft
      - GOMAXPROCS=1               # selaras cpus: 0.5 (opsional, 1.26 sudah cgroup-aware)
    networks:
      - traefik
    labels:
      - traefik.enable=true
      - traefik.http.routers.app.rule=Host(`app.wibudev.com`)
      - traefik.http.routers.app.entrypoints=websecure
      - traefik.http.routers.app.tls.certresolver=letsencrypt
      - traefik.http.services.app.loadbalancer.server.port=8080

networks:
  traefik:
    external: true
```

CI/CD: GitHub Actions build image → push GHCR → webhook Portainer redeploy (pola existing).

### Target Performa

| Metrik | Target | Catatan |
|--------|--------|---------|
| Ukuran image | < 20 MB | distroless/static + binary `-s -w` |
| RAM idle | < 30 MB | **aspiratif** — validasi empiris Fase 10; naik dgn River+SSE aktif. Set `GOMEMLIMIT` |
| Cold start | < 100 ms | tanpa migration berat saat start |
| TTFB halaman render | < 10 ms | lokal, tanpa query berat |
| Payload JS ke browser | **~ 12 KB** (Datastar minified) | turun dari ~30KB (HTMX+Alpine) — satu file |

> Angka RAM/cold-start adalah **target aspiratif**, bukan kontrak deploy. Ukur dengan add-on aktif sebelum diklaim. `GOMEMLIMIT` di-set manual (~`mem_limit` × 0.9) karena Go 1.26 belum auto-memlimit dari cgroup (baru GOMAXPROCS yang cgroup-aware).

---

## 9. Roadmap Pembangunan Template

Urutan pengerjaan (tiap fase harus `make check` hijau sebelum lanjut):

1. **Fondasi** — `go mod init`, config `MustLoad` fail-fast, slog JSON, Chi + middleware inti (recover, request ID, security headers, CSRF), `routes.go`, `/healthz` (liveness) + `/readyz` (readiness), embed static + cache-busting, Makefile, `.air.toml`.
2. **Database** — koneksi pgxpool, goose auto-migrate + advisory lock (§4.7), sqlc setup, migration + query pertama (`users`).
3. **UI dasar** — Tailwind standalone + Basecoat CSS vendored, Datastar vendored, `Layout()`, 5 wrapper komponen (Button [enum], Card, Input, Label, Alert).
4. **Auth** — register/login/logout, **argon2id**, scs + rueidis Store kustom (§4.9), `session` typed, `RequireAuth`, halaman login, CSRF aktif. **+ test:** `httptest` login (200 sukses, 400 email kosong, 303 redirect unauth) + verifikasi CSRF menolak cross-origin.
5. **Contoh CRUD end-to-end** — fitur `todos`: migration (+ indeks keyset) → query sqlc paginated → handler (renderPage + patch Datastar) → page gomponents → interaksi Datastar (`data-on`, `@post`, morph). **Pola rujukan agent — harus lengkap: pagination, error handling, authz ownership.** **+ test:** query sqlc (`TEST_DATABASE_URL`) + handler test.
6. **Realtime** — bus in-process, `/events` stream Datastar, satu contoh push (§10). Bus rueidis Pub/Sub bila multi-instance.
7. **(Add-on) Job** — River client + worker + migration River, satu contoh task. Skip kalau belum butuh async.
8. **Komponen lanjutan** — Dialog, Dropdown, Table, Tabs, Toast, FormField (wrapper Basecoat), **hanya setelah Fase 5 solid**.
9. **Deploy** — Dockerfile (nonroot, trimpath, GOMEMLIMIT), compose, GitHub Actions, tes end-to-end di server.
10. **Finalisasi** — `CLAUDE.md` matang, README, `VENDOR.md`, validasi performa empiris, tag `v1.0`.

> **Test bukan fase terpisah** — tiap fitur (mulai Fase 4) menyertakan test dalam pekerjaan yang sama. `make check` menjalankan `go test ./...` dan wajib hijau sebelum lanjut fase.

---

## 10. Realtime — Datastar SSE

Datastar menjadikan SSE mekanisme utama (bukan ekstensi tempelan). Dua pola: **aksi** (request→response SSE, §4.4 jalur 2) dan **stream panjang** (koneksi terbuka, server dorong kapan saja).

> **Kenapa SSE, bukan WebSocket?** Kebutuhan realtime template ini satu-arah (server→browser): notif, live list, progress, dashboard. SSE cukup, reconnect otomatis (`Last-Event-ID`), dan Datastar sudah menanganinya native. WS hanya bila butuh dua-arah latency-rendah (chat ketik-langsung, kolaborasi kursor) → add-on `coder/websocket`, jangan bebani core.

### 10.1 Bus swappable — in-process → rueidis Pub/Sub

```go
// internal/rt/bus.go — interface, implementasi ditukar tanpa ubah handler
type Bus interface {
    Publish(topic string, node g.Node)              // fragment gomponents
    Subscribe(topic string) (<-chan g.Node, func()) // channel + cancel
}
```

- **Default (single-instance):** `channelBus` — `map[topic][]chan`, in-process. Nol infra.
- **Multi-instance:** `redisBus` — **rueidis** Pub/Sub (klien Redis yang sama dengan session). Instance A publish → instance B yang pegang koneksi user ikut mendorong. Interface identik.

### 10.2 Stream handler — Datastar SDK

```go
// Handler stream — protected, di routes.go dalam grup RequireAuth.
// Datastar SDK urus header, flush, dan format event SSE.
func (h *Handler) Events(w http.ResponseWriter, r *http.Request) {
    sse := datastar.NewSSE(w, r) // set header text/event-stream + flush otomatis

    topic := fmt.Sprintf("user:%d", session.UserID(r.Context()))
    ch, cancel := h.Bus.Subscribe(topic)
    defer cancel() // tutup subscription saat klien putus — cegah goroutine bocor

    for {
        select {
        case <-r.Context().Done():
            return // klien putus / server shutdown
        case node := <-ch:
            var sb strings.Builder
            if err := node.Render(&sb); err != nil {
                h.Log.Error("rt render", "err", err)
                continue
            }
            if err := sse.PatchElements(sb.String()); err != nil {
                return // koneksi mati; keluar, defer cancel() jalan
            }
        }
    }
}
```

Publish dari mana saja (handler lain, atau worker River saat job selesai):

```go
h.Bus.Publish(fmt.Sprintf("user:%d", userID), pages.NotifItem(notif))
```

### 10.3 Jebakan wajib diingat

> ⚠️ **SSE butuh HTTP/2.** Di HTTP/1.1 browser batasi ~6 koneksi/domain — tab ke-7 hang. Di belakang Traefik+TLS, HTTP/2 otomatis. Dev lokal tanpa TLS: sadari batas ini (Datastar tetap jalan, cuma batas koneksi berlaku).
> ⚠️ **Setiap koneksi = 1 goroutine + 1 koneksi terbuka.** `defer cancel()` + `r.Context().Done()` wajib menutup subscription saat klien putus — kalau bocor, goroutine menumpuk.
> ⚠️ **Flush** ditangani Datastar SDK — tidak perlu `http.Flusher` manual lagi (SDK pakai `http.ResponseController` internal).

Sisi browser (Datastar) — buka stream saat halaman load:

```html
<div data-on-load="@get('/events')"></div>
<!-- Fragment yang dikirim server di-morph ke DOM berdasarkan id-nya. -->
```

---

## 11. Security & Observability — inti (90% dari stdlib)

Lapisan ini **core**, bukan add-on. Hampir semua dari stdlib Go 1.26 — nol/minim dependency. Middleware dirangkai di `routes.go` sebelum handler.

### 11.1 Middleware wajib (urutan penting)

```go
r.Use(mw.RequestID)        // ID unik per request → masuk semua log
r.Use(mw.Recover(log))     // panic → 500 + log stack, koneksi tidak mati senyap
r.Use(mw.SecurityHeaders)  // X-Content-Type-Options, Referrer-Policy, dst.
r.Use(csrf.Handler)        // http.CrossOriginProtection (stdlib, Go 1.25.1+)
r.Use(mw.RateLimit)        // golang.org/x/time/rate atau httprate (per-IP)
```

**CSRF — `http.CrossOriginProtection` (stdlib):**

```go
var csrf = http.NewCrossOriginProtection() // Go 1.25.1+
// Otomatis izinkan same-origin via Sec-Fetch-Site; tolak cross-origin non-aman.
```

> ⚠️ **Dev tanpa TLS:** `Sec-Fetch-Site` hanya dikirim di origin "trustworthy" (HTTPS/localhost). Di HTTP non-localhost, proteksi jatuh ke cek `Origin`/`Host` — pastikan `Host` benar. Wajib **Go ≥1.25.1** (ada bypass fix di 1.25.1).

**Panic recovery** — satu panic tanpa ini = koneksi mati tanpa jejak:

```go
func Recover(log *slog.Logger) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            defer func() {
                if v := recover(); v != nil {
                    log.Error("panic", "err", v, "path", r.URL.Path, "stack", string(debug.Stack()))
                    http.Error(w, "internal error", http.StatusInternalServerError)
                }
            }()
            next.ServeHTTP(w, r)
        })
    }
}
```

### 11.2 Health: liveness vs readiness (beda!)

```go
r.Get("/healthz", h.Liveness)  // proses hidup — selalu 200 kalau server jalan
r.Get("/readyz", h.Readiness)  // DB + Redis reachable — 503 kalau belum siap
```

> Traefik/orchestrator route ke `/readyz`, bukan `/healthz`. Saat startup (migration jalan) atau DB putus, `/readyz` balas 503 → tidak ada trafik masuk ke instance belum siap. Krusial untuk auto-migrate + rolling deploy.

### 11.3 Observability — cukup, tidak overkill

- **Logging:** `slog` JSON handler di production, text di dev. Request ID di tiap log line. **Jangan log PII/credential** (aturan global #12).
- **pprof:** daftar `net/http/pprof` di router terpisah/port internal (jangan expose publik).
- **Metrics:** `expvar` (stdlib) cukup untuk starter dipakai sendiri. **OpenTelemetry & Prometheus = overkill** — tambah hanya bila benar butuh.

### 11.4 Graceful shutdown — drain SSE + worker

```go
// Tangkap SIGTERM → stop terima koneksi baru → tunggu in-flight (termasuk SSE) → stop River.
srv.Shutdown(ctx)          // net/http drain
riverClient.Stop(ctx)      // (add-on) selesaikan job berjalan
```

> Koneksi SSE panjang: `Shutdown` menunggu handler selesai. Pastikan loop `Events` (§10.2) merespons `r.Context().Done()` agar tidak menahan shutdown.

### 11.5 Aset statis: cache-busting + upload

- **Cache-busting:** aset di-embed disajikan dengan **content-hash di path** (`/static/app.<sha8>.css`) + `Cache-Control: immutable, max-age=31536000`. Tanpa ini, setelah `make css` + redeploy, browser pakai CSS lama. Hash dihitung saat build/startup, di-inject ke `Layout()`.
- **Upload ke MinIO (add-on):** `http.MaxBytesReader` batasi ukuran (jaga RAM <target), **stream** ke MinIO (jangan buffer penuh di memori), validasi content-type dari magic bytes (bukan cuma ekstensi).

---

## 12. Testing

Test **menyertai fitur**, bukan fase terpisah (selaras aturan global). `make check` menjalankan `go test ./...` dan wajib hijau.

- **Handler** — `net/http/httptest`. Assert status + potongan HTML. Contoh: login 200 sukses, 400 email kosong, 303 redirect saat unauth, CSRF tolak cross-origin.
- **Query sqlc** — pakai `TEST_DATABASE_URL` (Postgres test, **bukan** dev/prod, **bukan** SQLite — jangan tukar engine). Migrate → insert → assert.
- **Komponen UI** — render node ke `bytes.Buffer`, assert class/atribut penting muncul. Enum variant bikin ini deterministik.

**Strategi DB test — template database (default):** buat satu DB ter-migrate sekali, tiap test `CREATE DATABASE ... TEMPLATE ...` (cepat, terisolasi). Ini default — ringan, nol dependency container.

```go
func TestLoginSubmit_MissingEmail(t *testing.T) {
    req := httptest.NewRequest("POST", "/login", strings.NewReader("email=&password=x"))
    req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
    rec := httptest.NewRecorder()
    h.LoginSubmit(rec, req)
    if rec.Code != http.StatusBadRequest {
        t.Fatalf("want 400, got %d", rec.Code)
    }
}
```

> **testcontainers-go = add-on CI opsional, bukan wajib.** Untuk CI hermetik (tanpa Postgres eksternal), testcontainers spin Postgres di Docker. Tapi overhead-nya nyata (~detik/startup) — untuk dev lokal, `TEST_DATABASE_URL` ke Postgres nyata lebih cepat. Jangan jadikan dependency wajib.

---

## 13. Vendoring aset — versi + integritas

Prinsip "no CDN" butuh mekanisme, bukan cuma niat. Tiap aset di `static/` dicatat di `static/VENDOR.md`:

```markdown
| Aset            | Versi   | SHA-256      | Sumber                          |
|-----------------|---------|--------------|---------------------------------|
| datastar.min.js | v1.0.2  | `a1b2…`      | https://.../datastar@1.0.2      |
| basecoat.css    | v1.0.1  | `c3d4…`      | https://basecoatui.com/...      |
```

`make setup` download + **verifikasi checksum** sebelum menaruh ke `static/`. Upgrade = ganti versi + checksum di `VENDOR.md`, jalankan `make vendor-verify`. Mismatch = STOP (kemungkinan file dirusak/salah versi).

---

## 14. Referensi

- gomponents (v1.0.0): https://www.gomponents.com
- gomponents-datastar (jembatan resmi): https://pkg.go.dev/maragu.dev/gomponents-datastar
- Datastar: https://data-star.dev
- Datastar Go SDK: https://github.com/starfederation/datastar-go
- Basecoat UI (v1.0): https://basecoatui.com
- sqlc: https://sqlc.dev
- pgx/v5: https://github.com/jackc/pgx
- Chi: https://go-chi.io
- goose: https://github.com/pressly/goose
- scs: https://github.com/alexedwards/scs
- rueidis: https://github.com/redis/rueidis
- River (job, Postgres): https://riverqueue.com
- Tailwind standalone: https://tailwindcss.com/blog/standalone-cli
- OWASP Password Storage: https://cheatsheetseries.owasp.org/cheatsheets/Password_Storage_Cheat_Sheet.html
