# Changelog

Semua perubahan penting pada go_stater dicatat di sini.

## [Unreleased]

### Added
- **Redirect per-role + panel `/admin` & `/user`:** setelah login user diarahkan ke home sesuai role (super_admin→`/dev`, admin→`/admin`, user→`/user`) via `authz.HomePathFor` (sumber tunggal). Landing `/` mengalihkan user yang sudah login ke home mereka. `/admin` & `/user` = AppShell stub, terproteksi Casbin (`admin:home`, `user:home`).
- **RBAC (Casbin) + panel `/dev`:** role hierarkis `user < admin < super_admin` via Casbin (subject=role, policy embed in-memory). Super-admin "sejati" dari `SUPER_ADMIN_EMAILS` (root immutable, kebal demote/block/delete). Panel `/dev/users`: kelola role, status (active/disabled/blocked), soft-delete — semua di-guard di service layer + audit trail (`audit_logs`). Enforcement 3 lapis: route (`RequireEnforce`) + UI (`When`) + service (`Guard*`). Keputusan arsitektur di `docs/decisions/0001`.
- **Avatar Google:** claim `picture` disimpan (`users.avatar_url`, di-refresh tiap login), tampil di nav + panel dengan fallback inisial lokal (`referrerpolicy=no-referrer`, CSP `img-src` +googleusercontent).
- **AppShell** (`/dev`): layout sidebar + header responsif (drawer mobile via Datastar signal), navigasi full-page, active-link server-side.
- Ikon: logo "super G" 4-warna resmi Google di tombol login (patuh pedoman merek, test regresi kepatuhan warna). Library ikon umum `gomponents-lucide` (zero-dep, tree-shaken linker, selaras Basecoat) — dipakai di todos (Plus/Trash2).
- **Login with Google (OAuth 2.0 / OIDC):** `golang.org/x/oauth2` + `coreos/go-oidc/v3`, dengan state (anti-CSRF), PKCE S256, nonce (anti-replay id_token), dan verifikasi `email_verified`. Identitas disimpan di tabel `oauth_accounts` (tautan via OIDC `sub`, bukan email). Auto-link ke akun email yang sudah ada. Padanan idiomatik Go untuk Better Auth — komposisi library, bukan framework.
- Password auth kini **dev-only**: `POST /login`, `/register` hanya terdaftar saat `ENV != production`; di produksi hanya tombol Google. `GET /login` adaptif (form password muncul hanya di dev).
- Landing page publik di `/` (tidak lagi redirect ke `/login`); CTA menyesuaikan status login. Route protected pindah ke `/todos`.
- Favicon SVG (monogram) + redirect `/favicon.ico`.
- **Fase 1-2 (spike):** fondasi Go + Chi + gomponents + Datastar + Postgres (pgx/sqlc) + goose. Vertical slice `todos` end-to-end (list keyset-paginated, create via `@post` SSE, delete via `@delete`).
- **Fase 4 (auth):** register/login/logout dengan argon2id (OWASP 2026), session scs + rueidis Store kustom (satu klien Redis), `RequireAuth` middleware sadar Datastar, CSRF via `http.CrossOriginProtection` (stdlib).
- Anti user-enumeration (pesan login generik) & anti session-fixation (`RenewToken`).
- Migrasi goose dengan Postgres advisory lock (aman multi-instance).
- Health check terpisah: `/healthz` (liveness) & `/readyz` (readiness).
- Middleware produksi: recover (log stack + request-id), security headers (CSP/nosniff/frame-deny), request-id, request logging (skip health probe).
- Cache-busting aset: `app.css` disajikan lewat path ber-hash konten (`app.<hash>.css`) dengan `Cache-Control: immutable`.
- Pipeline CSS no-Node: Tailwind v4 standalone CLI scan file `.go` → utility layer, plus Basecoat v1.0.2 vendored (checksum di `VENDOR.md`).
- Graceful shutdown (drain 20s via `signal.NotifyContext`).

### Added
- Reconcile super-admin saat boot: email di `SUPER_ADMIN_EMAILS` yang terdaftar dinaikkan ke `super_admin` di DB (promote-only — tak pernah menurunkan super-admin lain, dan email yang dicabut dari env tak otomatis turun). Membuat kolom `role` "jujur" untuk root env.
- Sidebar: pintasan cepat lintas-panel sesuai role (Developer → `/dev/users`, Admin → `/admin`), di-precompute dari izin Casbin, muncul di footer sidebar hanya bagi yang berhak.

### Changed
- AppShell: header dihilangkan — avatar/email/logout pindah ke footer sidebar. Sidebar bisa di-collapse jadi rail ikon (state persisten via `sidebar.js` + localStorage, tanpa dependency; label jadi tooltip saat rail). Tombol buka drawer mobile jadi floating hamburger.
- Schema `users`: `pass_hash` kini **nullable** (user Google-only tak punya password) + kolom `email_verified`. Ripple: `User.PassHash` jadi `*string`; login password menolak akun tanpa hash dengan pesan generik.
- Schema `users` += `role`, `status`, `avatar_url`, `deleted_at` (soft-delete). Query login (`GetUser`/`GetUserByEmail`) memfilter `deleted_at IS NULL`. Session menyimpan identitas (role/isRoot/email/avatar) saat login → `renderPage` tak lagi hit DB per render.

### Fixed
- Layout komponen berantakan (card melebar, spacing dobel, kotak error hantu): app.css semula meng-`@import "tailwindcss"` penuh → preflight-nya (dimuat setelah basecoat.css) me-reset border/input/spacing Basecoat. Fix: app.css hanya impor `theme.css`+`utilities.css` (tanpa preflight); satu preflight = milik basecoat.css.
- Kontrak markup Basecoat: `Card` kini bungkus isi dengan `<section>` (sumber padding & gap kartu), slot error pakai `AlertSlot` (div kosong) agar `.alert` (selalu ber-border) tak tampil sebagai kotak hantu saat kosong.
- Bug integrasi scs + Datastar SSE: `Set-Cookie` tak terkirim karena `NewSSE` flush header sebelum scs menulis cookie. Fix: `session.WriteCookie` manual sebelum `NewSSE`.
- `make build` gagal `sqlc: No such file or directory`: GNU Make 3.81 (macOS) meng-exec recipe tanpa metachar via `execvp` (lewat shell), jadi `export PATH` tak terbaca. Fix: panggil tool via path absolut `$(GOBIN)/sqlc`.
- `make setup` bisa hasilkan `tailwindcss` terpotong (unduh parsial `curl -sL` tanpa deteksi gagal) → Mach-O rusak → *"Malformed Mach-o file"* (SIGKILL Apple Silicon). Fix: `make tailwind` pakai `curl -fL --retry` + exec-test hasil unduh, tolak binary korup.

### Notes
- Auth masih memakai flow register/login langsung; belum ada halaman profil/authz berbasis role.
