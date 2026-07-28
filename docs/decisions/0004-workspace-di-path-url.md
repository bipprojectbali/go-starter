# 0004 — Workspace aktif di PATH URL, bukan di session

Status: **Diterima** (2026-07-28) — men-supersede pemilihan-workspace-via-session di
[0003](0003-membership-multi-workspace.md) keputusan #4 (validasi keanggotaannya TETAP)

## Konteks

0003 memberi satu user banyak workspace, tapi **workspace aktif disimpan di session**
(`session.SetActiveTenant`, dibaca `resolveActiveTenant` di `internal/handler/scope.go`).
URL tak menyebut workspace sama sekali: `/admin/members` berarti *"anggota dari
workspace apa pun yang kebetulan aktif di cookie-mu"*.

Tiga akibat yang bukan teoretis:

1. **Tautan tak bisa dibagikan.** Kirim `/admin/members` ke rekan → dia melihat
   workspace DIA. Tanpa error, hanya data yang salah — kelas bug paling buruk: senyap.
2. **Dua tab saling merusak.** Session satu per browser: buka A di tab 1, pindah ke B
   di tab 2, refresh tab 1 → sekarang B. Pasti terjadi, bukan edge case.
3. **Bookmark tak stabil** — menunjuk "yang terakhir aktif", bukan tempat tertentu.

Ketiganya berakar pada satu hal: **URL bukan state yang lengkap.**

Masalah kedua, terpisah: `authz.HomePath` (`internal/authz/role.go:76`) memetakan
**role → path** (`/dev`, `/admin`, `/user`). Di model membership, orang yang sama
owner di A dan member di B — jadi **alamat halaman yang sama berubah saat pindah
workspace**. `/admin` vs `/user` mengkodekan ROLE ke dalam URL, padahal role itu
per-workspace dan berubah-ubah. Yang stabil adalah RESOURCE.

## Keputusan

### 1. Workspace hidup di path, dengan prefix `/w/`

```
/w/{slug}/            beranda workspace
/w/{slug}/members     anggota
/w/{slug}/settings    pengaturan
```

**Slug, bukan ID.** Kolom `tenants.slug TEXT NOT NULL UNIQUE` sudah ada sejak
migrasi 00007 dan `GetTenantBySlug` sudah ada di `queries/tenants.sql` — keduanya
menganggur. ID numerik **enumerable**: `/w/1`, `/w/2`, … menyapu berapa banyak
tenant yang ada (kebocoran bisnis, bukan kebocoran data) dan tak terbaca manusia.

**Prefix `/w/` wajib, bukan `/{slug}/` telanjang.** Ini menghapus seluruh kelas
masalah *reserved slug*: tanpa prefix, workspace bernama `login`, `dev`, atau
`static` menabrak route sistem, dan setiap route baru berisiko menabrak slug
pelanggan yang sudah ada. Dengan `/w/`, tabrakan **mustahil secara struktural** —
tak perlu daftar kata terlarang yang harus dijaga selamanya. `/w/` (bukan
`/workspace/`) karena prefix ini muncul di tiap URL.

### 2. `/admin` dan `/user` DILEBUR jadi `/w/{slug}/`

Keduanya adalah pembedaan **role**, bukan **resource** — halaman anggotanya sama,
yang berbeda hanya aksi yang boleh dilakukan. Setelah dilebur, pindah workspace
tak lagi mengubah alamat; yang berubah hanya apa yang bisa kamu tekan di sana.

`HomePath` untuk role tenant menjadi `/w/{slug}/` (butuh slug workspace aktif);
`/dev` tetap untuk role platform.

### 3. Hierarki akses: SATU alamat, aksi mengikuti role

Ini menjawab "bagaimana member mengakses workspace". Route sama untuk semua
anggota; `RequireEnforce` menjaga pintu, handler menjaga aksi (pola `canEditWorkspace`
/`canManageMembers` yang sudah dipakai hari ini):

| Path | member | admin | owner |
|---|---|---|---|
| `/w/{slug}/` | lihat | lihat | lihat |
| `/w/{slug}/members` | lihat daftar | + undang, ubah role, keluarkan | + atur owner |
| `/w/{slug}/settings` | **403** | lihat (read-only) | + ganti nama |

Aturannya: **beda role = beda AKSI di halaman yang sama, bukan beda ALAMAT.**
Sebab satu orang bisa admin di A dan member di B — kalau alamat ikut berubah,
tautan yang dia simpan rusak begitu dia pindah workspace.

Guard hierarki yang ada (`authz.Role` ordinal: `member < admin < owner`, dan
`guard.go` yang melindungi owner terakhir) tidak berubah sama sekali.

### 4. TIGA jenis route yang SENGAJA di luar `/w/{slug}`

Ini bagian yang paling mudah salah dan paling menentukan:

| Route | Alasan tetap di luar |
|---|---|
| `/dev/*` | Panel **platform**. `ListUsers` lintas-workspace (0003 §konsekuensi). Menaruhnya di bawah satu slug = berbohong soal cakupan data |
| `/notifications` | Milik **user**, lintas-workspace. Undangan justru datang dari workspace yang BELUM jadi miliknya |
| `/invite/{token}` | **Publik** — penerima belum jadi anggota |
| `/workspace/new` | Justru untuk yang **belum punya** workspace |

Alasannya sama dengan alasan `notifications` sengaja tanpa RLS: cakupannya bukan
satu tenant. Path harus mencerminkan cakupan data, bukan melawannya.

### 5. Slug tak dikenal / bukan miliknya → **404**, bukan redirect

`Scope` mengambil tenant dari path bila ada, jatuh ke session bila tidak (`/dev`,
`/notifications`). Validasi keanggotaan yang sudah ada (`resolveActiveTenant`)
tetap jadi penjaga tunggal.

**404, bukan 403 dan bukan redirect.** 403 mengonfirmasi workspace itu ada
(kebocoran keberadaan); redirect ke workspace lain menampilkan data yang salah
secara senyap — persis penyakit yang sedang diobati ADR ini. Pola ini sama dengan
`WorkspaceSwitch` yang hari ini sengaja diam ke `/` saat bukan anggota.

### 6. Role PLATFORM juga wajib mengikuti slug (ditemukan saat implementasi)

Cabang platform di `Scope` bypass RLS dan tak butuh membership — tapi ia tetap
harus menerjemahkan slug. Handler membaca workspace dari `session.TenantID`, jadi
tanpa ini super_admin yang membuka `/w/acme/members` melihat anggota workspace
**lain** (yang kebetulan aktif di session-nya) di bawah URL yang menjanjikan
`acme`: salah data, senyap, dan justru di panel paling berwenang.

`adoptTenantBySlug` sengaja dipisah dari `resolveTenantBySlug` agar bedanya
terbaca: yang pertama TANPA cek membership, dan keputusan itu diambil dari ROLE —
tak pernah dari data DB (anti privilege-escalation, sama seperti `WithSuper`).
Slug tak dikenal tetap 404: bypass RLS bukan bypass 404.

### 7. Session tetap ada, turun pangkat jadi *petunjuk*

Session tak dihapus: masih dipakai untuk memilih workspace saat user membuka `/`
atau baru login (ke mana harus diantar). Bedanya, ia bukan lagi **sumber
kebenaran** — begitu ada slug di path, path yang menang.

## Alternatif ditolak

- **Subdomain `acme.app.com`** — butuh wildcard DNS + wildcard SSL, dan Google OAuth
  mewajibkan whitelist domain sehingga tiap tenant baru perlu pendaftaran manual.
  Melanggar batasan single-binary tanpa infrastruktur tambahan. Shortcut (eks
  Clubhouse) justru **pindah dari** subdomain ke path persis karena user yang
  tergabung di banyak organisasi dan integrasi pihak ketiga.
- **Pertahankan `/admin`/`/user` di bawah slug** (`/w/acme/admin/members`) — lebih
  mekanis, tapi mengabadikan role di URL: alamat halaman yang sama berubah saat
  pindah workspace (§2).
- **`/{slug}/` tanpa prefix** — lebih pendek, tapi menuntut daftar kata terlarang
  permanen dan tiap route baru berisiko menabrak slug pelanggan (§1).
- **ID numerik** — enumerable, tak terbaca (§1).
- **Tetap session-based** — masuk akal HANYA bila turunan template dipastikan
  single-workspace. Template ini justru dibuat untuk multi-workspace (0003).

## Konsekuensi

- **Setiap link internal berhenti bisa di-hardcode.** `"/admin/members"` menjadi
  `wsPath(slug, "/members")` — satu helper, agar tak ada literal tersebar (Rule 15).
  Terdampak: `AppShell` nav & quickLinks, `HomePath`, semua redirect handler, test.
  Perkiraan **8–12 file**.
- **`routes.go` makin jauh dari batas 150 baris** (kini 163). Grup `/w/{slug}` layak
  dipecah ke fungsi terpisah — tetap di file yang sama agar tetap single source of truth.
- **URL lama mati.** Karena belum ada pemakai produksi, TIDAK dibuat redirect
  kompatibilitas — biayanya tak sepadan. Bila sudah ada pemakai, ini keputusan berbeda.
- **`/dev` tetap tanpa slug** → chip PLATFORM & pembacaan cakupan data lintas-workspace
  tidak berubah (`internal/ui/panelkind.go`).
- **`panelOf(ctx, path)` perlu menyesuaikan**: `/w/…` → panel berdasarkan ROLE di
  workspace itu, sumber yang sama dengan `navFor` (chip tak boleh bertentangan dengan menu).
- **Slug immutable** (sudah begitu hari ini: `TenantUpdate` sengaja hanya mengganti
  nama tampilan). Slug berubah = semua tautan tersimpan mati. Kalau nanti perlu,
  simpan slug lama sebagai alias — bukan rename.
- **Waktunya sekarang, bukan nanti**: template ini di-clone. Mengubah bentuk URL
  setelah ada turunan berarti mematahkan bookmark tiap turunan sekaligus.
  Sekarang 12 file; nanti per-project, selamanya.
