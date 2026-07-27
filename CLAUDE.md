# CLAUDE.md — panduan agen untuk go_starter

Panduan spesifik proyek untuk agen. Baca ini + [`STARTER.md`](STARTER.md) (spec &
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
  `RequireAuth` → `Scope` → `RefreshIdentity` → `TrackPresence` → `RequireEnforce(obj, act)`.
  `Scope` (multi-tenancy) buka tx ber-tenant SEBELUM `RefreshIdentity`/`TrackPresence`
  (keduanya pakai `h.q(ctx)`). Lihat § Multi-tenancy.
- **Config dibaca HANYA di `internal/config`** — jangan `os.Getenv` tersebar.
- **Handler tak menyimpan config**; nilai di-inject via setter global saat startup
  (pola `SetCSSPath`, `SetDevMode`, `SetGoogleOAuth`, `SetSuperAdminChecker`,
  `SetAppTimezone`, `session.Init`, `authz.Init`). Ikuti pola ini, jangan bikin jalur baru.
- **Atribut Datastar via helper bertipe** (`internal/ui/dsx.go`): pakai `ClassOn`
  (class-toggle, auto-quote), `FormPostSelect` (@post form-valued + `<form>`),
  `PostAction`/`DeleteAction` (ekspresi aksi). Ini menutup gotcha #5 & #6 secara
  STRUKTURAL — jangan tulis `data.Class`/`@post` mentah di view (footgun senyap).
- **View murni-data**: fungsi gomponents menerima data siap-render; jangan panggil
  `authz.Can`/session dari dalamnya. Precompute flag di handler, oper ke view
  (lihat `ui.When`, `quickLinksFor`).
- **Dua jalur render**: `renderPage` (Layout landing/app) vs `renderShell`
  (AppShell panel dgn sidebar). `headNodes()` dibagi keduanya (DRY).

## Desain mobile-first (WAJIB — template ini di-clone)

Setiap UI **wajib** enak dipakai di mobile, tablet, DAN desktop. Ini base template;
kelalaian di sini menular ke tiap project turunan. Aturan yang bisa dicek (bukan
"pokoknya responsif"):

- **Arah mobile-first**: kelas dasar (TANPA prefix) = tampilan MOBILE; naikkan ke
  atas dengan `sm:`/`md:`/`lg:`. Tailwind/daisyUI memang mobile-first (unprefixed =
  semua lebar, prefix = min-width ke atas). JANGAN desain desktop-dulu lalu tambal
  `max-md:` — itu melawan arah framework & rapuh. ✅ `grid-cols-1 md:grid-cols-2`
  ❌ `grid-cols-2 max-md:grid-cols-1`.
- **Breakpoint standar** Tailwind: `sm` 640 · `md` 768 · `lg` 1024. Sidebar =
  drawer di `<md`, tetap di `md:` ke atas — **AppShell (`internal/ui/appshell.go`)
  adalah pola acuan** (`-translate-x-full` + `md:translate-x-0`, toggle via signal).
  Ikuti pola itu, jangan bikin mekanisme responsif baru.
- **Nol overflow horizontal** di 320–375px. Grid/flex turun ke 1 kolom di mobile.
  Konten lebar (angka panjang, URL) pakai `truncate`/`break-words`.
- **Tabel = titik gagal #1 di mobile → pakai `ui.TableScroll`.** Bungkus SETIAP
  `<table>` dengan helper itu; ada test regresi (`internal/ui/tablescroll_test.go`)
  yang menolak `h.Table(` tanpa pembungkus. Dua hal WAJIB bersamaan & mudah lupa:
  (a) `overflow-x-auto` di pembungkus **langsung** tabel — dipasang di `.card-body`
  TIDAK menahan (flex-col; min-content tabel tetap menular ke atas), dan (b)
  `min-w-0` — card/flex-item default `min-width:auto` sehingga MENOLAK menyusut,
  bikin overflow mubazir. Terukur: satu tabel telanjang membuat `/admin/members`
  meluber 439px di viewport 375px (H1 pun ikut melar — gejalanya menyesatkan,
  tampak seperti bug teks).
- **Baris tombol horizontal wajib `flex-wrap`.** Prev/next + angka halaman tak muat
  di 375px; tanpa wrap ia mendorong halaman (kejadian di `/dev/health` — `flex-wrap`
  di wrapper luar TIDAK menurun ke baris dalam).
- **Cara mendiagnosis overflow** (jangan menebak elemen mana): ukur di browser —
  `document.documentElement.scrollWidth` vs `clientWidth`, lalu daftar elemen dengan
  `getBoundingClientRect().right > vw` yang **tak** punya `closest('.overflow-x-auto')`.
  Yang di dalam kontainer scroll BUKAN pelanggaran; yang di luar itulah pelakunya.
- **Tap target ≥ 44px** (tombol/link aksi) — daisyUI `.btn` sudah cukup; hati-hati
  ikon-only kecil. **Input `text-base`** (≥16px) agar iOS tak auto-zoom saat fokus.
- **VERIFIKASI 3 LEBAR WAJIB** sebelum lapor selesai untuk perubahan UI apa pun:
  agent-browser `set_viewport` ke **375** (mobile), **768** (tablet), **1280**
  (desktop) + screenshot tiap lebar. Bukti visual, bukan klaim "sudah responsif".

## Gotcha (mahal — jangan temukan ulang)

1. **CSP wajib `unsafe-eval`.** Datastar mengevaluasi ekspresi `data-*` via
   `new Function()`. Tanpa `script-src 'self' 'unsafe-eval'`, SELURUH Datastar
   mati senyap (@post/signals/patch tak jalan). Ada test regresi di `mw`.
2. **scs + Datastar SSE cookie.** `NewSSE` flush header lebih dulu & bypass
   pembungkus scs → `Set-Cookie` tak terkirim. Panggil `session.WriteCookie`
   **sebelum** `NewSSE` di handler yang memulai session.
3. **SameSite=Lax, bukan Strict.** Callback OAuth = navigasi top-level dari
   accounts.google.com (cross-site); Strict menahan cookie → "state tidak valid".
4. **CSS: daisyUI = plugin Tailwind (satu pipeline, satu preflight).** `input.css`
   `@import "tailwindcss"` + `@plugin "./daisyui.js"` → `make css` menghasilkan
   `app.css`. Semua class komponen (`btn btn-primary`, `card`+`card-body`, `alert
   alert-error`, `badge badge-neutral`, `select`, `input`) & token tema
   (`bg-base-100/200/300`, `text-base-content`, `--color-error`) datang dari daisyUI
   — TANPA expose manual ke `@theme`. **Tapi tetap tree-shaken**: class yang tak
   dipakai di markup `.go` = tak digenerate; pakai di markup dulu. daisyUI TAK punya
   token `bg-sidebar`/`text-muted-foreground`/`--color-destructive` (itu Basecoat
   lama) — pakai padanan base: sidebar=`bg-base-200`, muted=`text-base-content/70`,
   destructive=`error`. `daisyui.js` WAJIB ada saat build (di-commit, di-`@plugin`).
5. **`data.Class` key ber-hyphen wajib di-quote** (`{translate-x-0: ...}` = JS
   invalid, Datastar crash). **Ditutup oleh `ui.ClassOn(class, expr)`** — helper
   mengutip otomatis, class ber-hyphen mustahil salah. Pakai helper, bukan
   `data.Class` mentah.
6. **`@post {contentType:'form'}` butuh `<form>` terdekat** (select tanpa `<form>`
   = tak ada value terkirim). **Ditutup oleh `ui.FormPostSelect(url, name, opts...)`**
   — merender `<form>`+`<select>` sebagai satu node, post form-valued tanpa `<form>`
   mustahil. Pakai helper, bukan `h.FormEl`+`@post` mentah.
7. **Toast/notifikasi wajib `pointer-events:none`** — `opacity:0` pun tetap
   menangkap klik & memblokir elemen di bawahnya.
8. **Super-admin env-override**: email di `SUPER_ADMIN_EMAILS` = super_admin
   efektif walau kolom `role` di DB = `user`. Reconcile boot mempromosikan mereka
   (promote-only, tak pernah demote). `RefreshIdentity` me-load role/status segar
   dari DB per-request (jangan cache di session — perubahan harus real-time).
9. **Panel dev-only** (`/dev/health`, `/dev/erd`) gated `devMode` di DUA tempat:
   route (`if devMode`) DAN menu (`devNav()`). Source/tooling tak ada di
   single-binary produksi.
10. **Tema: daftar HARUS selaras di 2 tempat.** `themeList` di
    `internal/ui/theme.go` (opsi dropdown) & blok `themes:` di `static/input.css`
    (yang di-generate daisyUI). Tema yang di-list Go tapi tak di input.css = CSS-nya
    tak ada → pilihan tak berefek. Tambah tema = edit KEDUANYA lalu `make css`.
11. **Hierarki permukaan warna (light & dark).** Latar halaman = `bg-base-200`,
    permukaan (card/sidebar) = `bg-base-100`. Token daisyUI RELATIF: kalau latar &
    card sama-sama `base-100`, kartu menyatu dengan latar di SEMUA tema. Ikuti
    hierarki ini; active-state menonjol pakai `bg-primary text-primary-content`
    (bukan `base-300` yang samar di sebagian tema). Jangan pakai warna absolut
    (`bg-white`, `text-black`, `bg-gray-*`) — tak adaptif antar-tema.
12. **Chart = ECharts vendored + init eksternal (BUKAN go-echarts).** go-echarts
    me-render lewat `<script>` inline → diblokir CSP (`script-src` tanpa
    `unsafe-inline`). Pola benar (sama ERD/mermaid): `echarts.min.js` vendored +
    `charts.js` eksternal; option chart dirakit di Go (`internal/activity/charts.go`),
    ditanam via `<script type="application/json">` (JSON tak dieksekusi → CSP-safe),
    `charts.js` `JSON.parse`+`setOption`. ECharts TAK butuh `unsafe-eval`.
13. **Presence tracking (Rule 13 — no write-storm).** `TrackPresence` merekam tiap
    request terautentikasi ke `activity_presence` via UPSERT bucket 15-menit
    (`hits+1`) — agregasi di level baris, bukan insert-per-request — + throttle
    in-process 60 dtk/user. WAJIB fail-soft (error rekam tak menggagalkan request).
    Pasang SETELAH `RefreshIdentity` (butuh uid).
14. **Timezone: simpan UTC, agregasi `AT TIME ZONE`.** Semua `created_at`/`bucket_at`
    TIMESTAMPTZ (UTC). "Jam berapa user aktif" dikonversi ke lokal via `AT TIME ZONE
    $tz` di SQL; `APP_TIMEZONE` (default Asia/Jakarta) dibaca di config, di-inject
    `handler.SetAppTimezone`. **`import _ "time/tzdata"` di `main.go` WAJIB** —
    `CGO_ENABLED=0` single-binary tak punya tzdata OS, `LoadLocation` gagal di prod
    tanpanya. sqlc agregasi: `SUM/COUNT` bungkus `COALESCE(...)::bigint` (bukan
    `pgtype.Numeric`); hindari `AT TIME ZONE` di SELECT list (sqlc emit `interface{}`)
    — kembalikan timestamptz mentah, format di Go.
15. **Response `content-type: text/javascript` = eksekusi JS di browser** (fitur
    Datastar, docs resmi — melengkapi #1). Body response dgn content-type itu
    dieksekusi sbg JavaScript; `PatchElements` yang menyisipkan `<script>` juga
    tereksekusi. Kita BERSIH (0 handler set content-type itu — jalur SSE cuma
    patch fragment ter-escape). Jangan set content-type itu, dan jangan patch
    `<script>` berisi data user, kecuali sengaja & sadar konsekuensinya. Aturan
    Datastar terkait (sudah dipatuhi): (a) escape SEMUA input user di view
    (`g.Text`, bukan `g.Raw`) — satu-satunya `g.Raw` = chart JSON dari
    `json.Marshal` (bukan user); (b) signal = user-modifiable → WAJIB validasi di
    backend (mis. `ValidRoleName`, `TrimSpace`+empty-check `$title`); (c) jangan
    taruh data sensitif di signal (terlihat plaintext di source).
16. **`sse.Redirect` (datastar-go) DIBLOKIR CSP — pakai HTTP 303 untuk NAVIGASI.**
    Konsekuensi langsung #1/#15, terverifikasi wire+CSP-event. `sse.Redirect`/
    `ExecuteScript`/`ReplaceURL` di datastar-go v1.2.2 mengirim `datastar-patch-
    elements` yang menyuntik `<script>setTimeout(...location.href)</script>` ke
    body. CSP kita (`script-src 'self' 'unsafe-eval'`, TANPA `unsafe-inline`)
    **memblokir** script inline itu (`script-src-elem blocked:inline`) → redirect
    MATI (aksi server sukses, tapi halaman tak pindah). Terjadi di logout/login/
    register. **Aturan**: aksi = NAVIGASI penuh (logout, login-sukses, register-
    sukses) → **native form POST → `http.Redirect(w,r,url,303)`**, BUKAN `sse.Redirect`.
    Native redirect juga commit cookie via scs `LoadAndSave` normal → tak perlu
    ritual `WriteCookie`+`NewSSE` (#2). Gagal validasi → PRG `?err=CODE`
    (`authErrMsg` map kode→pesan). SSE tetap untuk update FRAGMENT parsial (flash,
    row) yang di-escape — hanya redirect yang pindah ke HTTP. Asimetri kunci:
    `unsafe-eval` mengizinkan `new Function` (ekspresi `data-*` jalan), tapi
    `<script>` inline tetap diblokir — jadi menambah `unsafe-inline` BUKAN solusi
    (melemahkan CSP; lihat § Batasan).

## Multi-tenancy (RLS + membership + role 2-bidang) — keputusan 0002 & 0003

Isolasi tenant ditegakkan **Postgres RLS**, bukan cuma `WHERE tenant_id` di app.
Keanggotaan = model **membership** (satu user boleh di banyak workspace dgn role
berbeda) — `docs/decisions/0003` men-supersede "1 user = 1 tenant" di 0002.
Gotcha yang mahal ditemukan ulang:

- **`h.q(ctx)`, JANGAN `h.DB`** (dihapus). `h.q` ambil `*db.Queries` ber-tenant dari
  middleware `Scope`. Lupa Scope = **panic keras** (bug wiring ketahuan seketika),
  bukan query tak ter-scope. Jalur pre-identity (auth/oauth/boot — tenant belum
  diketahui) pakai `db.WithSuper` eksplisit, BUKAN `h.q`.
- **`WithTenant`/`WithSuper` = SATU tx dgn GUC `set_config(...,true)` TRANSACTION-
  LOCAL** (`internal/db/tenant.go`). `,true` wajib — plain `SET` bocor ke peminjam
  pool berikutnya (kebocoran tenant #1 paling umum). Keputusan bypass diambil dari
  **ROLE** (`isPlatformRole`), TAK PERNAH dari data DB (anti privilege-escalation).
- **`FORCE` RLS wajib** — tanpanya owner tabel bypass policy diam-diam. **App konek
  role non-owner** (`app_rw` `NOBYPASSRLS`, via `APP_DATABASE_URL`) — kalau konek
  owner/superuser, RLS TAK berlaku (bocor senyap). Dual-DSN: `DATABASE_URL` (owner:
  migrate+boot) vs `APP_DATABASE_URL` (runtime).
- **super_admin = ENV-ONLY, nol baris DB.** Role efektif di-overlay `RefreshIdentity`
  per-request (env-check → `platform_staff` lookup → `memberships.role` di workspace
  AKTIF). `platform_staff` TANPA RLS (platform-scope) → terbaca di WithTenant maupun
  WithSuper. Tak ada `PromoteSuperAdmins` (dihapus).
- **MEMBERSHIP: 1 user ↔ banyak workspace.** Role ada di `memberships` (user × tenant
  × role), BUKAN di `users` — orang yang sama bisa owner di A & member di B. `users`
  kini tabel **GLOBAL** (identitas murni): tanpa `tenant_id`/`role`, **KELUAR dari
  RLS**. Konsekuensi: `ListUsers` (panel /dev) lintas-workspace — itu route platform;
  daftar anggota workspace pakai `ListMembersByTenant`.
- **`notifications` TANPA RLS — SENGAJA, dan berbeda alasannya.** Bukan
  chicken-and-egg, tapi SEMANTIK: notifikasi milik USER dan lintas-workspace
  (undangan justru datang dari workspace yang BELUM jadi miliknya). RLS
  `tenant_id = GUC` malah akan menyembunyikan yang di luar workspace aktif —
  yaitu inti fiturnya. `tenant_id` di tabel ini cuma KONTEKS tampilan (nullable),
  bukan kunci isolasi. Konsekuensi: **jangan `h.q(ctx)`** (ter-scope satu tenant)
  — pakai `db.WithSuper` + `WHERE user_id/email` sebagai satu-satunya penjaga,
  dengan test isolasi antar-user sebagai pengganti jaring RLS.
- **`memberships`/`invites` TANPA RLS — SENGAJA.** Keduanya dibaca justru untuk
  MENENTUKAN scope (chicken-and-egg: tak bisa bergantung GUC yang belum di-set), dan
  invite dibuka di jalur publik. Keamanan dari filter query (`WHERE user_id = <uid
  sesi>` / token rahasia), bukan RLS.
- **`Scope` MEMVALIDASI keanggotaan** sebelum `WithTenant` (`resolveActiveTenant`):
  tenant di session itu user-controlled — tanpa cek ini user bisa memaksa workspace
  orang lain. Tak valid → fallback workspace pertama; tanpa workspace → `/workspace/new`.
  Validasi HARUS di Scope, bukan RefreshIdentity (yang jalan setelah scope terpilih).
- **Kuota workspace**: `users.workspace_quota` (default env `MAX_WORKSPACES_PER_USER`).
  Yang dihitung hanya workspace ber-role **owner** — diundang jadi member/admin tak
  memakan kuota. `CountTenantOwners` mencegah owner terakhir diturunkan (workspace yatim).
- **Register/OAuth = user + workspace + membership owner** dalam SATU tx `WithSuper`
  (atomik). `startIdentity(preferTenant)` memilih workspace aktif.
- **Audit di tx `WithSuper` TERPISAH** dari Scope tx (fail-soft struktural: gagal
  audit tak abort aksi utama). `tenant_id` audit = tenant aktor.
- **Test isolasi RLS** (`rls_test.go`) konek `app_rw` non-superuser via `SET ROLE` di
  `AfterConnect` — HARUS begitu utk membuktikan RLS sungguh mengikat. Test handler
  lain konek superuser (uji logika; RLS di-bypass diam-diam). Seed pakai owner-pool.
- **Casbin CSV TAK dukung komentar inline** di akhir baris `g,`/`p,` (jadi bagian
  nilai → link mati). Komentar HARUS di baris `#` tersendiri (gotcha: `g, super_admin,
  root  # x` bikin target `"root  # x"` → god-mode mati senyap).

## Client-side JS (CSP-safe)

Datastar bukan untuk manipulasi tabel (filter/paginate/copy), zoom, chart, atau
state UI persisten (collapse sidebar, tema). Untuk itu, file terpisah di `static/`
(`sidebar.js`, `theme.js`, `health.js`, `erd.js`, `charts.js`) — same-origin lolos
`script-src 'self'`, **bukan inline** (inline diblokir CSP). Muat via
`<script src>`, dimuat SINKRON bila perlu set state sebelum paint (no-FOUC):
`sidebar.js` & `theme.js` set atribut `<html>` (`data-sidebar`/`data-theme`)
sebelum body render. Data untuk JS (mis. option ECharts) ditanam via
`<script type="application/json">` — JSON tak dieksekusi (CSP-safe), JS `JSON.parse`.

## Aset vendored

`static/` berisi aset vendored (datastar.js, daisyui.js, mermaid.min.js, echarts.min.js) —
checksum di `static/VENDOR.md`. `daisyui.js` = plugin Tailwind (di-`@plugin` dari
input.css, dikonsumsi saat `make css` — bukan dimuat browser). Tailwind CLI
di-download `make setup` (gitignored, verifikasi integritas). Prinsip no-CDN:
semua di-embed.

## Batasan

- Semua interaktivitas = **Datastar** (satu paradigma). Jangan tambah framework JS.
- Single-binary — jangan tambah dependency yang butuh runtime kedua / Node.
- Password auth = dev-only; jangan aktifkan jalurnya di production.
- **`unsafe-eval` = keputusan sadar, BUKAN bug** (riset terverifikasi 2026): Datastar
  meng-`new Function()` ekspresi `data-*` di browser — nonce TAK bisa menggantikannya
  (beda concern). Menghapusnya butuh ganti runtime (fork dataSPA rapuh: 0 rilis/bus-
  factor-1, atau rombak ke `<form>`+POST yang buang SSE fragment patch). Risiko rendah:
  ekspresi cuma ID integer + literal internal, tak pernah input user. Jangan "perbaiki"
  dengan menambah `unsafe-inline` atau menukar runtime tanpa diskusi. Footgun authoring
  (bukan CSP) sudah ditutup helper `dsx.go` (gotcha #5–6).
