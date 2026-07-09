# go_stater

Starter web full-stack Go: **satu bahasa, satu binary, satu perintah build**.
Cepat, ringan, modern (2026), agent-friendly. Output = single binary (static
assets, migrations, dan template di-embed via `embed.FS`).

> Runtime tetap butuh **PostgreSQL** + **Redis** — "nol artefak tambahan", bukan
> "nol infra". Lihat [`STATER.md`](STATER.md) untuk spesifikasi & alasan arsitektur.

## Stack

| Lapis | Teknologi |
|-------|-----------|
| Router | `net/http` + [chi](https://github.com/go-chi/chi) |
| View | [gomponents](https://maragu.dev/gomponents) (HTML sebagai fungsi Go) |
| Interaktivitas | [Datastar](https://data-star.dev) (hypermedia, SSE-native) |
| CSS | Tailwind v4 (standalone CLI, no-Node) + [daisyUI](https://daisyui.com) (plugin, zero-JS komponen, multi-tema light/dark) |
| Chart | [ECharts](https://echarts.apache.org) vendored (panel aktivitas, init CSP-safe eksternal) |
| DB | PostgreSQL via [pgx](https://github.com/jackc/pgx) + [sqlc](https://sqlc.dev) (type-safe) |
| Migrasi | [goose](https://github.com/pressly/goose) (advisory-lock, aman multi-instance) |
| Session | [scs](https://github.com/alexedwards/scs) + [rueidis](https://github.com/redis/rueidis) (Redis store) |
| Auth | Password (argon2id, dev-only) + Google OAuth/OIDC |
| Otorisasi | [Casbin](https://casbin.org) RBAC (user < admin < super_admin) |

## Prasyarat

- **Go 1.26+**
- **PostgreSQL** & **Redis** berjalan
- macOS/Linux (Makefile mendeteksi OS/arch untuk unduh Tailwind CLI)

## Mulai cepat

```bash
# 1. Install tooling (sqlc, goose, air) + unduh aset vendored (sekali saja)
make setup

# 2. Salin konfigurasi, sesuaikan DATABASE_URL / REDIS_ADDR
cp .env.example .env

# 3. Buat database dev & test
createdb go_stater && createdb go_stater_test

# 4. Jalankan (live-reload; migrasi auto saat boot)
make dev
```

Buka <http://localhost:8080>. Aplikasi memigrasi DB otomatis (`AUTO_MIGRATE=true`).

## Konfigurasi (`.env`)

| Variabel | Wajib | Keterangan |
|----------|-------|------------|
| `DATABASE_URL` | ✓ | DSN Postgres |
| `REDIS_ADDR` | ✓ | alamat Redis (session store) |
| `TEST_DATABASE_URL` | untuk test | DB terpisah untuk `make test` |
| `PORT` | — | default `8080` |
| `ENV` | — | `dev` (default) \| `production` |
| `AUTO_MIGRATE` | — | default `true` |
| `SESSION_KEY` | prod | kunci sesi (wajib di production) |
| `GOOGLE_CLIENT_ID` / `_SECRET` / `_REDIRECT_URL` | prod | OAuth Google (wajib di production) |
| `SUPER_ADMIN_EMAILS` | — | email super-admin "root", dipisah koma |
| `APP_TIMEZONE` | — | zona waktu (IANA) agregasi panel aktivitas, default `Asia/Jakarta` |

## Autentikasi & role

- **Google OAuth** = jalur login utama. **Password** hanya di dev (`ENV != production`)
  untuk mempermudah agen/dev masuk.
- Role hierarkis **user < admin < super_admin**. Super-admin "root" ditentukan
  `SUPER_ADMIN_EMAILS` (immutable — tak bisa diturunkan/diblokir lewat panel).
- Setelah login, user diarahkan ke home per-role: super_admin→`/dev`,
  admin→`/admin`, user→`/user`. Landing `/` dapat diakses semua.

## Panel

| Rute | Akses | Isi |
|------|-------|-----|
| `/dev/users` | super_admin | kelola role/status/hapus user + audit trail |
| `/dev/logs` | super_admin | aktivitas user (presence "aktif jam berapa"), KPI + chart (harian/mingguan/bulanan) + event login/logout |
| `/dev/health` | super_admin, **dev-only** | scan kesehatan file `.go` (baris/karakter vs ambang) |
| `/dev/erd` | super_admin, **dev-only** | diagram ERD dari katalog live Postgres (Mermaid) |
| `/admin` | admin+ | dashboard admin (stub) |
| `/user` | user+ | beranda user (stub) |

Panel dev-only (`/dev/health`, `/dev/erd`) tak terdaftar di production karena
butuh source/tooling yang tak ada di build single-binary.

## Perintah (Makefile)

| Perintah | Aksi |
|----------|------|
| `make setup` | install tooling + unduh aset vendored |
| `make dev` | live-reload (air): regenerate CSS → sqlc → build → run |
| `make check` | **gerbang wajib**: sqlc · vet · gofmt · build · test |
| `make test` | jalankan test (butuh `TEST_DATABASE_URL`) |
| `make build` | single binary (`./app`) — regenerate CSS + sqlc dulu |
| `make run` | build lalu jalankan |
| `make css` | generate `static/app.css` dari class di file `.go` |
| `make migrate-new name=x` | buat file migrasi goose baru |

## Struktur

```
main.go            # entry point: config, wiring, graceful shutdown
routes.go          # SEMUA route (single source of truth)
migrations/        # goose *.sql (di-embed)
queries/           # sumber sqlc *.sql
internal/
  config/          # env → struct typed (fail-fast)
  db/              # GENERATED sqlc — jangan edit tangan
  database/        # migrasi (advisory lock)
  session/         # scs + rueidis, accessor typed
  auth/            # argon2id
  oauth/           # Google OIDC flow (PKCE, nonce, verify)
  authz/           # Casbin RBAC (model.conf + policy.csv embed) + guard
  mw/              # middleware (request-id, recover, log, security, auth, authz)
  handler/         # HTTP handler, satu file per fitur
  ui/              # gomponents: layout, komponen, halaman, helper Datastar bertipe (dsx)
  activity/        # agregasi presence + option ECharts (panel /dev/logs)
  health/          # scanner file-health (dev)
  erd/             # introspeksi katalog → teks Mermaid (dev)
  assets/          # cache-busting aset ber-hash
static/            # aset vendored + generated (di-embed)
```

## Testing

```bash
createdb go_stater_test
make test    # atau: make check (test + lint + build)
```

Test yang butuh DB memakai `TEST_DATABASE_URL` dan di-`skip` bila kosong.

## Deploy

`make build` menghasilkan satu binary `./app` (assets & migrasi ter-embed).
Set `ENV=production` + variabel wajib (lihat tabel konfigurasi). Binary butuh
Postgres + Redis yang dapat dijangkau. Lihat [`CHANGELOG.md`](CHANGELOG.md).
