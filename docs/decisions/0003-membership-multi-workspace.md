# 0003 — Membership: satu user, banyak workspace (+ invite)

Status: Diterima (2026-07-27) — **men-supersede keputusan #1 di [0002](0002-multi-tenancy-rls-role-2-bidang.md)**

## Konteks

Keputusan 0002 mengunci **1 user = 1 tenant** (`users.tenant_id NOT NULL`). Sederhana,
tapi konsekuensinya: register SELALU membuat workspace baru + owner, dan **tak ada
jalan masuk sebagai `admin`/`member`**. Praktisnya 1 workspace = 1 orang — tak ada tim.

Untuk SaaS nyata, model yang mapan (Slack/Notion/GitHub/Linear) memisahkan:
**identitas** (siapa kamu — global) dari **keanggotaan** (di mana kamu bekerja —
banyak, dengan role berbeda per tempat). User bisa punya workspace sendiri DAN
diundang membantu workspace orang lain.

## Keputusan

1. **`memberships` (user × tenant × role)** jadi sumber kebenaran keanggotaan.
   `UNIQUE(user_id, tenant_id)` — satu role per user per workspace.

2. **`users` jadi tabel GLOBAL** — identitas murni (email/pass_hash/avatar), **keluar
   dari RLS**, kolom `tenant_id` & `role` dihapus. Satu baris per orang, dipakai
   lintas workspace.

3. **RLS TETAP `tenant_id = GUC`** — TIDAK ditulis ulang jadi subquery membership.
   Alasan: memberships menentukan *workspace mana yang boleh DIPILIH*; isolasi data
   tetap oleh `tenant_id` + GUC di `oauth_accounts`/`audit_logs`/`activity_presence`
   (policy 00007 tak berubah sama sekali). Predikat skalar jauh lebih murah daripada
   `EXISTS` per-baris.

4. **Validasi keanggotaan di middleware `Scope`** (memecahkan chicken-and-egg):
   Scope memilih tenant SEBELUM RefreshIdentity memverifikasi identitas. Maka Scope
   sendiri yang memvalidasi tenant-di-session terhadap `memberships`; tak valid →
   fallback ke workspace pertama user; tanpa workspace → `/workspace/new`.
   **Tanpa ini, user bisa memaksa tenant sembarang lewat session.**

5. **`memberships` & `invites` TANPA RLS** — keduanya dibaca justru untuk MENENTUKAN
   scope (dan invite dibuka di jalur publik oleh orang yang belum jadi anggota).
   Keamanan dari filter query (`WHERE user_id = <uid sesi>` / token rahasia).

6. **Register auto-buat workspace pertama** (Slack/Notion style) — tak ada keadaan
   "user tanpa workspace" saat onboarding. Membuat workspace berikutnya dibatasi
   **kuota** (`users.workspace_quota`, default `MAX_WORKSPACES_PER_USER`). Diundang
   sebagai member/admin TIDAK memakan kuota (hanya role owner yang dihitung).

7. **Invite via token** (`invites.token` = 32-byte crypto/rand, TTL 7 hari, sekali
   pakai). Penerima yang belum punya akun → token disimpan di session, otomatis
   diterima setelah register/login. **Pengiriman email DITUNDA** — link ditampilkan
   di panel anggota untuk disalin manual (task terbuka).

8. **Role platform tak berubah** — `super_admin` (env-only) & `staff` tetap seperti
   0002; keduanya bypass RLS dan tak butuh membership.

## Alternatif ditolak

- **RLS lewat subquery membership** (`EXISTS (SELECT 1 FROM memberships …)`): lebih
  "murni" tapi menambah biaya per-baris tanpa manfaat — `tenant_id` di baris data
  sudah cukup, dan membership hanya soal *boleh memilih workspace mana*.
- **users tetap ber-tenant (duplikat per workspace)**: email tak lagi unik global,
  profil/password terduplikasi, login jadi ambigu.
- **Validasi keanggotaan di RefreshIdentity**: terlambat — scope sudah terlanjur
  dipilih. Harus di Scope.

## Konsekuensi

- **Migrasi 00008 destruktif tapi reversibel** (`DROP COLUMN users.tenant_id/role`);
  Down mem-backfill balik dari memberships. Diuji up→down→up sebelum dipakai.
- **`ListUsers` (panel /dev) kini lintas-workspace** — konsekuensi users keluar RLS.
  Itu memang route platform. Daftar anggota per-workspace pakai `ListMembersByTenant`.
- **Role di panel /dev jadi per-workspace**: tiap user menampilkan daftar
  workspace + role, platform bisa mengubah role di workspace mana pun. Satu query
  batch (`ListMembershipsForUsers`) — bukan N+1.
- **Guard workspace-yatim**: `CountTenantOwners` mencegah menurunkan/mengeluarkan
  owner terakhir.
- Invariant RLS 0002 tetap berlaku & tetap diuji: deny-default tanpa GUC, GUC
  transaction-local (anti pool-leak), WITH CHECK menolak INSERT lintas-workspace.
