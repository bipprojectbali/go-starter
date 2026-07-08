# 0001 — Casbin: subject=role, adapter file/embed in-memory

Status: Diterima (2026-07-08)

## Konteks

Butuh RBAC untuk dua panel — `/dev` (developer/owner, super-admin) sekarang dan
`/admin` (nanti) — dengan akses granular per-halaman & per-komponen. Role
hierarkis: `user < admin < super_admin`. Dipilih **Casbin** (bukan hardcode map)
untuk fondasi jangka panjang.

## Keputusan

1. **Casbin v2** (`github.com/casbin/casbin/v2`), `SyncedEnforcer` (handler konkuren).
2. **Adapter = file/embed in-memory**, BUKAN DB adapter (GORM/pgx).
   - Role fixed & policy statis. "Granular" = penamaan resource halus
     (`dev:users:btn-delete`), bukan editabilitas policy saat runtime.
   - `model.conf` + `policy.csv` di-embed (`//go:embed`), versionable di git,
     single-binary murni, nol drift DB↔Casbin.
3. **Subject Casbin = ROLE** (string dari kolom `users.role`), BUKAN userID.
   - Ubah role user = satu `UPDATE users SET role=$1`. Casbin tak tersentuh.
   - `g` (grouping) hanya untuk hierarki role statis, bukan mapping per-user.
4. **Super-admin env-override**: email di `SUPER_ADMIN_EMAILS` = root immutable.
   Bypass SEBELUM Enforce di `authz.Can()` (`session.IsRoot`), dan lolos gate
   status saat login. Root tak pernah bisa dikunci lewat app.
5. **Enforcement 3 lapis**: route (`RequireEnforce`) + UI (`When`, kosmetik) +
   service (`GuardSetRole`/`GuardMutateStatus`/`GuardDelete` = sumber kebenaran).

## Konsekuensi

- Perubahan `policy.csv` butuh redeploy (disengaja untuk starter).
- `authz.Can` fail-CLOSED pada error Enforce (deny bila authz tak pasti).

## Escape hatch (bila kelak butuh policy editable-via-UI / multi-tenant)

Ganti argumen adapter yang di-pass ke enforcer → `pckhoi/casbin-pgx-adapter/v3`
dengan `WithConnectionPool(pool)` + `WithSkipTableCreate()`, buat tabel
`casbin_rule` via goose (schema pckhoi: `id text PK, p_type text, v0..v5 text`),
seed `policy.csv` sekali via `e.SavePolicy()`. **Subject tetap role → nol rewrite
call-site.** Migrasi mekanis, bukan arsitektural.
