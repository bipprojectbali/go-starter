# 0008 — Daftar anggota hanya untuk pengelola

Status: **Diterima** (2026-07-30) — men-supersede sebagian
[0004](0004-workspace-di-path-url.md) §3 (baris `/w/{slug}/members`)

## Konteks

0004 membuka `/w/{slug}/members` untuk **semua anggota**, dengan alasan yang
ditulis eksplisit: *"tahu siapa saja yang punya akses adalah bagian dari
mempercayai ruang bersama — kalau daftarnya disembunyikan, anggota tak bisa tahu
siapa yang dapat membaca pekerjaannya."*

Alasan itu tidak salah, tapi ia menimbang satu sisi saja. Sisi yang tak
ditimbang: **daftar anggota adalah direktori orang.** Ia mengumpulkan nama,
wajah, dan keanggotaan setiap orang di satu halaman yang bisa disalin sekaligus
— oleh siapa pun yang pernah diundang, termasuk yang keanggotaannya sementara,
dan termasuk akun yang kelak diambil alih orang lain.

Perbandingan yang menentukan: seorang member tak bisa **berbuat apa-apa** dengan
daftar itu. Ia tak bisa mengundang, tak bisa mengubah role, tak bisa
mengeluarkan siapa pun. Yang tersisa hanyalah pengetahuan tanpa kegunaan — dan
kumpulan PII selalu punya biaya, sementara sisi manfaatnya di sini nol.

Penambahan kolom `users.name` (migrasi 00002) mempertajam ini alih-alih
melunakkannya. Sebelumnya halaman ini hanya membocorkan email tersamar; kini ia
juga menampilkan **nama asli dan foto** setiap orang. Direktorinya jadi lebih
lengkap, jadi pertanyaan "siapa yang sebenarnya butuh ini" jadi lebih mendesak.

## Keputusan

`/w/{slug}/members` hanya untuk **owner, admin, dan role platform** — di mode
single **maupun** multi.

### 1. Gerbang di HANDLER, bukan di route

`MembersPage` memanggil `canManageMembers` di baris pertama dan menolak sebelum
query apa pun dijalankan. Bukan middleware `RequireEnforce` di route, karena
0004 §3 tetap berlaku untuk hal lain: satu alamat melayani semua role, dan yang
membedakan adalah apa yang boleh dilakukan di dalamnya.

Ditolak **sebelum** query, bukan menyaring hasilnya: data yang tak pernah dibaca
tak bisa bocor lewat kekeliruan di lapisan tampilan.

### 2. Berlaku sama di kedua mode

Pembatasan ini soal **siapa yang mengelola keanggotaan**, bukan soal bentuk
aplikasi. Di mode single, pendaftar baru masuk sebagai `member` (0006 §6) —
artinya justru di sanalah paling banyak orang yang tak perlu melihat direktori.

Dikunci test yang menjalankan kedua mode (`TestMembers_MemberDitolak`).

### 3. 403 + penjelasan, BUKAN 404

Penerimanya sudah terbukti anggota workspace ini — `Scope` memvalidasinya lebih
dulu, dan yang bukan anggota memang sudah dapat 404 di sana. Menyangkal
keberadaan halaman kepada anggota sah tak melindungi apa pun; ia cuma membuat
orang mengira ada yang rusak, lalu melaporkannya.

Ini keadaan yang sama dengan workspace tersuspensi (0005 §3): **kepada yang sah,
katakan alasannya.** Halamannya menyebut SIAPA yang bisa membantu (owner &
admin) — penolakan telanjang tak bisa ditindaklanjuti penerimanya.

### 4. Menu mengikuti izin yang SAMA

`workspaceNav` kini menerima **dua** izin terpisah (`canMembers`,
`canSettings`), bukan satu `canManage` untuk keduanya. Keduanya tidak identik —
di mode single, `canEditWorkspace` melonggar untuk admin sementara aturan
keanggotaan dinilai terpisah — jadi menyatukannya akan membuat salah satu menu
berbohong begitu aturannya bergeser.

Sumbernya wajib fungsi yang sama dengan yang menjaga halamannya. Menu yang
tampil lalu ditolak 403 adalah menu hantu (pola yang sama dengan `devNav` &
`quickLinksFor`).

### 5. `maskEmail` DIPERTAHANKAN

Setelah keputusan ini, setiap penglihat halaman anggota adalah pengelola — dan
pengelola melihat email utuh. Jadi jalur penyamaran praktis tak pernah tereksekusi.

Ia tetap dipertahankan atas keputusan sadar: **aturan tampilan dan aturan akses
adalah dua hal berbeda, dan yang satu tak boleh diam-diam bergantung pada yang
lain.** Kalau kelak halaman ini dibuka lagi untuk sebagian orang, perlindungannya
sudah di tempatnya — bukan sesuatu yang harus diingat untuk dipasang kembali.

Konsekuensi yang diterima: ini melanggar semangat "jangan simpan kode mati"
(Rule 17). Dibayar sadar karena yang dipertaruhkan adalah PII, dan biaya lupa
memasang kembali jauh lebih besar daripada biaya menyimpan fungsi kecil beserta
testnya.

## Konsekuensi

- Member kehilangan cara mengetahui siapa yang punya akses ke ruang kerjanya.
  Itu **kerugian nyata**, bukan efek samping yang diabaikan: ia harus bertanya ke
  owner/admin. Diterima karena member juga tak bisa berbuat apa pun dengan
  jawabannya — yang bisa bertindak adalah orang yang ia tanyai.
- Bila kelak ada kebutuhan "anggota tahu siapa rekannya" (mis. fitur mention,
  penugasan, atau kolaborasi), jawabannya **bukan** membuka kembali halaman ini,
  melainkan permukaan yang menyajikan orang **dalam konteks pekerjaan** — di
  sana namanya muncul karena ada gunanya, bukan sebagai direktori telanjang.
  `maskEmail` (§5) menunggu di situ.
- Baris `/w/{slug}/members` di tabel 0004 §3 kini terbaca: member **403**,
  admin & owner penuh. Aturan induk 0004 ("beda role = beda AKSI, bukan beda
  ALAMAT") **tetap berlaku** — alamatnya tetap satu; yang berubah hanya siapa
  yang boleh melewatinya.
