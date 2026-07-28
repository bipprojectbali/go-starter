# 0005 — Siklus hidup workspace: suspend, archive, delete

Status: **Diterima** (2026-07-28) — melengkapi [0003](0003-membership-multi-workspace.md)
(workspace lahir) & [0004](0004-workspace-di-path-url.md) (workspace beralamat)

## Konteks

Workspace bisa DIBUAT tapi tak pernah bisa dihentikan atau dilepas. Dua lubang
nyata:

1. **`tenants.status` sudah ada sejak migrasi 00007 tapi TIDAK PERNAH dibaca.**
   Kolomnya berkomentar `active|suspended`, namun tak ada satu pun pembacanya di
   kode non-generated. Artinya `UPDATE tenants SET status='suspended'` hari ini
   TIDAK BEREFEK apa pun — semua anggota tetap masuk. Kolom yang berbohong lebih
   berbahaya daripada kolom yang tak ada: ia tampak seperti pengaman yang sudah
   terpasang.

2. **Tak ada jalan keluar bagi owner.** `CountTenantOwners` mencegah workspace
   jadi yatim (owner terakhir tak boleh keluar/diturunkan) — benar, tapi
   akibatnya workspace yang sudah tak dipakai menggantung selamanya, memakan
   kuota `workspace_quota` pemiliknya.

Dan satu ranjau bila penghapusan ditambahkan sembarangan: FK ke `tenants` tidak
seragam. `memberships`/`invites`/`notifications` = `ON DELETE CASCADE`, sedangkan
`audit_logs` TANPA CASCADE. `DELETE FROM tenants` akan **gagal** karena audit,
sementara notifikasi & keanggotaan sudah lenyap tanpa jejak.

## Keputusan

### 1. TIGA keadaan, bukan dua — dibedakan oleh SIAPA yang berwenang

| Keadaan | Pelaku | Dibalik oleh | Data |
|---|---|---|---|
| `active` | — | — | normal |
| `suspended` | super_admin/staff (platform) | **hanya platform** | utuh, tak bisa diakses |
| `archived` | owner | **owner sendiri** | utuh, READ-ONLY |
| (deleted) | owner atau platform | owner/platform, ≤30 hari | soft → purge |

"Nonaktif" dan "pause" adalah kebutuhan yang SAMA — dibuat satu mekanisme, bukan
dua. Yang benar-benar berbeda adalah **kewenangannya**:

- `suspended` = tindakan platform TERHADAP workspace (tunggakan, penyalahgunaan).
  Owner TIDAK boleh membatalkannya sendiri — kalau bisa, gunanya hilang.
- `archived` = keputusan owner bahwa pekerjaannya selesai. Owner harus bisa
  membukanya kembali tanpa memohon ke siapa pun.

Menyatukan keduanya jadi satu tombol "pause" pasti menghasilkan salah satu dari
dua kegagalan: owner membatalkan suspensi platform, atau owner tak bisa
mengarsipkan workspace-nya sendiri.

### 2. Penegakan di `Scope`, SATU titik

Semua route ber-workspace melewati `Scope` — satu-satunya tempat yang tak bisa
dilupakan saat menambah handler baru. Cek di handler berarti setiap handler baru
adalah lubang baru. `resolveTenantBySlug` sudah membaca baris `tenants`, jadi
statusnya sudah di tangan; tinggal dipakai (nol query tambahan).

### 3. Urutan cek: KEANGGOTAAN dulu, status kemudian

```
bukan anggota          → 404  (0004: jangan bocorkan keberadaan workspace)
anggota + suspended    → 403 + halaman penjelasan
anggota + archived     → 200, READ-ONLY (semua POST ditolak)
anggota + deleted      → 404  (bagi anggota, ia memang sudah tak ada)
```

**403 bukan 404 untuk anggota sah.** Berbeda dari kasus slug asing: anggota
workspace yang di-suspend BERHAK tahu kenapa ia tak bisa masuk. 404 membuatnya
mengira workspace-nya hilang lalu menghubungi support tanpa perlu. Yang dilindungi
0004 adalah keberadaan workspace dari ORANG LUAR — bukan alasan dari orang dalam.

**`deleted` → 404 bagi anggota**, karena masa tenggang adalah jaring pengaman
operasional (pembatalan lewat panel platform / owner), bukan keadaan yang perlu
dijelaskan ke anggota. Bagi mereka workspace itu memang sudah berakhir.

### 4. Archived = READ-ONLY, ditegakkan per-METODE

`archived` menolak semua request non-GET di dalam workspace (`POST`), bukan
menyembunyikan tombolnya saja. UI ikut menyesuaikan (tombol disembunyikan), tapi
penegakan sebenarnya di `Scope` — UI adalah kenyamanan, bukan pengaman.

Pengecualian: **unarchive sendiri harus tetap jalan**. Route restore diletakkan di
LUAR prefix `/w/{slug}` (`/workspace/{slug}/restore`) supaya tak terkena gerbang
read-only-nya sendiri — jebakan klasik "pintu keluar terkunci dari dalam".

### 5. Delete = SOFT + masa tenggang 30 hari

`deleted_at TIMESTAMPTZ` mengikuti pola `users` yang sudah ada di codebase.
Penghapusan keras langsung adalah satu-satunya aksi di sistem ini yang **tak bisa
dibatalkan**, dan pemicunya biasanya emosi atau salah klik.

- Workspace ber-`deleted_at` hilang dari switcher, kuota, dan semua daftar.
- Purge permanen = tindakan terpisah & terjadwal, BUKAN otomatis di request.
- Slug **tidak** dilepas selama masa tenggang (`UNIQUE` tetap berlaku) — kalau
  dilepas, orang lain bisa mengambil slug itu dan restore jadi mustahil.

### 6. `audit_logs` TIDAK ikut terhapus

FK `audit_logs.tenant_id` diubah jadi `ON DELETE SET NULL`. Bukti tak boleh lenyap
bersama yang dibuktikan — justru pada peristiwa paling penting (penghapusan
workspace) jejaknya paling dibutuhkan. `tenant_id` jadi NULL, tapi baris audit,
aktor, dan waktunya tetap ada.

`memberships`/`invites`/`notifications` tetap CASCADE: itu data operasional, bukan
bukti.

### 7. Kuota hanya menghitung workspace HIDUP

`CountOwnedWorkspaces` menambah filter `deleted_at IS NULL`. Workspace terhapus
yang masih memakan kuota akan terasa seperti bug bagi user, dan mendorong mereka
menghapus permanen lebih cepat — persis kebalikan dari tujuan masa tenggang.

Workspace `archived` **TETAP** memakan kuota: datanya masih disimpan, dan bisa
diaktifkan lagi kapan saja oleh owner. Kalau tidak, arsip jadi celah kuota gratis.

## Alternatif ditolak

- **Satu tombol "pause"** untuk platform & owner — §1: pasti gagal di salah satu
  sisi kewenangan.
- **Hard delete langsung** — tak bisa dibatalkan, dan FK audit membuatnya gagal
  di tengah jalan (sebagian data sudah CASCADE terhapus sebelum error).
- **Status lewat RLS policy** (`tenants.status='active'` di predikat) — RLS
  melindungi BARIS data, sementara ini soal AKSES ROUTE. Menaruhnya di RLS membuat
  workspace suspended tampak KOSONG (bukan terjelaskan), dan platform yang
  seharusnya tetap bisa melihatnya jadi ikut terhalang.
- **`deleted` sebagai nilai `status`** — soft-delete adalah dimensi ORTOGONAL
  terhadap suspend/archive (workspace bisa dihapus saat sedang di-suspend).
  Preseden `users` sudah memisahkan `status` dan `deleted_at`; ikuti.
- **Melepas slug saat delete** — restore jadi mustahil bila slug keburu diambil.

## Konsekuensi

- **`Scope` bertambah satu cabang** dan mengembalikan status ke handler (untuk UI
  read-only). Ini menambah tanggung jawab middleware yang sudah padat — dapat
  dibenarkan karena alternatifnya (cek tersebar di handler) jauh lebih rapuh.
- **Route restore di luar `/w/{slug}`** (§4) — pengecualian yang harus tetap
  terdokumentasi, kalau tidak akan terlihat seperti inkonsistensi.
- **Purge butuh pemicu terjadwal.** Belum ada scheduler di single-binary ini →
  disediakan sebagai perintah manual dulu; otomatisasi = task terbuka, BUKAN
  diam-diam tak dikerjakan.
- **Owner terakhir kini punya jalan keluar** (archive/delete) tanpa melanggar
  guard workspace-yatim `CountTenantOwners`.
- **Migrasi 00010 mengubah FK audit_logs** — `DROP CONSTRAINT` + `ADD` dengan
  `ON DELETE SET NULL`; idempotent, dan Down mengembalikannya.
