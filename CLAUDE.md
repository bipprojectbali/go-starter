# CLAUDE.md — panduan agen untuk go_starter

Panduan spesifik proyek untuk agen. Baca ini + [`STARTER.md`](STARTER.md) (spec &
alasan arsitektur) sebelum kode. README untuk cara pakai; file ini untuk
konvensi + **gotcha yang mahal ditemukan ulang**.

## Alur kerja wajib

- **`make check` adalah gerbang** — jalankan setiap selesai perubahan; harus hijau
  (sqlc · vet · gofmt · build · test) sebelum lapor selesai.
- **Tiap paket test punya SCHEMA Postgres sendiri** (`internal/testdb`), jadi
  `go test ./...` boleh paralel. Paket yang butuh DB wajib punya `TestMain` yang
  memanggil `testdb.Pool(ctx, "<nama>")` + `testdb.Drop`; test mengambil pool
  dari variabel paket (`pkgPool`), TIDAK membuka `pgxpool.New` sendiri — pool
  yang dibangun dari DSN mentah menunjuk `public`, tempat tabelnya tak ada.
  **`search_path` sengaja TANPA `public`**: goose mencari tabel versinya lewat
  search_path, dan dengan `public` di sana ia menemukan `goose_db_version`
  database utama, membacanya sebagai "sudah versi terakhir", lalu tak
  menjalankan apa pun — schema baru tetap KOSONG dan gagalnya menunjuk ke
  mana-mana kecuali penyebabnya. Migrasi `00004` memberi GRANT `app_rw` ke
  `current_schema()` (bukan `public` harfiah, seperti `00001`) supaya RLS tetap
  mengikat di schema mana pun.
- Tooling (`sqlc`/`goose`/`air`) di `$(go env GOPATH)/bin`. Makefile memanggilnya
  via **path absolut** (`$(GOBIN)/sqlc`) — GNU Make 3.81 di macOS meng-exec recipe
  tanpa metachar lewat `execvp`, jadi `export PATH` tak terbaca. Jangan ubah balik
  ke pemanggilan telanjang.
- Ubah schema → tulis migrasi goose → jalankan lokal → `sqlc generate` → perbaiki
  ripple. Jangan edit `internal/db/*` (generated).
- **Skema hidup di SATU migrasi** (`00001_schema.sql`) — 11 migrasi bertahap
  disatukan saat belum ada deployment (0007). Perubahan berikutnya **inkremental
  seperti biasa** (`00002_...`); jangan menyunting `00001` setelah orang lain
  meng-clone repo ini, dan jangan menyatukan ulang setelah ada produksi.
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
  bikin overflow mubazir. Terukur: satu tabel telanjang membuat halaman anggota
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
  skill **ego-browser**, set lebar ke **375** (mobile), **768** (tablet), **1280**
  (desktop) + screenshot tiap lebar. Bukti visual, bukan klaim "sudah responsif".
  Tak ada helper `set_viewport` — pakai CDP langsung, lalu reset setelah selesai:
  `await cdp('Emulation.setDeviceMetricsOverride', {width: 375, height: 812,
  deviceScaleFactor: 2, mobile: true})` … `await cdp('Emulation.clearDeviceMetricsOverride', {})`.

## Identitas panel (/w/{slug} · /dev)

Semua halaman memakai AppShell yang SAMA. Tanpa penanda, user tak tahu sedang di
shell mana — dan itu bukan cuma kosmetik: **`/dev` menampilkan data LINTAS-workspace**
(`ListUsers` melihat semua tenant), jadi salah mengira sedang di ruang kerja =
salah membaca cakupan data.

- **Sumbernya `internal/ui/panelkind.go`** (`Panel` bertipe + `panelStyles` array
  berindeks enum → menambah panel tanpa gaya = compile error). Handler tak perlu
  mengoper apa-apa: `panelOf(ctx, currentPath)` menurunkannya.
- **Hanya `/dev` ditentukan PATH.** Sejak 0004 satu alamat `/w/{slug}` melayani
  semua role, jadi chip-nya diturunkan dari ROLE (`panelForRole`) — sumber yang
  sama dengan `navFor`, supaya chip tak pernah bertentangan dengan menu yang
  tampil. Di ruang kerja, chip justru MENJADI penanda otoritas: owner/admin
  melihat `ADMIN`, member melihat `RUANG KERJA`, di halaman yang identik.
- **DUA penanda, sengaja**: chip TEKS (`RUANG KERJA`/`ADMIN`/`PLATFORM`) + aksen
  warna di tepi atas sidebar. Teks saja kurang menonjol; warna saja gagal untuk
  yang buta warna. Keduanya juga saling menutupi keadaan: saat sidebar collapse
  jadi rail 4rem, `.app-brand` disembunyikan penuh oleh `input.css` (chip ikut
  hilang) — **aksen tepi yang bertahan**. Jangan hapus salah satunya.
- **Warna WAJIB token semantik daisyUI** (`primary`/`secondary`/`warning`), bukan
  absolut (`bg-red-500`) — token didefinisikan ulang tiap tema (gotcha #11).
  `/dev` memakai `warning` bukan sebagai warna ketiga, tapi sebagai peringatan.
  Terverifikasi terbaca di ke-6 tema (selisih luminance 0.41–0.69).
- Chip **menggantikan** sub-label panel, tidak menumpuk — dua penanda konteks di
  satu tempat justru bising. Brand utama = NAMA WORKSPACE (role tenant) atau
  `go_starter /dev` (platform).

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
    **Presence & audit menjawab pertanyaan BERBEDA — jangan digabung.** Presence =
    "kapan orang ada" (agregat, murah, tanpa detail); `audit_logs` = "siapa
    melakukan apa" (bukti, per-peristiwa). Keduanya tampil sebagai dua tabel
    terpisah di `/dev/logs`. Godaan "biar detail" yang berakhir mencatat tiap
    request melanggar Rule 13 — dan itu justru alasan bucket 15-menit ini ada.
    **Retensi audit ada di `internal/maintenance`**, bukan lagi task terbuka.
    `audit_logs` dibersihkan harian sesuai `settings.KeyAuditRetentionDays`
    (default 365 hari, minimal 30 — sama dengan tenggang workspace terhapus,
    agar jejak penghapusan tak hilang sebelum workspace-nya sendiri dibuang).
    Batasnya di DB supaya operator bisa MENAIKKANNYA sebelum sesuatu telanjur
    hilang; menurunkannya ter-audit, sebab mempersingkat retensi adalah cara
    paling rapi menghapus jejak. Dua perlakuan berbeda yang SENGAJA: angka di
    luar batas → tolak & jangan hapus apa pun (niat yang keliru, tak bisa
    dibatalkan); nilai tak terparse → jatuh ke default & tetap jalan (ketiadaan
    nilai; menghentikan pemeliharaan karena satu baris rusak membuat tabel
    tumbuh diam-diam berbulan-bulan).
    **`target_type` menentukan tabel yang di-JOIN saat jejak dibaca**
    (`user`/`session`→users, `workspace`→tenants, `platform`→tak di-JOIN), jadi
    salah menyebutnya bukan label keliru melainkan NAMA ORANG yang keliru: id
    workspace akan mencocoki baris users yang id-nya kebetulan sama. Pakai
    `h.audit` (sasaran user) · `h.auditWorkspace` · `h.auditPlatform` — jangan
    `auditLog` telanjang. Pernah terjadi: `h.audit` meng-hardcode `"user"` untuk
    SEMUA aksi, dan tujuh aksi mengirim `tenants.id` (diperbaiki migrasi 00003).
    **Kalimat peristiwa di `internal/activity/trail.go`** (pure, tanpa DB) —
    action tak dikenal WAJIB jatuh ke kalimat netral + kodenya, jangan string
    kosong: jejak dibaca justru saat ada yang aneh, dan baris yang menghilang
    karena kodenya belum dikenal adalah kegagalan terburuk halaman ini. Nama
    di-JOIN saat BACA, tak pernah disalin ke `metadata` (kolom itu bebas PII;
    salinan nama di sana tak ikut terhapus bersama usernya).
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

## Mode tenancy: single → multi (ratchet) — keputusan 0006 & 0007

Template melayani DUA bentuk, tapi **bukan dua jalur kode**. Setiap aplikasi
lahir sebagai **single** dan boleh **dinaikkan** ke multi; turun **tak pernah**.

- **Single = multi-tenant dengan N=1.** Di dalam tetap ada TEPAT SATU tenant;
  RLS/memberships/audit jalan apa adanya. Yang hilang cuma chrome-nya. Jangan
  tergoda membuang `tenant_id` "karena toh cuma satu" — itu melahirkan aplikasi
  kedua dalam satu repo, dan jalur yang jarang dipakai pasti membusuk.
- **SATU bentuk URL untuk kedua mode** (`/w/{slug}`); aplikasi tunggal ada di
  **`/w/app`**. `SingleAppPrefix` sudah DIHAPUS. Dulu mode single memakai
  `/app/...`, sehingga menaikkan mode mengubah SETIAP alamat yang sudah tersebar.
  Sekarang `wsPath` & `slugFromRequest` tak punya cabang mode sama sekali.
- **Mode hidup di DATABASE** (`platform_settings.tenancy_mode`), bukan env.
  Nol baris = single. `APP_MODE` **sudah dihapus** — jangan hidupkan kembali.
  Penurunan ditolak DUA trigger: `BEFORE UPDATE` (multi→lain) dan `BEFORE DELETE`
  (baris absen dibaca sebagai single, jadi menghapusnya adalah penurunan yang
  menyamar — ini pernah lolos saat dirancang dengan satu trigger).
- **Route SELALU didaftarkan**, tak lagi bersyarat mode (mengubah 0006 §9). Yang
  menahan zona bahaya adalah `tenants.is_primary` — dijaga di handler DAN di SQL
  (`AND NOT is_primary`). Route bersyarat telanjur salah begitu mode bisa naik
  saat aplikasi berjalan.
- **Workspace PRIMER = rumah aplikasi.** Kolom `is_primary` + unique partial index
  (tepat satu). **Tak bisa diarsipkan/dihapus** (mengarsipkannya = seluruh aplikasi
  read-only lewat tombol yang tampak rutin) dan **tak memakan kuota** (kuota
  membatasi yang DIBUAT; rumah aplikasi tak dibuat siapa pun). Pengecualian kuota
  WAJIB sama di sidebar & penegakan — beda sedikit = tombol yang lalu ditolak.
- **super_admin = OWNER workspace primer**, dipasang saat LOGIN
  (`ensurePrimaryOwner`, idempotent & promote-only) — bukan saat boot, sebab di
  sana belum ada satu pun baris `users`. Tanpa ini rumah aplikasi tak punya owner,
  dan setelah naik ke multi ia jadi satu-satunya workspace yang mustahil dikelola
  seperti yang lain. **Jangan pernah menentukan jalur RLS dari keanggotaan** —
  keputusan bypass tetap dari ROLE, tak pernah dari data.
  Konsekuensi disengaja: mencabut email dari `SUPER_ADMIN_EMAILS` menurunkannya
  jadi **owner biasa**, bukan user biasa.
- **Pendaftar di mode single = `member`, BUKAN owner** (`placeNewUser`). Tanpa
  ini setiap orang yang mendaftar jadi pemilik aplikasi.
- **Wewenang single**: super_admin (env) → fundamental; admin → operasional
  (termasuk nama app); member → pakai. `canEditWorkspace` melonggar untuk admin
  di mode single — tapi **alasannya bukan lagi "tak ada owner"** (sejak 0007 ada):
  admin adalah pembantu operasional, dan mengganti nama aplikasi tak boleh
  menuntut orang menyunting `.env` lalu restart.
- **`GuardSetRole` memakai `>=`**, bukan `>`: tanpa itu admin bisa mengangkat
  sesama admin — memperbanyak dirinya sendiri.
- **Boot** (`BootstrapPrimary`): workspace primer dibuat bila belum ada, mode
  dibaca dari DB. Pemeriksaan lama ">1 workspace tapi single → tolak start"
  **dihapus, bukan dipindah** — keadaan itu tak bisa terjadi lagi.
- **Test WAJIB di kedua mode** untuk jalur yang bergantung mode (`withMode` +
  `t.Cleanup` di `appmode_test.go`) — mode adalah state paket, dan yang lupa
  dipulihkan meracuni test lain dengan gejala di tempat tak berhubungan.
  `setupTest` menyetel MULTI secara eksplisit (seed-nya workspace biasa, bukan
  primer); default paket adalah Single.
- **Menaikkan mode ada di `/dev/settings`** (kartu Mode Tenancy), di-gate
  `platform:settings` yang sama seperti kuota — ini keputusan paling fundamental
  di halaman itu, jadi tak boleh lebih longgar. Konfirmasi = MENGETIK nama
  aplikasi, bukan checkbox (checkbox bisa dicentang tanpa dibaca; menyalin nama
  menuntut orangnya melihat objek yang terlibat). Setelah naik, formnya HILANG
  diganti keterangan keadaan — tombol tanpa efek lebih buruk daripada tombol yang
  tak ada. Wajib ter-audit: perubahan bentuk aplikasi yang tak bisa dibatalkan
  harus punya jawaban untuk "siapa & kapan".

## Multi-tenancy (RLS + membership + role 2-bidang) — keputusan 0002, 0003 & 0004

Isolasi tenant ditegakkan **Postgres RLS**, bukan cuma `WHERE tenant_id` di app.
Keanggotaan = model **membership** (satu user boleh di banyak workspace dgn role
berbeda) — `docs/decisions/0003` men-supersede "1 user = 1 tenant" di 0002.
Workspace aktif hidup di **PATH** (`/w/{slug}`), bukan session — `0004`
men-supersede pemilihan-via-session di 0003 #4. Gotcha yang mahal ditemukan ulang:

- **URL workspace HANYA lewat `wsPath`/`wsRedirect`** (`internal/handler/wspath.go`)
  — satu-satunya tempat literal `/w/` boleh muncul (Rule 15). Slug kosong →
  `/workspace/new`, bukan `/w//x` yang rusak senyap.
- **`/admin` & `/user` TIDAK ADA lagi** — dilebur jadi `/w/{slug}`. Keduanya
  membedakan ROLE, bukan RESOURCE: satu orang bisa owner di A & member di B, jadi
  alamat per-role membuat halaman yang sama berpindah alamat saat ganti workspace.
  Aturannya: **beda role = beda AKSI di halaman yang sama, BUKAN beda ALAMAT.**
  Pembatasan di handler (`canEditWorkspace`/`canManageMembers`), bukan di route.
- **Slug asing/tak dikenal → `http.NotFound`**, jangan 403 (mengonfirmasi
  workspace itu ada) dan jangan redirect (menampilkan data workspace lain secara
  senyap — persis penyakit yang diobati 0004).
- **Role PLATFORM pun wajib mengikuti slug** — cabang platform di `Scope` bypass
  RLS tapi TETAP memanggil `adoptTenantBySlug`; tanpa itu super_admin membuka
  `/w/acme/members` melihat anggota workspace lain di bawah URL yang menjanjikan
  `acme`. Keputusan bypass diambil dari ROLE, tak pernah dari data DB.
- **`/dev`, `/notifications`, `/invite/{token}`, `/workspace/new` SENGAJA tanpa
  slug** — cakupan datanya bukan satu tenant. Path mengikuti cakupan data.
- **DUA pintu mengubah role, kabarnya harus SAMA.** `/w/{slug}/members` (pengelola
  workspace) & `/dev/users` (operator platform) memanggil `UpdateMemberRole` dan
  `authz.GuardSetRole` yang sama; keduanya WAJIB `h.notify(...,
  "member.role.changed")`. Efeknya di sisi penerima identik — yang tak boleh
  terjadi adalah ia mengetahui perubahan wewenangnya atau tidak, tergantung pintu
  mana yang kebetulan dipakai. Di `/dev` tenant notifikasi = workspace **TARGET**
  (dari form), BUKAN workspace aktor: panel itu lintas-workspace, jadi keduanya
  sering berbeda. Sebaliknya **status & soft-delete SENGAJA tanpa notifikasi** —
  keduanya menutup pintu login, jadi kabar in-app tak akan pernah terbaca; ia
  hanya menumpuk untuk saat statusnya dipulihkan, ketika sudah basi.
- **Daftar panjang wajib punya JALAN ke halaman berikutnya, bukan cuma `LIMIT`.**
  Query keyset yang selalu diminta dari halaman pertama = baris ke-21 dst
  mustahil dijangkau, dan gagalnya SENYAP (halaman tampil rapi, isinya saja tak
  lengkap). Pola: ambil `pageSize+1` → `splitPage` (baris lebih hanya penanda
  "masih ada", tak dirender — menghindari `COUNT` yang memindai seluruh tabel) →
  cursor lewat `?after=` (`internal/handler/pagecursor.go`). **Keyset, bukan
  OFFSET**: daftar diurut `created_at DESC` dan baris baru masuk di atas, jadi
  OFFSET menggeser isi halaman di antara dua klik. Cursor rusak → halaman
  pertama, JANGAN halaman kosong (terbaca sebagai "tak ada data"). Ujung daftar
  dikatakan eksplisit; navigasinya link biasa (`<a>`), bukan Datastar — pindah
  halaman itu NAVIGASI, harus bisa di-bookmark & dimuat ulang (sekaligus lolos
  gotcha #16 tanpa menyentuhnya).
- **Daftar anggota HANYA untuk pengelola** (owner/admin/platform), di kedua mode
  — 0008 men-supersede 0004 §3 untuk baris ini. Ia DIREKTORI ORANG (nama, wajah,
  keanggotaan dalam satu halaman yang bisa disalin sekaligus), dan member tak
  bisa berbuat apa pun dengannya: nol manfaat, biaya PII tetap. Gerbang di
  HANDLER (`canManageMembers` di baris pertama `MembersPage`, sebelum query
  dijalankan) — bukan route, sebab 0004 tetap berlaku: satu alamat, aksi
  mengikuti role. Ditolak **403 + penjelasan**, BUKAN 404: penerimanya sudah
  terbukti anggota (Scope memvalidasinya), jadi menyangkal keberadaan halaman
  hanya membuat orang mengira ada yang rusak. Menu `Anggota` WAJIB memakai izin
  yang SAMA — `workspaceNav` menerima dua izin terpisah (`canMembers`,
  `canSettings`) karena keduanya tak identik di mode single.
- **PII disamarkan di HANDLER, bukan di view.** Kalau view yang menyamarkan,
  alamat ASLI tetap harus dioper ke sana — dan satu pemakaian yang lupa akan
  mengirimnya ke browser, tempat ia terbaca di view-source meski tak tampak di
  layar. `maskEmail` kini nyaris tak tereksekusi (semua penglihat = pengelola)
  tapi SENGAJA dipertahankan (0008 §5): aturan tampilan & aturan akses adalah dua
  hal berbeda, dan yang satu tak boleh diam-diam bergantung pada yang lain.
  Domain DIPERTAHANKAN (itu yang membedakan rekan satu organisasi dari orang
  luar); panjang bagian lokal TIDAK dibocorkan; email sendiri selalu utuh.
- **Penanda orang = NAMA (`users.name`), email cadangan.** Claim `name` Google
  dulu dibaca lalu dibuang — `maskEmail` lahir sebagai kompensasi. Nama
  di-refresh tiap login (`UpdateUserProfile`, COALESCE: argumen NULL =
  "provider diam", BUKAN "hapus"). Nilainya USER-CONTROLLED → dibersihkan
  `oauth.NormalizeDisplayName` (kontrol/newline dibuang, spasi diciutkan,
  dipotong pada batas RUNE bukan byte) dan TAK PERNAH dipakai sebagai penanda
  unik maupun untuk otorisasi — `id` yang dipakai. **Catatan untuk kelak**: fitur
  "edit profil" akan tertimpa refresh tiap login; saat itu tiba, nama pilihan
  sendiri harus jadi kolom terpisah yang diutamakan, bukan mematikan refresh
  (avatar tetap perlu ikut berubah).
- **View TAK BOLEH merakit path workspace sendiri** — oper `base` dari handler
  (`panel.Members`, `panel.WorkspaceView.Base`). Pelanggarannya senyap: form
  tetap ter-render rapi, baru ketahuan saat di-SUBMIT (pernah terjadi — semua
  aksi anggota menunjuk `/admin/*` yang sudah 404). **Verifikasi UI harus
  mencakup submit, bukan cuma render.**

## Siklus hidup workspace (suspend · archive · delete) — keputusan 0005

`tenants.status` = `active|suspended|archived`, plus `deleted_at` (soft-delete,
tenggang 30 hari). Ditegakkan di `gateLifecycle` yang dipanggil `Scope`.

- **Bedanya KEWENANGAN, bukan rasa**: `suspended` = tindakan PLATFORM (owner tak
  bisa membatalkannya sendiri — kalau bisa, gunanya hilang); `archived` =
  keputusan OWNER (harus bisa dibuka lagi tanpa memohon). Guard `status='active'`
  di SQL `ArchiveTenant` mencegah owner keluar dari suspensi lewat arsip→unarsip.
- **Kode status per keadaan**: bukan-anggota → 404 · suspended → **403 + alasan**
  (anggota sah berhak tahu KENAPA; 404 bikin ia mengira workspace-nya hilang) ·
  archived → GET lolos, non-GET 403 · deleted → 404.
- **Platform SENGAJA menembus gerbang** (cabang platform di `Scope` tak memanggil
  `gateLifecycle`) — merekalah yang menangguhkan; menghalangi mereka bikin
  suspensi mustahil diselidiki. Dikunci `TestPlatformTembusSuspensi`.
- **Unarchive ada DI LUAR `/w/{slug}`** (`/workspace/{slug}/unarchive`): gerbang
  read-only memblokir semua POST di dalam, jadi pintu keluar tak boleh berada di
  ruangan yang ia buka. Konsekuensinya handler itu mencari tenant sendiri — dan
  **wajib lewat `tenantBySlug` untuk platform**, bukan `resolveTenantBySlug` yang
  mensyaratkan keanggotaan (platform bukan anggota → 404 padahal `isOwnerOf`
  mengizinkan; dua cek saling bertentangan, pernah terjadi).
- **`audit_logs.tenant_id` NULLABLE + `ON DELETE SET NULL`**.
  Sebelumnya NOT NULL tanpa CASCADE → `DELETE FROM tenants` GAGAL di tengah jalan
  setelah memberships/invites/notifications terlanjur CASCADE terhapus. Bukti tak
  boleh lenyap bersama yang dibuktikan.
- **Kuota**: terhapus TAK dihitung (terasa seperti bug & mendorong purge cepat);
  terarsip TETAP dihitung (datanya masih disimpan — arsip bukan celah kuota).
- **Slug tak dilepas saat terhapus** — kalau dilepas, orang lain bisa mengambilnya
  dan restore jadi mustahil.
- **Purge kini TERJADWAL** (`internal/maintenance`, dipanggil dari `main.go`).
  Dulu `PurgeTenant` + `ListExpiredTenants` ada tapi satu-satunya pemanggilnya
  test, jadi workspace terhapus menumpuk selamanya dan slug-nya tak pernah
  bebas. Runner-nya IN-PROCESS, bukan cron host: menuntut cron berarti
  pemeliharaan yang "seharusnya sudah dipasang", yaitu yang tak pernah dipasang.
  Dikunci `pg_try_advisory_lock` (id 4243, BEDA dari lock migrasi 4242) —
  `try`, bukan blocking: instance yang kalah lomba harus LEWAT, bukan antre lalu
  mengulang pekerjaan yang baru selesai. Purge dijalankan SATU PER SATU, bukan
  DELETE massal: satu baris bermasalah tak boleh menggagalkan pembersihan
  sisanya. Ditunda 5 menit setelah boot (saat tersibuk) dan berhenti sendiri
  saat SIGTERM (ctx yang sama dengan shutdown).

- **`h.q(ctx)`, JANGAN `h.DB`** (dihapus). `h.q` ambil `*db.Queries` ber-tenant dari
  middleware `Scope`. Lupa Scope = **panic keras** (bug wiring ketahuan seketika),
  bukan query tak ter-scope. Jalur pre-identity (auth/oauth/boot — tenant belum
  diketahui) pakai `db.WithSuper` eksplisit, BUKAN `h.q`.
- **`WithTenant`/`WithSuper` = SATU tx dgn GUC `set_config(...,true)` TRANSACTION-
  LOCAL + `SET LOCAL ROLE app_rw`** (`internal/db/tenant.go`). `,true` wajib —
  plain `SET` bocor ke peminjam pool berikutnya (kebocoran tenant #1 paling umum).
  Keputusan bypass diambil dari **ROLE** (`isPlatformRole`), TAK PERNAH dari data
  DB (anti privilege-escalation).
- **SATU DSN, hak diturunkan PER-TRANSAKSI (0007).** `FORCE` RLS wajib — tanpanya
  owner tabel bypass policy diam-diam — dan owner/superuser bypass apa pun yang
  terjadi. Dulu itu menuntut koneksi kedua (`APP_DATABASE_URL`, sudah DIHAPUS);
  sekarang `dropPrivileges` menjalankan `SET LOCAL ROLE app_rw` di dalam tx.
  Terverifikasi: superuser ikut tercabut di dalam tx, hak pulih di COMMIT MAUPUN
  ROLLBACK, `app.is_super` tetap bypass (jalur `/dev` utuh), DDL ditolak, ~5 µs.
  **Migrasi WAJIB `GRANT app_rw TO CURRENT_USER`** — owner non-superuser tak
  otomatis anggota app_rw, dan `SET LOCAL ROLE`-nya gagal di tiap transaksi.
  Yang dibayar: injection yang berhasil bisa `RESET ROLE`. Diterima karena semua
  query digenerate sqlc; **timbang ulang bila kelak ada SQL mentah dari user.**
  Konsekuensi bagus: RLS mengikat di DEV juga — query yang lupa `WHERE tenant_id`
  gagal di laptop, bukan di produksi.
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
- **Kuota workspace = DUA lapis** (`internal/settings`): default global di tabel
  `platform_settings` (bisa diubah dari `/dev/settings`, berlaku SEKETIKA tanpa
  restart) + override per-user di `users.workspace_quota`. **`NULL` = ikut
  global**, angka = hak khusus yang KEBAL perubahan global — pembedaan itu
  mustahil dengan kolom NOT NULL, dan itulah alasan ia dibuat nullable.
  `MAX_WORKSPACES_PER_USER` kini hanya **fallback** saat baris DB belum ada
  (deployment baru); dulu ia "aturan global" yang menyesatkan — nilainya cuma
  disalin saat user DIBUAT, jadi mengubahnya tak pernah menyentuh user lama.
  Hitung kuota HANYA lewat `settings.EffectiveWorkspaceQuota` — penegakan
  (`WorkspaceCreate`) dan tampilan (sidebar) wajib memakai sumber yang sama,
  beda sedikit = user melihat tombol yang lalu ditolak.
  Yang dihitung hanya workspace ber-role **owner** & belum terhapus — diundang
  jadi member/admin tak memakan kuota. `CountTenantOwners` mencegah owner
  terakhir diturunkan (workspace yatim).
- **`/dev/settings` di-gate `platform:settings`, BUKAN `dev:users`.** Seluruh
  grup `/dev` dijaga `dev:users` yang dimiliki **staff** — tanpa objek Casbin
  tersendiri, staff (yang sengaja dibatasi: tak boleh suspend tenant/kelola staff)
  bisa mengubah aturan yang berlaku bagi SETIAP user. `devNav` juga memeriksa izin
  ini agar menunya tak jadi menu hantu. Deny-default; hanya super_admin lolos.
- **Cache settings = PER-PROSES.** Perubahan seketika di instance yang melayani;
  instance lain menyusul saat boot. Diterima karena penulisnya hanya operator
  platform & efek terburuknya sementara. Kalau kelak butuh serempak: pub/sub
  Redis (sudah ada di stack), bukan menghapus cache.
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

## Konfigurasi produksi: gagal keras, jangan andalkan ingatan

Prinsipnya: **yang berbahaya bila salah harus menggagalkan boot; yang bisa
diturunkan otomatis jangan diminta ke manusia.** Warning di log adalah hal yang
paling sering diabaikan — ia bukan pengaman.

- **Isolasi tenant DIBUKTIKAN, bukan dijanjikan** (`db.CheckRLSTx`, dipanggil
  `verifyTenantIsolation` di `main.go`). Diperiksa DI DALAM `WithSuper` — yaitu
  pada transaksi yang sudah menurunkan haknya, persis keadaan setiap query
  aplikasi. Memeriksa pool telanjang menjawab pertanyaan yang SALAH: di sana
  koneksi memang masih owner, dan memang seharusnya (migrasi butuh itu).
  Yang ditanyakan ke Postgres jawabannya pasti: `rolsuper`/`rolbypassrls`/
  pemilik-tabel + `FORCE RLS`.
  **Berlaku sama di dev & production** — tak ada lagi kelonggaran per-mode, sebab
  gesekannya kini nol (dulu mengikat RLS menuntut role & DSN kedua).
  Sejarah yang tak boleh berulang: dulu ini sekadar "env `APP_DATABASE_URL`
  terisi", dan mengisinya dengan DSN yang SAMA seperti `DATABASE_URL` lolos
  sambil tetap membocorkan data — terukur di DB nyata: 82 baris dari 15 tenant.
- **Produksi tak butuh persiapan role manual.** Migrasi membuat `app_rw` lengkap
  dengan GRANT + `ALTER DEFAULT PRIVILEGES` (tabel dari migrasi berikutnya
  terjangkau otomatis) + `GRANT app_rw TO CURRENT_USER`. Rolenya tetap `NOLOGIN`
  — tak ada password, jadi tak ada `ALTER ROLE ... LOGIN` maupun entri
  `userlist.txt` PgBouncer. Kolom `GENERATED ALWAYS AS IDENTITY` juga tak
  menuntut GRANT sequence terpisah (terverifikasi: semua INSERT/UPDATE/DELETE
  aplikasi lolos sebagai `app_rw`).
- **`SESSION_KEY` divalidasi PANJANGNYA** (min 32, `config.MinSessionKeyLen`),
  bukan cuma keberadaannya. `mustEnv` meloloskan `SESSION_KEY=rahasia` — kunci
  lemah lebih berbahaya daripada kosong, sebab kosong menggagalkan boot sedangkan
  lemah menciptakan rasa aman yang keliru.
- **`SESSION_KEY` kini benar-benar DIPAKAI**: menurunkan nama cookie sesi
  (`Config.SessionCookieName`) agar dua deployment di host sama tak saling
  menimpa sesi. Yang dipakai HASH-nya — nama cookie terlihat di browser, jadi
  menaruh kuncinya di sana justru membocorkannya. Sebelumnya env ini divalidasi
  tapi nol pemakai: konfigurasi yang berbohong, pola yang sama dengan
  `tenants.status` (0005) & `MAX_WORKSPACES_PER_USER` (0006).
- **`Cookie.Secure` diturunkan dari `ENV=production`**, BUKAN env sendiri —
  tak ada yang bisa lupa mengisinya. Konsekuensi disengaja: production tanpa
  HTTPS = login tak berfungsi sama sekali (gagal keras, bukan bocor senyap).
- **Jangan tambah env baru untuk hal yang bisa diturunkan** dari env yang sudah
  ada. Tiap env baru adalah satu lagi hal yang bisa lupa diisi.

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
