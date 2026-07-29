package handler

import (
	"context"
	"fmt"

	"go_starter/internal/appmode"
	"go_starter/internal/db"

	"github.com/jackc/pgx/v5/pgxpool"
)

// bootstrap.go — penyiapan keadaan awal mode single (keputusan 0006 §5, §10).
// Dijalankan SEKALI saat startup, sebelum server melayani request.

// BootstrapSingleApp memastikan mode single punya TEPAT SATU tenant, dan menolak
// start bila keadaan DB bertentangan dengan mode yang diminta.
//
// Dua hal yang dikerjakannya, keduanya sengaja di startup — bukan lazily saat
// request pertama:
//
//  1. Tenant tunggal dibuat bila belum ada. Aplikasi jadi tak pernah berada di
//     keadaan "belum ada workspace", sehingga tak perlu jalur khusus untuk user
//     pertama — dan keadaan paling jarang diuji adalah yang paling sering rusak.
//
//  2. GAGAL KERAS bila sudah ada lebih dari satu tenant. Memilih diam-diam salah
//     satu berarti tenant lain LENYAP dari pandangan tanpa jejak: kehilangan data
//     yang terlihat seperti bug UI. Operator harus memutuskan sendiri.
//
// Arah sebaliknya (single → multi) tak butuh apa-apa: tenant tunggal menjadi
// workspace pertama dari banyak.
func BootstrapSingleApp(ctx context.Context, pool *pgxpool.Pool, appName string) error {
	if !appmode.IsSingle() {
		return nil
	}
	return db.WithSuper(ctx, pool, func(q *db.Queries) error {
		n, err := q.CountTenants(ctx)
		if err != nil {
			return fmt.Errorf("hitung tenant: %w", err)
		}
		if n > 1 {
			return fmt.Errorf(
				"APP_MODE=single tetapi database berisi %d workspace — "+
					"menjalankan mode single akan menyembunyikan sisanya. "+
					"Pakai APP_MODE=multi, atau pindahkan/hapus workspace lain lebih dulu", n)
		}
		if n == 1 {
			// Sudah ada tepat satu. Slug-nya WAJIB %q: kalau tidak, /app menunjuk
			// tenant yang tak pernah bisa di-resolve dan SETIAP halaman berakhir 404
			// — kegagalan yang membingungkan karena datanya jelas ada.
			if _, err := q.GetTenantBySlug(ctx, appmode.SingleSlug); err == nil {
				return nil // sudah benar
			}
			// Slug belum cocok. Boleh DIADOPSI hanya bila workspace itu masih
			// KOSONG (nol anggota) — keadaan normal untuk DB baru, karena migrasi
			// 00007 membuat tenant "default" sebagai wadah backfill. Mengubah slug
			// workspace yang sudah dipakai orang adalah hal lain sama sekali: itu
			// mematikan setiap tautan yang sudah tersebar (alasan slug immutable
			// sejak 0004), jadi di situ operator yang harus memutuskan.
			rows, err := q.ListTenantsForPlatform(ctx, db.ListTenantsForPlatformParams{Limit: 1, Offset: 0})
			if err != nil {
				return fmt.Errorf("baca workspace tunggal: %w", err)
			}
			if len(rows) == 0 {
				return fmt.Errorf("workspace tunggal tak terbaca")
			}
			if rows[0].MemberCount > 0 {
				return fmt.Errorf(
					"APP_MODE=single: ada 1 workspace (%q, slug %q) yang sudah berisi %d anggota, "+
						"tetapi slug-nya bukan %q. Mengubah slug akan mematikan tautan yang sudah "+
						"tersebar — ubah manual di database bila memang diinginkan, atau pakai APP_MODE=multi",
					rows[0].Name, rows[0].Slug, rows[0].MemberCount, appmode.SingleSlug)
			}
			if err := q.SetTenantSlug(ctx, db.SetTenantSlugParams{
				ID: rows[0].ID, Slug: appmode.SingleSlug, Name: appName,
			}); err != nil {
				return fmt.Errorf("adopsi workspace kosong jadi aplikasi tunggal: %w", err)
			}
			return nil
		}
		// Belum ada apa pun → buat tenant tunggal.
		if _, err := q.CreateTenant(ctx, db.CreateTenantParams{
			Name: appName, Slug: appmode.SingleSlug,
		}); err != nil {
			return fmt.Errorf("buat aplikasi tunggal: %w", err)
		}
		return nil
	})
}
