# 0002 — Multi-tenancy: Postgres RLS + model role 2-bidang

Status: Diterima (2026-07-24)

## Konteks

`go_stater` jadi base template untuk SaaS multi-tenant (satu deployment, banyak
organisasi/klien, tiap klien hanya lihat datanya). Semula single-tenant: data
terikat `user_id`, role global `user < admin < super_admin`, tanpa `tenant_id`.
Menambah multi-tenancy = keputusan fondasi yang menyentuh setiap tabel/query/
middleware — jauh lebih murah diputuskan di fase starter daripada di-retrofit ke
tiap project turunan.

## Keputusan

1. **1 user = 1 tenant** (`users.tenant_id NOT NULL`). Sederhana; tak perlu tabel
   membership. Register/OAuth user baru = buat tenant baru + owner.

2. **Isolasi = Postgres RLS** (bukan hanya `WHERE tenant_id` di app). Defense-in-
   depth: kelalaian developer (lupa filter) tak jadi kebocoran — DB yang menolak.
   - `ENABLE` + **`FORCE`** ROW LEVEL SECURITY (FORCE = berlaku juga utk owner tabel).
   - Policy `tenant_isolation` (USING + WITH CHECK) berbasis **dua GUC**:
     `app.tenant_id` (scope) + `app.is_super` (bypass platform). `current_setting
     (..., true)` = missing-safe (NULL → deny, bukan error).
   - **Role `app_rw` `NOBYPASSRLS`** untuk pool runtime — kalau app konek sebagai
     owner/superuser, RLS TAK berlaku (bocor senyap). Dual-DSN memisahkan ini.

3. **Primitive `WithTenant` / `WithSuper`** (`internal/db/tenant.go`): satu tx per
   pemanggilan dengan `set_config(..., true)` **transaction-local** → GUC tak bocor
   ke peminjam pool berikutnya (pool-leak = kebocoran tenant #1 paling umum).
   `WithSuper` (is_super=on) = jalur platform: auth pre-identity, boot, super_admin/
   staff. Keputusan bypass diambil di Go (dari ROLE), TAK PERNAH dari data DB.

4. **`h.DB` dihapus** dari Handler → **`h.q(ctx)`** ambil Queries ber-tenant dari
   middleware `Scope`. Efek: lupa scope = panic keras di dev, bukan kebocoran
   senyap. Properti anti-footgun terpenting untuk template yang di-clone.

5. **Model role 2-BIDANG tegak-lurus** (bukan satu tangga):
   ```
   PLATFORM (bypass RLS, lintas-tenant)
   ├─ super_admin  → ENV-ONLY (SUPER_ADMIN_EMAILS). Immutable, nol baris DB.
   └─ staff        → tabel platform_staff (mutable). Support aktif; TAK bisa
                     hapus/suspend tenant / kelola staff lain.
   TENANT (scoped RLS, users.tenant_id NOT NULL)
   └─ owner > admin > member
   ```
   Operator platform BUKAN "owner yang lebih besar" — mereka di luar semua tenant.
   Ini yang bikin RLS bersih. Casbin menyatukan lewat hierarki `g` agar platform
   mewarisi izin tenant (untuk impersonate/bantu aktif).

6. **super_admin nol baris DB**: role efektif di-overlay `RefreshIdentity` per-
   request (env-check + lookup `platform_staff`), TAK ditulis ke `users.role`
   (CHECK hanya `owner/admin/member`). God-mode tak bisa dibuat lewat app =
   properti keamanan terkuat. `PromoteSuperAdmins` (reconcile boot lama) dihapus.

7. **Dual-DSN**: `DATABASE_URL` (owner: migrate+boot, bypass) + `APP_DATABASE_URL`
   (app_rw: runtime, RLS mengikat). Fallback ke `DATABASE_URL` bila kosong (dev).

## Alternatif ditolak

- **Kolom `tenant_id` + filter app saja** (tanpa RLS): satu query lupa filter =
  kebocoran. RLS memindahkan jaminan ke DB (tak bisa dilewati kelalaian app).
- **Skema per-tenant / DB per-tenant**: operasional berat (migrasi × N), tak cocok
  untuk starter yang ingin ringan. RLS = satu skema, isolasi baris.
- **super_admin di DB (mutable)**: god-mode via app = permukaan serang. Env-only
  lebih aman & lebih sederhana (sudah ada infrastrukturnya).

## Konsekuensi

- Test isolasi (`internal/handler/rls_test.go`) konek sebagai `app_rw` non-superuser
  (via `SET ROLE` di `AfterConnect`) — membuktikan RLS SUNGGUH mengikat, bukan
  cuma logika app. Test lain konek superuser (uji logika; RLS di-bypass diam-diam).
- Audit ditulis di tx `WithSuper` TERPISAH dari Scope tx (fail-soft struktural:
  gagal audit tak abort aksi utama).
- Gotcha operasional terdokumentasi di `CLAUDE.md` (§ multi-tenancy).
