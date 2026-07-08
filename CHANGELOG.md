# Changelog

Semua perubahan penting pada go_stater dicatat di sini.

## [Unreleased]

### Added
- **Fase 1-2 (spike):** fondasi Go + Chi + gomponents + Datastar + Postgres (pgx/sqlc) + goose. Vertical slice `todos` end-to-end (list keyset-paginated, create via `@post` SSE, delete via `@delete`).
- **Fase 4 (auth):** register/login/logout dengan argon2id (OWASP 2026), session scs + rueidis Store kustom (satu klien Redis), `RequireAuth` middleware sadar Datastar, CSRF via `http.CrossOriginProtection` (stdlib).
- Anti user-enumeration (pesan login generik) & anti session-fixation (`RenewToken`).
- Migrasi goose dengan Postgres advisory lock (aman multi-instance).
- Health check terpisah: `/healthz` (liveness) & `/readyz` (readiness).
- Middleware produksi: recover (log stack + request-id), security headers (CSP/nosniff/frame-deny), request-id, request logging (skip health probe).
- Cache-busting aset: `app.css` disajikan lewat path ber-hash konten (`app.<hash>.css`) dengan `Cache-Control: immutable`.
- Pipeline CSS no-Node: Tailwind v4 standalone CLI scan file `.go` → utility layer, plus Basecoat v1.0.2 vendored (checksum di `VENDOR.md`).
- Graceful shutdown (drain 20s via `signal.NotifyContext`).

### Fixed
- Bug integrasi scs + Datastar SSE: `Set-Cookie` tak terkirim karena `NewSSE` flush header sebelum scs menulis cookie. Fix: `session.WriteCookie` manual sebelum `NewSSE`.

### Notes
- Auth masih memakai flow register/login langsung; belum ada halaman profil/authz berbasis role.
