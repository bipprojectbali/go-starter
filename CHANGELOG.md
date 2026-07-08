# Changelog

Semua perubahan penting pada go_stater dicatat di sini.

## [Unreleased]

### Added
- **Fase 1-2 (spike):** fondasi Go + Chi + gomponents + Datastar + Postgres (pgx/sqlc) + goose. Vertical slice `todos` end-to-end (list keyset-paginated, create via `@post` SSE, delete via `@delete`).
- **Fase 4 (auth):** register/login/logout dengan argon2id (OWASP 2026), session scs + rueidis Store kustom (satu klien Redis), `RequireAuth` middleware sadar Datastar, CSRF via `http.CrossOriginProtection` (stdlib).
- Anti user-enumeration (pesan login generik) & anti session-fixation (`RenewToken`).
- Migrasi goose dengan Postgres advisory lock (aman multi-instance).
- Health check terpisah: `/healthz` (liveness) & `/readyz` (readiness).

### Fixed
- Bug integrasi scs + Datastar SSE: `Set-Cookie` tak terkirim karena `NewSSE` flush header sebelum scs menulis cookie. Fix: `session.WriteCookie` manual sebelum `NewSSE`.

### Notes
- Auth masih memakai user seed di spike; belum ada halaman profil/authz berbasis role.
- CSS masih placeholder minimal (belum Tailwind + Basecoat asli).
