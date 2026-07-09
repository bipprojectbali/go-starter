# CLAUDE.md — panduan agen untuk go_stater

Panduan spesifik proyek untuk agen. Baca ini + [`STATER.md`](STATER.md) (spec &
alasan arsitektur) sebelum kode. README untuk cara pakai; file ini untuk
konvensi + **gotcha yang mahal ditemukan ulang**.

## Alur kerja wajib

- **`make check` adalah gerbang** — jalankan setiap selesai perubahan; harus hijau
  (sqlc · vet · gofmt · build · test) sebelum lapor selesai.
- Tooling (`sqlc`/`goose`/`air`) di `$(go env GOPATH)/bin`. Makefile memanggilnya
  via **path absolut** (`$(GOBIN)/sqlc`) — GNU Make 3.81 di macOS meng-exec recipe
  tanpa metachar lewat `execvp`, jadi `export PATH` tak terbaca. Jangan ubah balik
  ke pemanggilan telanjang.
- Ubah schema → tulis migrasi goose → jalankan lokal → `sqlc generate` → perbaiki
  ripple. Jangan edit `internal/db/*` (generated).
- Fitur baru wajib disertai test dalam pekerjaan yang sama. Test butuh DB pakai
  `TEST_DATABASE_URL` (Postgres, jangan tukar engine), di-`skip` bila kosong.

## Arsitektur & konvensi

- **`routes.go` = single source of truth** semua route. Middleware terproteksi:
  `RequireAuth` → `RefreshIdentity` → `RequireEnforce(obj, act)`.
- **Config dibaca HANYA di `internal/config`** — jangan `os.Getenv` tersebar.
- **Handler tak menyimpan config**; nilai di-inject via setter global saat startup
  (pola `SetCSSPath`, `SetDevMode`, `SetGoogleOAuth`, `SetSuperAdminChecker`,
  `session.Init`, `authz.Init`). Ikuti pola ini, jangan bikin jalur baru.
- **View murni-data**: fungsi gomponents menerima data siap-render; jangan panggil
  `authz.Can`/session dari dalamnya. Precompute flag di handler, oper ke view
  (lihat `ui.When`, `quickLinksFor`).
- **Dua jalur render**: `renderPage` (Layout landing/app) vs `renderShell`
  (AppShell panel dgn sidebar). `headNodes()` dibagi keduanya (DRY).

## Gotcha (mahal — jangan temukan ulang)

1. **CSP wajib `unsafe-eval`.** Datastar mengevaluasi ekspresi `data-*` via
   `new Function()`. Tanpa `script-src 'self' 'unsafe-eval'`, SELURUH Datastar
   mati senyap (@post/signals/patch tak jalan). Ada test regresi di `mw`.
2. **scs + Datastar SSE cookie.** `NewSSE` flush header lebih dulu & bypass
   pembungkus scs → `Set-Cookie` tak terkirim. Panggil `session.WriteCookie`
   **sebelum** `NewSSE` di handler yang memulai session.
3. **SameSite=Lax, bukan Strict.** Callback OAuth = navigasi top-level dari
   accounts.google.com (cross-site); Strict menahan cookie → "state tidak valid".
4. **CSS: token & class harus ada di app.css theme.** app.css impor
   `theme.css`+`utilities.css` SAJA (tanpa preflight — basecoat.css punya preflight;
   dua preflight saling reset). Class dari token Basecoat (`bg-sidebar`,
   `text-muted-foreground`, `--color-destructive`) TAK tergenerate kecuali token-nya
   ditambahkan ke `@theme` di `static/input.css`. Class yang tak dipakai = tak
   digenerate; kalau perlu, pakai di markup dulu.
5. **`data.Class` key ber-hyphen wajib di-quote**: `data.Class("'translate-x-0'", ...)`
   — tanpa kutip jadi `{translate-x-0: ...}` = JS invalid, Datastar crash.
6. **`@post {contentType:'form'}` butuh `<form>` terdekat** untuk kirim value.
   Select tanpa `<form>` pembungkus = tak ada data terkirim.
7. **Toast/notifikasi wajib `pointer-events:none`** — `opacity:0` pun tetap
   menangkap klik & memblokir elemen di bawahnya.
8. **Super-admin env-override**: email di `SUPER_ADMIN_EMAILS` = super_admin
   efektif walau kolom `role` di DB = `user`. Reconcile boot mempromosikan mereka
   (promote-only, tak pernah demote). `RefreshIdentity` me-load role/status segar
   dari DB per-request (jangan cache di session — perubahan harus real-time).
9. **Panel dev-only** (`/dev/health`, `/dev/erd`) gated `devMode` di DUA tempat:
   route (`if devMode`) DAN menu (`devNav()`). Source/tooling tak ada di
   single-binary produksi.

## Client-side JS (CSP-safe)

Datastar bukan untuk manipulasi tabel (filter/paginate/copy) atau zoom. Untuk itu,
file terpisah di `static/` (`sidebar.js`, `health.js`, `erd.js`) — same-origin
lolos `script-src 'self'`, **bukan inline** (inline diblokir CSP). Muat via
`<script src>`, dimuat sinkron bila perlu set state sebelum paint.

## Aset vendored

`static/` berisi aset vendored (datastar.js, basecoat.css, mermaid.min.js) —
checksum di `static/VENDOR.md`. Tailwind CLI di-download `make setup` (gitignored,
verifikasi integritas). Prinsip no-CDN: semua di-embed.

## Batasan

- Semua interaktivitas = **Datastar** (satu paradigma). Jangan tambah framework JS.
- Single-binary — jangan tambah dependency yang butuh runtime kedua / Node.
- Password auth = dev-only; jangan aktifkan jalurnya di production.
