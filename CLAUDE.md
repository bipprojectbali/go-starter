# CLAUDE.md — panduan agen untuk go_starter

Konvensi + **gotcha yang mahal ditemukan ulang**; baca sebelum kode. README = cara
pakai. Keputusan mutakhir di `docs/decisions/`.
[`docs/archive/STARTER-original.md`](docs/archive/STARTER-original.md) = spec desain
ASLI, **diarsipkan & sebagian usang** (Basecoat→daisyUI) — historis, jangan acuan.

## Alur kerja wajib

- **`make check` = gerbang** (sqlc · vet · gofmt · build · test) — hijau sebelum lapor selesai.
- **Tiap paket test punya SCHEMA Postgres sendiri** (`internal/testdb`) → `go test ./...`
  paralel. Paket ber-DB: `TestMain` panggil `testdb.Pool(ctx,"<nama>")` + `testdb.Drop`;
  test ambil pool dari var paket (`pkgPool`), JANGAN `pgxpool.New` sendiri (DSN mentah
  menunjuk `public`, tabelnya tak ada). **`search_path` sengaja TANPA `public`**: dengan
  `public`, goose menemukan `goose_db_version` DB utama, kira "sudah versi terakhir", tak
  jalan → schema baru KOSONG. Migrasi `00004` GRANT `app_rw` ke `current_schema()` (bukan
  `public` harfiah spt `00001`) → RLS mengikat di schema mana pun.
- Tooling (`sqlc`/`goose`/`air`) di `$(go env GOPATH)/bin`; Makefile panggil **path absolut**
  (`$(GOBIN)/sqlc`) — GNU Make 3.81 macOS exec via `execvp`, `export PATH` tak terbaca.
- Ubah schema → migrasi goose → jalankan lokal → `sqlc generate` → perbaiki ripple. Jangan
  edit `internal/db/*` (generated).
- **Skema di SATU migrasi** (`00001_schema.sql`) — 11 migrasi disatukan pra-deploy (0007).
  Berikutnya **inkremental** (`00002_...`); jangan sunting `00001` setelah di-clone, jangan
  satukan ulang setelah ada produksi.
- Fitur baru wajib test dalam pekerjaan sama; DB pakai `TEST_DATABASE_URL` (Postgres, jangan
  tukar engine), `skip` bila kosong.

## Arsitektur & konvensi

- **`routes.go` = single source of truth** semua route. Middleware terproteksi: `RequireAuth`
  → `Scope` → `RefreshIdentity` → `TrackPresence` → `RequireEnforce`. `Scope` buka tx
  ber-tenant SEBELUM `RefreshIdentity`/`TrackPresence` (pakai `h.q(ctx)`). Lihat § Multi-tenancy.
- **Ini TEMPLATE yang di-clone** — nama project/aplikasi tak boleh di-hardcode: module path
  via `make rename name=X`; nama layar dari `APP_NAME` lewat `handler.SetAppName` (`devBrand()`
  sidebar `/dev`, `LayoutData.Brand` header). Test brand pakai `devBrand()`, BUKAN string
  harfiah. Target `dev`/`css` bergantung `tailwind` supaya clone baru langsung jalan (binary
  gitignored; tanpanya `make dev` gagal tanpa menyebut `make setup`).
- **Query penyaring `table_schema` WAJIB `current_schema()`**, bukan `'public'` harfiah (sama
  GRANT 00004). Pernah di `internal/erd`: ERD KOSONG di schema non-public padahal halaman
  termuat → tampak DB kosong, bukan filter keliru.
- **Config dibaca HANYA di `internal/config`** — jangan `os.Getenv` tersebar.
- **Handler tak menyimpan config**; di-inject via setter global saat startup (`SetCSSPath`,
  `SetDevMode`, `SetGoogleOAuth`, `SetSuperAdminChecker`, `SetAppTimezone`, `session.Init`,
  `authz.Init`). Ikuti pola ini.
- **Atribut Datastar via helper bertipe** (`internal/ui/dsx.go`): `ClassOn` (class-toggle
  auto-quote), `FormPostSelect` (@post form-valued + `<form>`), `PostAction`/`DeleteAction`.
  Menutup gotcha #5 & #6 STRUKTURAL — jangan tulis `data.Class`/`@post` mentah (footgun senyap).
- **View murni-data**: gomponents terima data siap-render; jangan panggil `authz.Can`/session
  di dalamnya. Precompute flag di handler (`ui.When`, `quickLinksFor`).
- **Dua jalur render**: `renderPage` (Layout landing/app) vs `renderShell` (AppShell sidebar).
  `headNodes()` dibagi keduanya.

## Desain mobile-first (WAJIB — di-clone)

Setiap UI wajib enak di mobile, tablet, DAN desktop; kelalaian menular ke turunan.

- **Mobile-first**: kelas dasar (tanpa prefix) = MOBILE; naikkan `sm:`/`md:`/`lg:`. JANGAN
  desktop-dulu lalu tambal `max-md:`. ✅ `grid-cols-1 md:grid-cols-2`.
- **Breakpoint** `sm` 640 · `md` 768 · `lg` 1024. Sidebar = drawer `<md`, tetap `md:` ke atas
  — **AppShell (`internal/ui/appshell.go`) pola acuan** (`-translate-x-full` + `md:translate-x-0`,
  toggle signal). Jangan bikin mekanisme responsif baru.
- **Nol overflow horizontal** 320–375px. Konten lebar (angka/URL) pakai `truncate`/`break-words`.
- **Tabel = titik gagal #1 → `ui.TableScroll`.** Bungkus SETIAP `<table>` (test regresi
  `tablescroll_test.go` menolak `h.Table(` telanjang). Dua hal WAJIB bersamaan: (a) `overflow-x-auto`
  di pembungkus **langsung** tabel (di `.card-body` TIDAK menahan — flex-col); (b) `min-w-0`
  (card/flex-item default `min-width:auto` menolak menyusut). Satu tabel telanjang meluberkan
  halaman di 375px (H1 ikut melar — gejala menyesatkan).
- **Baris tombol horizontal wajib `flex-wrap`** (`flex-wrap` di wrapper luar TIDAK menurun ke
  baris dalam — kejadian `/dev/health`).
- **Diagnosis overflow** (jangan menebak): `scrollWidth` vs `clientWidth`, lalu elemen
  `getBoundingClientRect().right > vw` yang **tak** punya `closest('.overflow-x-auto')` (yang di
  dalam scroll BUKAN pelanggaran).
- **Tap target ≥ 44px** (daisyUI `.btn` cukup). **Input `text-base`** (≥16px) agar iOS tak auto-zoom.
- **VERIFIKASI 3 LEBAR WAJIB** untuk perubahan UI apa pun: skill **ego-browser**, **375**/**768**/**1280**
  + screenshot tiap lebar. Tak ada `set_viewport` — CDP: `cdp('Emulation.setDeviceMetricsOverride',
  {width:375,height:812,deviceScaleFactor:2,mobile:true})` lalu reset `clearDeviceMetricsOverride`.

## Identitas panel (/w/{slug} · /dev)

Semua halaman pakai AppShell SAMA; tanpa penanda user tak tahu shell mana — bukan kosmetik:
**`/dev` menampilkan data LINTAS-workspace** (`ListUsers` lihat semua tenant), salah kira sedang
di ruang kerja = salah baca cakupan.

- **Sumbernya `internal/ui/panelkind.go`** (`Panel` bertipe + `panelStyles` berindeks enum →
  tambah panel tanpa gaya = compile error). Handler tak mengoper: `panelOf(ctx, currentPath)`
  menurunkannya.
- **Hanya `/dev` ditentukan PATH.** Sejak 0004 `/w/{slug}` melayani semua role → chip dari ROLE
  (`panelForRole`, sumber sama `navFor` agar tak bentrok menu): owner/admin `ADMIN`, member
  `RUANG KERJA`, di halaman identik.
- **DUA penanda, sengaja**: chip TEKS + aksen warna tepi atas sidebar (teks kurang menonjol;
  warna gagal untuk buta warna). Saat rail 4rem `.app-brand` disembunyikan (chip hilang), aksen
  tepi bertahan. Jangan hapus salah satu.
- **Warna WAJIB token semantik daisyUI** (`primary`/`secondary`/`warning`), bukan absolut
  (gotcha #11). `/dev` pakai `warning` (peringatan, bukan warna ketiga).
- Chip **menggantikan** sub-label panel, tak menumpuk. Brand utama = NAMA WORKSPACE atau
  `go_starter /dev` (platform).

## Gotcha (mahal — jangan temukan ulang)

1. **CSP wajib `unsafe-eval`.** Datastar `new Function()` ekspresi `data-*`. Tanpa `script-src
   'self' 'unsafe-eval'`, SELURUH Datastar mati senyap. Test regresi di `mw`.
2. **scs + Datastar SSE cookie.** `NewSSE` flush header dulu & bypass scs → `Set-Cookie` tak
   terkirim. Panggil `session.WriteCookie` **sebelum** `NewSSE`.
3. **SameSite=Lax, bukan Strict.** Callback OAuth = navigasi cross-site top-level; Strict menahan
   cookie → "state tidak valid".
4. **CSS: daisyUI = plugin Tailwind (satu pipeline).** `input.css` `@import "tailwindcss"` +
   `@plugin "./daisyui.js"` → `make css` → `app.css`. Semua class komponen (`btn`, `card`+`card-body`,
   `alert`, `badge`, `select`, `input`) & token (`bg-base-100/200/300`, `text-base-content`,
   `--color-error`) dari daisyUI, TANPA `@theme` manual. **Tree-shaken**: class tak dipakai di
   markup `.go` = tak digenerate; pakai di markup dulu. daisyUI TAK punya
   `bg-sidebar`/`text-muted-foreground`/`--color-destructive` (Basecoat lama) — padanan:
   sidebar=`bg-base-200`, muted=`text-base-content/80`, destructive=`error`. `daisyui.js` WAJIB
   ada saat build (di-commit).
5. **`data.Class` key ber-hyphen wajib di-quote** (`{translate-x-0:...}` = JS invalid, crash).
   Ditutup `ui.ClassOn(class, expr)`. Pakai helper.
6. **`@post {contentType:'form'}` butuh `<form>` terdekat** (select tanpa `<form>` = tak ada
   value). Ditutup `ui.FormPostSelect(url, name, opts...)`. Pakai helper.
7. **Toast wajib `pointer-events:none`** — `opacity:0` pun tetap menangkap klik.
8. **Super-admin env-override**: email di `SUPER_ADMIN_EMAILS` = super_admin efektif walau `role`
   DB = `user`. Reconcile boot promote-only. `RefreshIdentity` load role/status segar per-request
   (jangan cache di session).
9. **Panel dev-only** (`/dev/health`, `/dev/erd`) gated `devMode` di DUA tempat: route (`if
   devMode`) DAN menu (`devNav()`). Source/tooling tak ada di binary produksi.
10. **Tema: daftar HARUS selaras 2 tempat.** `themeList` di `internal/ui/theme.go` & blok
    `themes:` di `static/input.css`. Tema di Go tapi tak di input.css = CSS tak ada. Tambah tema
    = edit KEDUANYA lalu `make css`.
11. **Hierarki permukaan warna.** Latar halaman `bg-base-200`, permukaan (card/sidebar) `bg-base-100`
    (token RELATIF: keduanya `base-100` → kartu menyatu). Active-state `bg-primary text-primary-content`
    (bukan `base-300` samar). Jangan warna absolut (`bg-white`/`text-black`/`bg-gray-*`) — tak adaptif.
12. **Chart = ECharts vendored + init eksternal (BUKAN go-echarts).** go-echarts render `<script>`
    inline → diblokir CSP. Pola benar (sama ERD/mermaid): `echarts.min.js` + `charts.js` eksternal;
    option dirakit di Go (`internal/activity/charts.go`), ditanam `<script type="application/json">`
    (CSP-safe), `charts.js` `JSON.parse`+`setOption`. ECharts TAK butuh `unsafe-eval`.
13. **Presence tracking (Rule 13 — no write-storm).** `TrackPresence` rekam tiap request auth ke
    `activity_presence` via UPSERT bucket 15-menit (`hits+1`) + throttle in-process 60 dtk/user.
    WAJIB fail-soft. Pasang SETELAH `RefreshIdentity` (butuh uid).
    **Presence & audit menjawab pertanyaan BEDA — jangan digabung.** Presence = "kapan orang ada"
    (agregat murah); `audit_logs` = "siapa melakukan apa" (per-peristiwa). Dua tabel terpisah di
    `/dev/logs`. Mencatat tiap request "biar detail" melanggar Rule 13.
    **Retensi audit di `internal/maintenance`**: `audit_logs` dibersihkan harian sesuai
    `settings.KeyAuditRetentionDays` (default 365, minimal 30 = tenggang workspace terhapus). Batas
    di DB agar operator bisa MENAIKKAN; menurunkannya ter-audit. Perlakuan beda SENGAJA: di luar
    batas → tolak & jangan hapus (tak bisa dibatalkan); tak terparse → default & tetap jalan.
    **`target_type` menentukan tabel di-JOIN saat baca** (`user`/`session`→users, `workspace`→tenants,
    `platform`→tak di-JOIN) → salah menyebut = NAMA ORANG keliru. Pakai `h.audit` (user) ·
    `h.auditWorkspace` · `h.auditPlatform`, jangan `auditLog` telanjang. Pernah: `h.audit` hardcode
    `"user"`, 7 aksi kirim `tenants.id` (diperbaiki 00003).
    **Kalimat peristiwa di `internal/activity/trail.go`** (pure) — action tak dikenal WAJIB jatuh
    ke kalimat netral + kode, jangan string kosong. Nama di-JOIN saat BACA, tak pernah disalin ke
    `metadata` (bebas PII).
14. **Timezone: simpan UTC, agregasi `AT TIME ZONE`.** Semua `created_at`/`bucket_at` TIMESTAMPTZ
    UTC; konversi lokal via `AT TIME ZONE $tz` di SQL. `APP_TIMEZONE` (default Asia/Jakarta)
    di-inject `handler.SetAppTimezone`. **`import _ "time/tzdata"` di `main.go` WAJIB**
    (`CGO_ENABLED=0` tak punya tzdata OS → `LoadLocation` gagal prod). sqlc: `SUM/COUNT` bungkus
    `COALESCE(...)::bigint`; hindari `AT TIME ZONE` di SELECT list (emit `interface{}`) — kembalikan
    timestamptz mentah, format di Go.
15. **`content-type: text/javascript` = eksekusi JS** (fitur Datastar, melengkapi #1).
    Body/`PatchElements` yang menyisipkan `<script>` tereksekusi. Kita BERSIH. Jangan set content-type
    itu, jangan patch `<script>` berisi data user. Aturan terkait: (a) escape SEMUA input user
    (`g.Text`, bukan `g.Raw`) — satu-satunya `g.Raw` = chart JSON `json.Marshal`; (b) signal
    user-modifiable → validasi backend; (c) jangan taruh data sensitif di signal.
16. **`sse.Redirect` (datastar-go) DIBLOKIR CSP — pakai HTTP 303.** `sse.Redirect`/`ExecuteScript`/
    `ReplaceURL` menyuntik `<script>...location.href</script>`; CSP (`script-src 'self' 'unsafe-eval'`,
    TANPA `unsafe-inline`) memblokir → redirect MATI (aksi sukses, halaman tetap). Terjadi
    logout/login/register. **Aturan**: aksi = NAVIGASI penuh → **native form POST →
    `http.Redirect(w,r,url,303)`** (commit cookie via scs normal → tak perlu ritual #2). Gagal
    validasi → PRG `?err=CODE` (`authErrMsg`). SSE tetap untuk update FRAGMENT parsial ter-escape.
    Menambah `unsafe-inline` BUKAN solusi.

## Mode tenancy: single → multi (ratchet) — keputusan 0006 & 0007

Template melayani DUA bentuk, **bukan dua jalur kode**. Lahir **single**, boleh **dinaikkan** ke
multi; turun **tak pernah**. Rasional di `docs/decisions/0006` & `0007`.

- **Single = multi-tenant N=1.** TEPAT SATU tenant; RLS/memberships/audit jalan apa adanya, yang
  hilang cuma chrome. Jangan buang `tenant_id` "karena toh satu" (jalur jarang-pakai pasti membusuk).
- **SATU bentuk URL kedua mode** (`/w/{slug}`); aplikasi tunggal di **`/w/app`** (`SingleAppPrefix`
  DIHAPUS). `wsPath`/`slugFromRequest` tanpa cabang mode.
- **Mode hidup di DATABASE** (`platform_settings.tenancy_mode`), bukan env (`APP_MODE` DIHAPUS).
  Nol baris = single. Penurunan ditolak DUA trigger: `BEFORE UPDATE` (multi→lain) DAN `BEFORE
  DELETE` (baris absen = single, hapus = penurunan menyamar).
- **Route SELALU didaftarkan**, tak bersyarat mode (mengubah 0006 §9). Penahan zona bahaya =
  `tenants.is_primary`, dijaga di handler DAN SQL (`AND NOT is_primary`).
- **Workspace PRIMER = rumah aplikasi** (`is_primary` + unique partial index). **Tak bisa
  diarsipkan/dihapus** (arsip = app read-only) & **tak memakan kuota** (kuota batasi yang DIBUAT).
  Pengecualian kuota WAJIB sama di sidebar & penegakan.
- **super_admin = OWNER workspace primer**, dipasang saat LOGIN (`ensurePrimaryOwner`, idempotent
  promote-only), BUKAN saat boot (belum ada `users`). **Jangan tentukan jalur RLS dari keanggotaan**
  — bypass dari ROLE. Cabut email dari `SUPER_ADMIN_EMAILS` → **owner biasa**, bukan user biasa.
- **Pendaftar mode single = `member`, BUKAN owner** (`placeNewUser`).
- **Wewenang single**: super_admin (env) fundamental; admin operasional (termasuk nama app,
  `canEditWorkspace` longgar untuk admin); member pakai.
- **`GuardSetRole` memakai `>=`**, bukan `>` (tanpa itu admin mengangkat sesama admin).
- **Boot** (`BootstrapPrimary`): workspace primer dibuat bila belum ada, mode dari DB.
- **Test WAJIB di kedua mode** untuk jalur bergantung-mode (`withMode` + `t.Cleanup` di
  `appmode_test.go`; mode = state paket, lupa dipulihkan meracuni test lain). `setupTest` menyetel
  MULTI eksplisit; default paket Single.
- **Menaikkan mode di `/dev/settings`** (kartu Mode Tenancy), gated `platform:settings`. Konfirmasi
  = MENGETIK nama aplikasi. Setelah naik form HILANG. Wajib ter-audit.

## Multi-tenancy (RLS + membership + role 2-bidang) — keputusan 0002, 0003 & 0004

Isolasi tenant ditegakkan **Postgres RLS**, bukan cuma `WHERE tenant_id`. Keanggotaan = model
**membership** (0003 supersede "1 user = 1 tenant"). Workspace aktif di **PATH** (`/w/{slug}`),
bukan session (0004 supersede 0003 #4).

- **URL workspace HANYA lewat `wsPath`/`wsRedirect`** (`internal/handler/wspath.go`) — satu-satunya
  tempat literal `/w/` boleh muncul (Rule 15). Slug kosong → `/workspace/new`, bukan `/w//x`.
- **`/admin` & `/user` TIDAK ADA** — dilebur `/w/{slug}` (membedakan ROLE bukan RESOURCE). **Beda
  role = beda AKSI di halaman sama, BUKAN beda ALAMAT.** Pembatasan di handler
  (`canEditWorkspace`/`canManageMembers`), bukan route.
- **Slug asing/tak dikenal → `http.NotFound`**, jangan 403 (konfirmasi ada) dan jangan redirect
  (tampilkan data workspace lain senyap — penyakit yang diobati 0004).
- **Role PLATFORM pun wajib mengikuti slug** — cabang platform di `Scope` bypass RLS tapi TETAP
  `adoptTenantBySlug` (tanpa itu super_admin buka `/w/acme/members` melihat anggota workspace lain).
  Bypass dari ROLE, tak pernah dari data.
- **`/dev`, `/notifications`, `/invite/{token}`, `/workspace/new` SENGAJA tanpa slug** — cakupan
  datanya bukan satu tenant. Path mengikuti cakupan data.
- **DUA pintu mengubah role harus SAMA.** `/w/{slug}/members` & `/dev/users` memanggil
  `UpdateMemberRole` + `authz.GuardSetRole` sama; keduanya WAJIB `h.notify(..., "member.role.changed")`.
  Di `/dev` tenant notifikasi = workspace **TARGET** (dari form), BUKAN aktor. **Status &
  soft-delete SENGAJA tanpa notifikasi** (menutup pintu login → kabar in-app tak terbaca).
- **Daftar panjang wajib JALAN ke halaman berikutnya, bukan cuma `LIMIT`.** Keyset dari halaman
  pertama terus = baris ke-21+ mustahil dijangkau (gagal senyap). Pola: ambil `pageSize+1` →
  `splitPage` (baris lebih = penanda "masih ada", tak dirender → hindari `COUNT`) → cursor `?after=`
  (`pagecursor.go`). **Keyset, bukan OFFSET** (`created_at DESC`, OFFSET menggeser antar-klik).
  Cursor rusak → halaman pertama, JANGAN kosong. Navigasi `<a>` biasa, bukan Datastar
  (bookmark/reload-able, lolos #16).
- **Daftar anggota HANYA untuk pengelola** (owner/admin/platform), kedua mode (0008, supersede
  0004 §3 — direktori PII). Gerbang di HANDLER (`canManageMembers` baris pertama `MembersPage`),
  bukan route. Ditolak **403 + penjelasan** BUKAN 404 (penerimanya terbukti anggota). Menu `Anggota`
  izin SAMA — `workspaceNav` terima dua izin terpisah (`canMembers`, `canSettings`) karena tak
  identik di single.
- **PII disamarkan di HANDLER, bukan view** (view menyamarkan → alamat ASLI tetap dioper, bocor di
  view-source). `maskEmail` dipertahankan (0008 §5). Domain DIPERTAHANKAN (pembeda orang
  dalam/luar); panjang lokal TIDAK dibocorkan; email sendiri utuh.
- **Penanda orang = NAMA (`users.name`), email cadangan.** Di-refresh tiap login (`UpdateUserProfile`,
  COALESCE: NULL = "provider diam" bukan "hapus"). USER-CONTROLLED → `oauth.NormalizeDisplayName`
  (buang kontrol/newline, ciutkan spasi, potong batas RUNE) & TAK PERNAH penanda unik/otorisasi
  (pakai `id`). **Kelak**: "edit profil" tertimpa refresh; nama pilihan harus kolom terpisah yang
  diutamakan, bukan matikan refresh (avatar tetap ikut berubah).
- **View TAK BOLEH merakit path workspace** — oper `base` dari handler (`panel.Members`,
  `panel.WorkspaceView.Base`). Pelanggaran senyap: ketahuan saat SUBMIT (pernah — aksi anggota
  menunjuk `/admin/*` yang 404). **Verifikasi UI harus mencakup submit.**

## Siklus hidup workspace (suspend · archive · delete) — keputusan 0005

`tenants.status` = `active|suspended|archived` + `deleted_at` (soft-delete, tenggang 30 hari).
Ditegakkan `gateLifecycle` (dipanggil `Scope`).

- **Bedanya KEWENANGAN**: `suspended` = tindakan PLATFORM (owner tak bisa batalkan sendiri);
  `archived` = keputusan OWNER (bisa dibuka tanpa memohon). Guard `status='active'` di SQL
  `ArchiveTenant` mencegah keluar dari suspensi lewat arsip→unarsip.
- **Kode status**: bukan-anggota → 404 · suspended → **403 + alasan** (anggota sah berhak tahu
  KENAPA) · archived → GET lolos, non-GET 403 · deleted → 404.
- **Platform SENGAJA menembus gerbang** (cabang platform di `Scope` tak panggil `gateLifecycle`) —
  merekalah yang menangguhkan. Dikunci `TestPlatformTembusSuspensi`.
- **Unarchive DI LUAR `/w/{slug}`** (`/workspace/{slug}/unarchive`): gerbang read-only blokir semua
  POST di dalam. Handler cari tenant sendiri — **wajib `tenantBySlug` untuk platform**, bukan
  `resolveTenantBySlug` (mensyaratkan keanggotaan → platform 404 padahal `isOwnerOf` mengizinkan).
- **`audit_logs.tenant_id` NULLABLE + `ON DELETE SET NULL`.** NOT NULL tanpa CASCADE → `DELETE FROM
  tenants` GAGAL di tengah setelah anak-anaknya CASCADE. Bukti tak boleh lenyap bersama yang dibuktikan.
- **Kuota**: terhapus TAK dihitung; terarsip TETAP dihitung (data masih disimpan).
- **Slug tak dilepas saat terhapus** (kalau dilepas, restore mustahil).
- **Purge TERJADWAL** (`internal/maintenance`, dipanggil `main.go`). Runner IN-PROCESS (bukan cron
  host). Dikunci `pg_try_advisory_lock` (id 4243, BEDA lock migrasi 4242) — `try` bukan blocking
  (instance kalah LEWAT). Purge SATU PER SATU. Ditunda 5 menit pasca-boot, berhenti saat SIGTERM.

## RLS, tx & role (0007)

- **`h.q(ctx)`, JANGAN `h.DB`** (dihapus). `h.q` ambil `*db.Queries` ber-tenant dari `Scope`. Lupa
  Scope = **panic keras** (bug wiring ketahuan seketika). Jalur pre-identity (auth/oauth/boot) pakai
  `db.WithSuper` eksplisit.
- **`WithTenant`/`WithSuper` = SATU tx, GUC `set_config(...,true)` TRANSACTION-LOCAL + `SET LOCAL
  ROLE app_rw`** (`internal/db/tenant.go`). `,true` wajib — plain `SET` bocor ke peminjam pool
  berikutnya (kebocoran tenant #1). Bypass dari **ROLE** (`isPlatformRole`), tak pernah dari data.
- **SATU DSN, hak diturunkan PER-TRANSAKSI (0007).** `FORCE` RLS wajib (tanpanya owner/superuser
  bypass senyap). `dropPrivileges` `SET LOCAL ROLE app_rw` di tx (pulih di COMMIT/ROLLBACK;
  `app.is_super` bypass → `/dev` utuh; DDL ditolak). **Migrasi WAJIB `GRANT app_rw TO CURRENT_USER`**
  (owner non-superuser tak otomatis anggota → `SET LOCAL ROLE` gagal). Bayar: injection bisa `RESET
  ROLE` — diterima karena semua query digenerate sqlc; **timbang ulang bila ada SQL mentah dari
  user.** Bonus: RLS mengikat di DEV (query lupa `WHERE tenant_id` gagal di laptop).
- **super_admin = ENV-ONLY, nol baris DB.** Role efektif di-overlay `RefreshIdentity` per-request
  (env → `platform_staff` → `memberships.role` workspace AKTIF). `platform_staff` TANPA RLS. Tak ada
  `PromoteSuperAdmins` (dihapus).
- **MEMBERSHIP: 1 user ↔ banyak workspace.** Role di `memberships` (user × tenant × role), BUKAN
  `users`. `users` = tabel **GLOBAL** (tanpa `tenant_id`/`role`, KELUAR dari RLS). `ListUsers` (/dev)
  lintas-workspace; anggota workspace pakai `ListMembersByTenant`.
- **`notifications` TANPA RLS — SENGAJA (semantik).** Notifikasi milik USER & lintas-workspace
  (undangan dari workspace yang BELUM miliknya). `tenant_id` cuma KONTEKS tampilan (nullable).
  **Jangan `h.q(ctx)`** — `db.WithSuper` + `WHERE user_id/email` sebagai penjaga, dengan test isolasi
  antar-user.
- **`memberships`/`invites` TANPA RLS — SENGAJA.** Dibaca untuk MENENTUKAN scope (chicken-and-egg),
  invite di jalur publik. Keamanan dari filter query / token rahasia.
- **`Scope` MEMVALIDASI keanggotaan** sebelum `WithTenant` (`resolveActiveTenant`): tenant session
  user-controlled. Tak valid → fallback workspace pertama; tanpa workspace → `/workspace/new`. HARUS
  di Scope, bukan RefreshIdentity.
- **Kuota workspace = DUA lapis** (`internal/settings`): default global `platform_settings` (diubah
  `/dev/settings`, SEKETIKA tanpa restart) + override per-user `users.workspace_quota` (**`NULL` =
  ikut global**, angka = hak khusus KEBAL global). `MAX_WORKSPACES_PER_USER` = **fallback** saat baris
  DB belum ada. Hitung HANYA lewat `settings.EffectiveWorkspaceQuota` (penegakan `WorkspaceCreate` &
  tampilan sidebar sumber sama). Yang dihitung: workspace ber-role **owner** & belum terhapus.
  `CountTenantOwners` mencegah owner terakhir diturunkan.
- **`/dev/settings` di-gate `platform:settings`, BUKAN `dev:users`.** Grup `/dev` dijaga `dev:users`
  (dimiliki staff) — tanpa objek Casbin tersendiri, staff bisa ubah aturan bagi SETIAP user. `devNav`
  juga cek izin ini. Deny-default; hanya super_admin lolos.
- **Cache settings = PER-PROSES.** Seketika di instance yang melayani; lain menyusul saat boot. Kalau
  kelak butuh serempak: pub/sub Redis, bukan hapus cache.
- **Register/OAuth = user + workspace + membership owner** dalam SATU tx `WithSuper`.
  `startIdentity(preferTenant)` memilih workspace aktif.
- **Audit di tx `WithSuper` TERPISAH** dari Scope tx (fail-soft struktural). `tenant_id` audit =
  tenant aktor.
- **Test isolasi RLS** (`rls_test.go`) konek `app_rw` non-superuser via `SET ROLE` di `AfterConnect`
  (membuktikan RLS mengikat). Test handler lain konek superuser (RLS bypass diam-diam). Seed pakai
  owner-pool.
- **Casbin CSV TAK dukung komentar inline** di akhir `g,`/`p,` (jadi bagian nilai → link mati).
  Komentar di baris `#` tersendiri.

## Client-side JS (CSP-safe)

Datastar bukan untuk manipulasi tabel (filter/paginate/copy), zoom, chart, atau state UI persisten
(collapse sidebar, tema). Untuk itu file terpisah `static/` (`sidebar.js`, `theme.js`, `health.js`,
`erd.js`, `charts.js`) — same-origin lolos `script-src 'self'`, **bukan inline**. Muat `<script src>`,
SINKRON bila perlu set state sebelum paint (no-FOUC): `sidebar.js`/`theme.js` set atribut `<html>`
sebelum body render. Data untuk JS ditanam `<script type="application/json">` (CSP-safe), JS `JSON.parse`.

## Aset vendored

`static/` = aset vendored (datastar.js, daisyui.js, mermaid.min.js, echarts.min.js) — checksum di
`static/VENDOR.md`. `daisyui.js` = plugin Tailwind (`@plugin` dari input.css, dikonsumsi `make css`,
bukan dimuat browser). Tailwind CLI di-download `make setup` (gitignored, verifikasi integritas).
No-CDN: semua di-embed. Aset raksasa/minified dikecualikan dari ripgrep via `.ignore` (tetap
ter-commit & ter-embed).

## Konfigurasi produksi: gagal keras, jangan andalkan ingatan

Prinsip: **yang berbahaya bila salah harus menggagalkan boot; yang bisa diturunkan otomatis jangan
diminta ke manusia.** Warning di log diabaikan — bukan pengaman.

- **Gagallah dengan PETUNJUK** (`internal/preflight`, dipanggil awal `run()` DAN `make doctor` —
  sumber SAMA). `database "x" does not exist` benar tapi menyembunyikan yang dibutuhkan: nama dicari,
  database MIRIP yang ada (salah ketik nyaris selalu beda tipis — `-` vs `_`, `_test` tertinggal, nama
  pra-`make rename`), perintah persis membereskannya. Tiap `Problem` WAJIB punya `Fix`. Semua masalah
  dikumpulkan sekaligus.
- **Database dibuat otomatis HANYA di dev** (`AutoCreateDB: !cfg.IsProduction()`). Di produksi ini
  mengubah DSN salah ketik jadi database KOSONG yang tampak sehat. `make doctor` pun tak pernah
  membuatnya (alat diagnosis tak boleh mengubah keadaan).
- **Isolasi tenant DIBUKTIKAN, bukan dijanjikan** (`db.CheckRLSTx`, `verifyTenantIsolation` di
  `main.go`). Diperiksa DI DALAM `WithSuper` — tx yang sudah menurunkan haknya (memeriksa pool
  telanjang menjawab pertanyaan SALAH: di sana masih owner). Tanya Postgres pasti:
  `rolsuper`/`rolbypassrls`/pemilik-tabel + `FORCE RLS`. Berlaku sama dev & produksi. Sejarah: dulu
  sekadar "env `APP_DATABASE_URL` terisi", mengisinya DSN SAMA seperti `DATABASE_URL` lolos sambil
  bocor (82 baris/15 tenant).
- **Produksi tak butuh persiapan role manual.** Migrasi membuat `app_rw` + GRANT + `ALTER DEFAULT
  PRIVILEGES` + `GRANT app_rw TO CURRENT_USER`. Rolenya `NOLOGIN` (tak ada password → tak ada `ALTER
  ROLE ... LOGIN` / entri PgBouncer `userlist.txt`). Kolom `GENERATED ALWAYS AS IDENTITY` tak menuntut
  GRANT sequence terpisah.
- **`SESSION_KEY` divalidasi PANJANGNYA** (min 32, `config.MinSessionKeyLen`), bukan cuma keberadaannya
  (kunci lemah lebih berbahaya dari kosong).
- **`SESSION_KEY` benar-benar DIPAKAI**: menurunkan nama cookie sesi (`Config.SessionCookieName`) agar
  dua deployment di host sama tak saling menimpa. Dipakai HASH-nya (kuncinya sendiri di nama cookie =
  bocor). Dulu divalidasi tapi nol pemakai (konfigurasi yang berbohong).
- **`Cookie.Secure` diturunkan dari `ENV=production`**, bukan env sendiri. Konsekuensi sengaja:
  produksi tanpa HTTPS = login tak berfungsi (gagal keras, bukan bocor senyap).
- **Jangan tambah env baru untuk hal yang bisa diturunkan** dari env yang ada.

## MCP server read-only (`internal/mcpserver`) — akses runtime untuk agent AI

Memberi agent AI "mata" ke runtime dev/staging/prod — SEMUA read-only. SDK
`modelcontextprotocol/go-sdk` v1.7.0.

- **ADAPTER TIPIS, bukan pintu baru.** Tiap tool memanggil ULANG fungsi baca yang aman
  (`erd.Introspect`, `db.CheckRLS`, `preflight.Run`, `List*`/`Count*`/`Presence*`), tak menulis
  logika/query baru. DILARANG: tool SQL/shell mentah, embed `maintenance`/`MigrateWithLock`/`Create*`,
  mengalirkan nilai rahasia (lapor keberadaan/panjang saja — Rule 7/12).
- **`platform_stats` menyaring settings lewat ALLOWLIST** (`exposedSettings` di `tools_health.go`),
  bukan seluruh `platform_settings`. Sebab: key baru (SMTP, secret webhook) tak boleh bocor diam-diam;
  dengan allowlist, key baru TAK muncul sampai sengaja didaftarkan. JANGAN kembalikan ke "ekspos
  semua". Dikunci `TestPlatformStats_AllowlistMenyaringKeySensitif`.
- **Read-only STRUKTURAL, bukan disiplin.** Semua DB lewat `db.WithSuper` → `SET LOCAL ROLE app_rw` →
  DDL ditolak DB. `h.q(ctx)` TAK BISA (panic tanpa Scope). Test `TestReadOnly_NolTulis` hitung baris
  sebelum/sesudah = harus SAMA.
- **BUKAN service/proses terpisah.** `mcpserver.Handler()` = `http.Handler` biasa, rute `/mcp` di app
  yang sudah jalan (Streamable HTTP, Stateless+JSONResponse). Image/deploy SAMA (refleks "MCP = binary
  terpisah" benar untuk stdio, SALAH untuk HTTP remote).
- **Rute opt-in, dijaga Bearer.** `routes.go` daftar `/mcp` HANYA bila `cfg.MCPToken != ""`. Dijaga
  `mw.RequireBearer` (constant-time), BUKAN RequireAuth (klien agent/program). Sejajar `/healthz`, di
  luar auth sesi.
- **Satu server, dua transport.** `build()` merakit `*mcp.Server` sekali; dipakai HTTP (`Handler`) &
  stdio (`ServeStdio`, `./app mcp`). Di stdio, logger WAJIB ke STDERR (stdout milik JSON-RPC).
- **Fase berikutnya (belum ada):** tool tulis terjaga (migrasi/purge) dev+staging saja, gated
  `!IsProduction()`, tiap aksi → `audit_logs` (aktor "agent"). Produksi read-only.

## Batasan

- Semua interaktivitas = **Datastar** (satu paradigma). Jangan tambah framework JS.
- Single-binary — jangan tambah dependency yang butuh runtime kedua / Node.
- Password auth = dev-only; jangan aktifkan di produksi.
- **`unsafe-eval` = keputusan sadar, BUKAN bug** (riset 2026): Datastar `new Function()` ekspresi
  `data-*` di browser — nonce tak bisa menggantikannya. Menghapusnya butuh ganti runtime (fork dataSPA
  rapuh, atau rombak ke `<form>`+POST buang SSE patch). Risiko rendah: ekspresi cuma ID integer +
  literal internal, tak pernah input user. Jangan "perbaiki" dengan `unsafe-inline` atau tukar runtime
  tanpa diskusi. Footgun authoring sudah ditutup helper `dsx.go` (gotcha #5–6).
