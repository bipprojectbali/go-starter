# 0006 — Mode aplikasi: single-app vs multi-tenant (`APP_MODE`)

Status: **Sebagian di-supersede** (2026-07-30) oleh
[0007](0007-satu-dsn-dan-ratchet-tenancy.md) — melengkapi
[0004](0004-workspace-di-path-url.md) (workspace di path) &
[0003](0003-membership-multi-workspace.md) (membership)

> **Yang sudah tidak berlaku** (lihat 0007):
> - **Bentuk URL per-mode.** `/app/...` dihapus; kedua mode memakai `/w/{slug}`,
>   dengan workspace primer ber-slug `app`. Alasannya: menaikkan mode dulu
>   mengubah setiap alamat yang sudah tersebar.
> - **`APP_MODE` dari env.** Mode pindah ke `platform_settings.tenancy_mode`
>   dengan trigger yang menolak penurunan. Env bisa dibalik; ratchet DB tidak.
> - **Route mode-lain tak didaftarkan** (§9). Seluruh route kini selalu
>   terdaftar; yang menahan zona bahaya adalah `tenants.is_primary`, dijaga di
>   handler dan di SQL. Route bersyarat telanjur salah begitu mode bisa naik
>   saat jalan.
> - **"Di mode single tak ada owner"** (§7). Sejak 0007 workspace primer punya
>   owner: super_admin. Kelonggaran `canEditWorkspace` untuk admin TETAP ada,
>   tapi alasannya kini bahwa admin adalah pembantu operasional.
>
> Yang MASIH berlaku: mode single = multi-tenant dengan N=1 (bukan jalur kode
> kedua), pendaftar di mode single jadi `member` bukan owner, `GuardSetRole`
> memakai `>=`, dan `wsPath` sebagai satu-satunya tempat bentuk URL diputuskan.

## Konteks

Template ini di-clone untuk banyak project turunan, dan tidak semuanya
multi-tenant. Sebagian hanya butuh **satu aplikasi** dengan banyak user —
tanpa konsep workspace sama sekali di mata pemakai. Hari ini turunan seperti itu
harus membuang kode tenancy sendiri: pekerjaan berulang, dan tiap turunan
membuangnya dengan cara berbeda.

Menambahkannya sekarang jauh lebih murah daripada nanti. Alasannya bukan
kebetulan: **0004 sudah memusatkan pembentukan URL di `wsPath()`** —
10 pemanggil, tak satu pun handler menulis `/w/` sendiri. Sebelum 0004,
keputusan ini akan menyentuh puluhan tempat.

## Keputusan

### 1. Single-app = multi-tenant dengan N=1, BUKAN jalur kode kedua

Dalam mode single, database tetap punya **tepat satu tenant**. RLS, memberships,
audit, notifikasi — semuanya berjalan apa adanya. Yang disembunyikan hanya
*chrome*-nya: switcher workspace, tombol buat-baru, kuota.

Godaan yang DITOLAK: "single app tak butuh tenant_id, buang saja". Itu melahirkan
aplikasi kedua di dalam satu repo — tiap fitur baru ditulis dua kali, dan salah
satu jalur pasti membusuk karena lebih jarang dipakai. Dengan N=1, ada satu jalur
yang sama-sama teruji, dan pindah single→multi kelak hanya ganti env.

### 2. `APP_MODE=single|multi`, default `multi`

Default `multi` = perilaku template hari ini tidak berubah bagi yang tak mengisi
env. Nama `APP_MODE` (bukan `MULTI_TENANT=false`) karena tenancy tidak pernah
benar-benar dimatikan — di dalam tetap ada satu tenant. Nama yang berbohong pada
pembaca berikutnya adalah hutang yang dibayar berkali-kali.

### 3. Prefix path ditentukan mode — SATU fungsi

```
multi  → /w/{slug}/members   (wsPath("acme", "/members"))
single → /app/members        (wsPath(_, "/members"))
```

Perubahannya terkurung di `internal/handler/wspath.go`. Handler, view, RLS,
audit: **nol perubahan** — semuanya sudah lewat `wsPath()` dan
`session.TenantID(ctx)`.

Peleburan 0004 (`/admin`+`/user` → satu alamat, aksi mengikuti role) ikut
terbawa: di mode single, `/app` melayani semua role dengan aturan yang sama.
Itu menjawab "user biasa tetap punya menunya" tanpa kode tambahan.

### 4. Route kedua mode TIDAK PERNAH hidup bersamaan

`registerWorkspaceRoutes` mendaftarkan `/w/{workspace}` ATAU `/app`, tak pernah
keduanya. Kalau keduanya terdaftar, ada dua URL untuk halaman yang sama — test
lulus lewat satu jalur sementara jalur lain rusak diam-diam.

Route yang khusus multi (`/workspace/new`, `/workspace/switch`,
`/dev/workspaces`) **tak didaftarkan** di mode single — bukan sekadar menunya
disembunyikan. Menu tersembunyi + route hidup = pintu belakang.

### 5. Bootstrap: tenant tunggal dibuat saat STARTUP

Bila mode single dan belum ada tenant, app membuatnya dari `APP_NAME`
(slug `app`). Deterministik: aplikasi tak pernah berada di keadaan "belum ada
workspace", sehingga tak perlu jalur khusus untuk user pertama — dan keadaan
paling jarang diuji adalah keadaan yang paling sering rusak.

### 6. Register/OAuth di mode single: GABUNG, bukan buat baru — dan BUKAN owner

Ini konsekuensi yang paling mudah terlewat. Hari ini register & OAuth SELALU
`CreateTenant` + `CreateMembership(owner)` (`auth.go`, `oauth_google.go`). Di
mode single itu berarti **setiap orang yang mendaftar jadi owner aplikasi** —
celah nyata, bukan detail kosmetik.

Di mode single:

- pendaftar bergabung ke tenant tunggal, **tak ada tenant baru dibuat**;
- role default = **`member`**, bukan `owner`;
- pengelola = `SUPER_ADMIN_EMAILS` (env, sudah ada) — konsisten dengan model
  0002: otoritas tertinggi datang dari env, tak pernah dari pendaftaran.

Menaikkan seseorang jadi `owner`/`admin` tetap lewat panel, seperti biasa.

### 7. Wewenang di mode single: dua SUMBU, bukan satu tangga

Mode single punya tiga peran, dan batas antar-mereka bukan "boleh sedikit lebih
banyak" melainkan **sifat tindakannya**:

| Sumbu | Isi | super_admin | admin | member |
|---|---|---|---|---|
| Fundamental | mode app, kuota, hapus/arsip app, **angkat admin** | ✅ | ❌ | ❌ |
| Operasional | nama app, kelola anggota, angkat/turunkan member | ✅ | ✅ | ❌ |
| Pakai | membuka aplikasi | ✅ | ✅ | ✅ |

Pembagian ini tidak bergantung pada tangga otoritas, melainkan pada: **bisa
dibatalkan atau tidak**, dan **memengaruhi satu orang atau seluruh aplikasi**.
Admin adalah pembantu super_admin — ia menjalankan aplikasi sehari-hari tanpa
bisa mengubah dasarnya.

`owner` TETAP ADA di kode, tapi tak pernah diberikan di mode single. Menghapus
satu anak tangga dari `authz.Role` (ordinal, dipakai `GuardSetRole` untuk
membandingkan otoritas) berarti dua tangga berbeda dan tiap guard harus tahu
sedang di mode apa. Membiarkannya kosong = satu jalur kode melayani keduanya
(prinsip N=1 §1). Praktisnya: dropdown role di mode single hanya menawarkan
`member` dan `admin`.

Konsekuensi yang harus disadari: karena super_admin **hanya** dari env,
menambahnya berarti edit `.env` + restart. Konsisten dengan 0002 (env-only = tak
bisa dieskalasi lewat aplikasi), tapi lebih terasa di mode single karena tak ada
owner sebagai perantara.

### 8. `GuardSetRole`: `>=`, bukan `>` (celah ditemukan saat merancang ini)

`guard.go` dulu menolak `newRole > actorRole` — sehingga **admin bisa mengangkat
member jadi admin**: sesama admin, otoritas setara, lolos.

Itu memperbanyak dirinya sendiri dan mencabut kendali atasan atas siapa yang
memegang wewenang. Di multi-tenant hal ini tersamar karena masih ada owner di
atas admin. Di mode single, admin adalah jabatan tertinggi yang bisa DIANGKAT —
jadi satu admin bisa melahirkan semuanya.

Diperbaiki jadi `>=`: yang menentukan siapa jadi admin haruslah pihak DI ATAS
admin. Berlaku di KEDUA mode — di multi pun, owner-lah yang seharusnya
menentukan admin, bukan sesama admin.

### 9. Zona bahaya TIDAK ADA di mode single

Route `/app/archive` & `/app/delete` **tak didaftarkan** sama sekali — bukan
sekadar tombolnya disembunyikan. Menghapus "workspace" di mode single berarti
menghapus SELURUH aplikasi: tak ada tempat untuk kembali, dan tak ada gunanya
(menutup app sementara = suspend lewat `/dev`).

### 10. Tenant "default" bawaan migrasi diadopsi (ditemukan saat verifikasi)

Migrasi 00007 membuat tenant `default` sebagai wadah backfill, jadi **setiap
database baru sudah punya tepat satu tenant dengan slug yang salah** — dan mode
single menolak start sebelum sempat dipakai sekali pun. Bug yang hanya muncul
saat menjalankan aplikasi sungguhan di DB bersih, tak tertangkap unit test.

Bootstrap kini mengadopsinya: slug diubah ke `app` + nama dari `APP_NAME`, TAPI
**hanya bila workspace itu masih kosong** (nol anggota). Yang sudah berisi orang
tidak: mengubah slug-nya mematikan setiap tautan yang sudah tersebar (alasan slug
immutable sejak 0004), jadi di situ operator yang memutuskan.

### 11. Membalik mode pada DB terisi = GAGAL KERAS saat startup

`APP_MODE=single` sementara ada >1 tenant → app **menolak start** dengan pesan
yang menyebut jumlahnya. Memilih diam-diam salah satu berarti tenant lain lenyap
dari pandangan tanpa jejak — kehilangan data yang terlihat seperti bug UI.

Arah sebaliknya (single→multi) **aman**: tenant tunggal menjadi workspace pertama
dari banyak. Tak perlu guard.

## Alternatif ditolak

- **Dua jalur kode terpisah** (§1) — dua aplikasi dalam satu repo.
- **Membuang tenant_id di mode single** — menuntut skema & query kedua; RLS,
  audit, notifikasi semuanya bercabang.
- **`MULTI_TENANT=false`** (§2) — nama yang menyesatkan.
- **Menghapus `owner` dari `authz.Role` di mode single** — dua tangga ordinal
  berbeda; tiap guard harus tahu mode. Dibiarkan ada tapi tak diberikan (§7).
- **`/{slug}` vs `/` telanjang untuk single** — akar (`/`) sudah dipakai landing,
  dan tanpa prefix, tabrakan route-vs-konten kembali jadi masalah yang 0004
  hapus secara struktural.
- **Mode dibaca dari DB** — mode menentukan bentuk ROUTE, dan route didaftarkan
  sekali saat startup. Membacanya dari DB berarti route bisa berubah saat jalan:
  sumber kebingungan tanpa manfaat.

## Konsekuensi

- `wsPath()` bergantung state global mode (di-inject saat startup, pola
  `SetDevMode`/`SetCSSPath` yang sudah ada). Test wajib mengembalikan mode
  semula lewat `t.Cleanup`.
- **Test dijalankan di KEDUA mode** untuk jalur yang membentuk path — kalau
  hanya satu mode diuji, yang lain rusak diam-diam. Ini biaya nyata yang
  dibayar terus-menerus; tanpanya fitur ini justru menurunkan kualitas.
- `/dev/workspaces` (0005) hilang di mode single — tapi **suspend/archive tetap
  ada** lewat panel sebagai maintenance mode: satu-satunya cara menutup aplikasi
  sementara tanpa restart.
- Kuota workspace (0006/`platform_settings`) tak relevan di mode single —
  kartunya disembunyikan, nilainya tetap tersimpan (pindah ke multi kelak
  langsung berlaku).
- Dokumentasi turunan: README perlu menyebut `APP_MODE` sebagai keputusan
  PERTAMA saat clone — mengubahnya setelah ada data adalah operasi berisiko (§7).
