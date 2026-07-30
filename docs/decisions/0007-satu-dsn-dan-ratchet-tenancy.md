# 0007 — Satu DSN, satu bentuk URL, dan mode tenancy sebagai ratchet

Status: **Diterima** (2026-07-30) — men-supersede sebagian
[0006](0006-mode-aplikasi-single-vs-multi.md) (bentuk URL per-mode, `APP_MODE`
dari env) dan menyempurnakan [0002](0002-multi-tenancy-rls-role-2-bidang.md)
(dual-DSN)

## Konteks

Tiga keputusan sebelumnya masing-masing masuk akal saat dibuat, tapi bersama-sama
menumpuk beban yang harus DIINGAT manusia:

1. **Dual-DSN** (0002). Isolasi tenant ditegakkan RLS, dan RLS tak mengikat
   pemilik tabel maupun superuser — bahkan dengan `FORCE`. Maka runtime harus
   konek sebagai role terbatas (`APP_DATABASE_URL`), terpisah dari owner yang
   menjalankan migrasi (`DATABASE_URL`).

2. **Bentuk URL per-mode** (0006). Mode single memakai `/app/...`, multi memakai
   `/w/{slug}/...`.

3. **`APP_MODE` di env** (0006). Bentuk aplikasi ditentukan variabel lingkungan.

Yang ketiganya punya bersama: **konfigurasi yang harus benar, tanpa cara
memastikannya benar selain mengingat.** Dan masing-masing punya mode kegagalan
yang senyap:

- `APP_DATABASE_URL` diisi dengan DSN yang **sama** seperti `DATABASE_URL`
  ("biar aman, samakan saja") melewati setiap pemeriksaan env sambil tetap
  menjalankan pool sebagai owner. Terukur di database nyata: query yang lupa
  `WHERE tenant_id` membaca **82 baris dari 15 tenant** — sementara orangnya
  merasa sudah mengamankan.
- Menaikkan aplikasi dari single ke multi mengubah **setiap alamat** yang sudah
  tersebar: bookmark, tautan di email, dokumentasi turunan.
- `APP_MODE` bisa **dibalik**. Menurunkannya kembali ke `single` setelah ada
  banyak workspace menyembunyikan sisanya — kehilangan data yang tampak seperti
  bug UI. Perlindungannya berupa pemeriksaan saat boot: aturan yang harus
  diingat dan dijalankan dengan benar.

Pemicu peninjauan: pertanyaan "apakah `APP_DATABASE_URL` masih diperlukan?",
disusul kesepakatan bahwa **setiap aplikasi dimulai sebagai single dan boleh
dinaikkan ke multi, tapi tak pernah diturunkan**.

## Keputusan

### 1. Satu DSN. Hak diturunkan PER-TRANSAKSI, bukan per-koneksi

`WithTenant`/`WithSuper` menjalankan `SET LOCAL ROLE app_rw` di dalam transaksi
yang sudah mereka buka.

Terverifikasi di Postgres 17 (`internal/db/privileges_test.go`):

| Sifat | Hasil |
|---|---|
| `current_user` di dalam tx | `app_rw` |
| `rolsuper` di dalam tx | `false` — ikut tercabut |
| Setelah `COMMIT` | pulih ke owner |
| Setelah `ROLLBACK` | pulih ke owner |
| Transaksi berikutnya, koneksi sama | owner — **tak bocor ke peminjam pool** |
| `app.is_super='on'` | tetap bypass — jalur `/dev` utuh |
| `CREATE TABLE` sebagai app_rw | ditolak |
| Biaya | ~5 µs per transaksi |

Yang lenyap: env `APP_DATABASE_URL`, password `app_rw` (rolenya tetap `NOLOGIN`
— **tak ada kredensial untuk dilupakan atau bocor**), `ALTER ROLE ... LOGIN` di
produksi, entri `userlist.txt` PgBouncer, pool kedua, dan logika fallback config.

Yang didapat: **RLS mengikat di dev juga.** Sebelumnya dev jalan sebagai owner,
jadi query yang lupa `WHERE tenant_id` baru ketahuan di produksi. Sekarang gagal
di laptop.

**Yang dibayar** — ini pengurangan defense-in-depth yang sungguhan, bukan gratis:
dengan koneksi `app_rw` sungguhan, SQL injection pun tak bisa naik hak. Dengan
`SET LOCAL ROLE`, injection yang berhasil bisa memanggil `RESET ROLE`. Diterima
karena seluruh query digenerate sqlc (berparameter) dan tak ada jalur SQL mentah
dari input user. **Kalau kelak ada, timbang ulang keputusan ini.**

Catatan yang menguatkan: pendekatan ini kebal masalah yang justru menjerat
`SET ROLE` biasa di PgBouncer transaction pooling — di sana session state tak
bertahan antar-transaksi, dan gejalanya adalah satu tenant membaca baris tenant
lain di bawah beban. `SET LOCAL` adalah obat yang direkomendasikan untuk itu.

Migrasi wajib memuat `GRANT app_rw TO CURRENT_USER`: owner non-superuser tak
otomatis anggota `app_rw`, dan `SET LOCAL ROLE`-nya akan gagal di setiap
transaksi.

### 2. Satu bentuk URL. Mode single memakai `/w/app`

`SingleAppPrefix` dihapus. Kedua mode memakai `/w/{slug}`; workspace primer
ber-slug tetap `app`.

Konsekuensinya berantai:

- `wsPath` kehilangan cabang mode seluruhnya.
- `slugFromRequest` tak perlu lagi **mengarang** slug di mode single — satu
  tempat yang harus tahu sedang di mode apa, hilang.
- Route tree jadi satu, bukan dua yang didaftarkan bersyarat.
- Saat mode naik: `/w/app` tetap valid, cuma kini satu di antara banyak.
  **Nol tautan mati, nol redirect, nol restart.**

Yang dibayar: `/w/` muncul di URL aplikasi satu-workspace. 0006 pernah
menjanjikan "user tak pernah melihat kata workspace" — tapi `/w/` bukan kata.

### 3. Mode tenancy hidup di DATABASE, sebagai ratchet

`platform_settings.tenancy_mode`, dijaga dua trigger:

- `BEFORE UPDATE` — menolak `multi` → apa pun selain `multi`
- `BEFORE DELETE` — menolak penghapusan baris yang bernilai `multi`

Trigger kedua **bukan kelebihan**: baris yang absen dibaca sebagai `single`, jadi
`DELETE` adalah penurunan yang menyamar. Terbukti bisa dilakukan saat dirancang
dengan satu trigger; ditutup setelah diuji. Keempat jalur (UPDATE, UPSERT,
DELETE, dan kenaikan berulang) dikunci `TestRatchetMode_MultiTakBisaTurun`.

Nol baris = `single`. **`APP_MODE` dihapus** — deployment baru lahir sebagai satu
aplikasi tanpa siapa pun perlu mengisi apa pun.

Pemeriksaan lama "mode single tapi DB berisi >1 workspace → tolak start"
**dihapus, bukan dipindah**: keadaan itu tak bisa terjadi lagi. Untuk punya
banyak workspace, mode harus sudah naik ke multi, dan turunnya ditolak database.
Aturan yang dulu harus diperiksa kini mustahil dilanggar.

### 4. Workspace PRIMER: rumah aplikasi

Kolom `tenants.is_primary`, dengan unique partial index (`WHERE is_primary`) yang
menjamin tepat satu. Kolom, bukan perbandingan slug: yang bergantung padanya
adalah penolakan arsip/hapus, dan aturan sepenting itu tak boleh bergantung pada
string yang kebetulan cocok.

**Tak bisa diarsipkan maupun dihapus** — dijaga di handler DAN di SQL
(`AND NOT is_primary`). Di mode single, mengarsipkannya membuat seluruh aplikasi
read-only lewat tombol yang tampak rutin, dan tak ada halaman tersisa untuk
membatalkannya dari dalam.

**Tak memakan kuota.** Kuota membatasi berapa yang boleh DIBUAT, dan rumah
aplikasi tak dibuat siapa pun — ia ada sebelum user pertama mendaftar. Tanpa ini,
super_admin yang jadi ownernya memakai jatahnya sendiri: dengan default 1, orang
yang **menetapkan** aturan kuota justru tak bisa membuat workspace apa pun.

Sidebar dan penegakan memakai definisi yang sama (`is_primary` dikecualikan di
keduanya) — dikunci `TestSidebarKuota_SamaDenganPenegakan`. Dua hitungan yang
berbeda sedikit saja membuat user melihat tombol yang lalu ditolak.

### 5. super_admin = OWNER workspace primer

Dipasang saat **login** (`ensurePrimaryOwner`), bukan saat boot: saat boot belum
ada satu pun baris `users`. Idempotent & promote-only.

Kenapa perlu: super_admin punya nol baris DB (0002) — ia bekerja lewat
`WithSuper`, tanpa keanggotaan di mana pun. Selama masih single itu tak terasa,
tapi begitu mode naik ke multi, rumah aplikasi jadi satu-satunya workspace yang
mustahil dikelola seperti workspace lain: **wasitnya tak pernah bisa berhenti
bermain.**

Dengan baris membership, dua topi terpisah — kepemilikan workspace (bidang
tenant) dan otoritas platform (bidang env) — dan masing-masing bisa dilepas
sendiri. super_admin bisa menyerahkan `/w/app` lewat panel anggota dan mundur
jadi wasit murni, tanpa menyentuh `SUPER_ADMIN_EMAILS`.

Ini **tidak** melanggar 0002: yang dilarang naik dari DB adalah otoritas
PLATFORM, dan itu tetap env-only. Baris ini hanya menambah kepemilikan tenant.

**Konsekuensi yang disengaja dan disetujui**: menghapus email dari
`SUPER_ADMIN_EMAILS` tak lagi mencabut kepemilikan atas workspace primer —
orangnya turun jadi **owner biasa**, bukan user biasa. Itu lebih jujur: mencabut
orang dari sebuah workspace harus terlihat di panel anggota dan terekam audit,
bukan jadi efek samping menyunting file env.

Yang **tidak** boleh menyusul: menentukan jalur RLS dari keanggotaan ("kalau ia
anggota pakai `WithTenant`, kalau bukan `WithSuper`"). Itu mengambil keputusan
bypass dari DATA — persis yang dilarang berulang di codebase ini. Keputusan tetap
dari ROLE.

## Alternatif yang ditolak

**Membuang RLS sekalian**, karena nol query saat ini bergantung padanya
(ditelusuri: semua query ke tabel ber-RLS menyertakan `tenant_id` eksplisit atau
lewat `WithSuper`). Ditolak: nilainya bukan untuk query yang ADA, melainkan untuk
yang BELUM ditulis — ini template yang di-clone, dan yang menulis `SELECT ... FROM
activity_presence` tanpa filter adalah project turunan enam bulan lagi. Biayanya
kini nyaris nol.

**Mempertahankan dua DSN**, demi lapisan anti-injection. Ditolak: permukaan
injection-nya nol (sqlc berparameter, tak ada SQL mentah), sementara biayanya
adalah env yang bisa lupa diisi — dan mode kegagalannya adalah kebocoran senyap,
bukan error.

**Restart setelah upgrade mode**, agar route bisa didaftarkan bersyarat seperti
0006. Ditolak setelah keputusan #2: dengan satu bentuk URL, tak ada route yang
perlu didaftarkan ulang.

**Menjaga ratchet di Go**, bukan di database. Ditolak: penjagaan yang bisa
dilewati dengan memanggil fungsi lain bukanlah penjagaan. `appmode.Set` sengaja
tetap menerima arah apa pun — yang menahan adalah trigger.

## Konsekuensi

- Produksi tak lagi butuh `ALTER ROLE app_rw LOGIN PASSWORD`. Migrasi mengurus
  seluruhnya.
- Dev kini terikat RLS. Query yang lupa `WHERE tenant_id` gagal lebih awal.
- 11 migrasi disatukan jadi satu skema dasar. Jendela ini hanya terbuka selama
  belum ada deployment; setelah ada, migrasi kembali inkremental seperti biasa.
- Tenant seed `default` hilang — beserta kode boot yang dulu harus mengadopsinya.
- `canEditWorkspace` tetap melonggar di mode single, tapi **alasannya berubah**:
  bukan lagi "tak ada owner" (sekarang ada), melainkan bahwa admin adalah
  pembantu operasional dan mengganti nama aplikasi tak boleh menuntut orang
  menyunting `.env` lalu restart.
- Belum ada UI untuk menaikkan mode. `UpgradeToMulti` tersedia dan diuji;
  pemicunya masih manual. **Task terbuka, bukan lupa.**
