package maintenance

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"go_starter/internal/db"
	"go_starter/internal/settings"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

// tasks.go — pekerjaan pemeliharaan konkret + penguncian antar-instance.

// lockID = advisory lock agar hanya SATU instance menjalankan pemeliharaan pada
// satu waktu. Angkanya berbeda dari lock migrasi (4242) dengan sengaja: dua
// pekerjaan berbeda yang berbagi satu lock akan saling menunggu tanpa alasan,
// dan pemeliharaan yang menunggu migrasi selesai adalah perilaku yang justru
// diinginkan seandainya keduanya memakai lock yang SAMA — tapi itu kebetulan,
// bukan rancangan, dan kebetulan tak bertahan.
const lockID = 4243

// TryLock mengambil advisory lock TANPA menunggu.
//
// try, bukan blocking: instance yang kalah lomba harus LEWAT, bukan antre lalu
// mengulang pekerjaan yang baru saja selesai. Lock dilepas saat koneksi
// dikembalikan, jadi instance yang mati mendadak tak meninggalkan kunci macet.
func tryLock(ctx context.Context, pool *pgxpool.Pool) (release func(), ok bool) {
	conn, err := pool.Acquire(ctx)
	if err != nil {
		return nil, false
	}
	var got bool
	if err := conn.QueryRow(ctx, "SELECT pg_try_advisory_lock($1)", lockID).Scan(&got); err != nil || !got {
		conn.Release()
		return nil, false
	}
	return func() {
		_, _ = conn.Exec(context.WithoutCancel(ctx), "SELECT pg_advisory_unlock($1)", lockID)
		conn.Release()
	}, true
}

// PurgeAuditLogs membuang jejak audit yang lebih tua dari batas retensi.
//
// Batasnya dibaca SETIAP siklus, bukan sekali saat boot: operator yang
// menaikkannya karena sedang menyelidiki sesuatu tak boleh perlu me-restart
// aplikasi agar keputusannya berlaku.
func PurgeAuditLogs(pool *pgxpool.Pool) func(context.Context) (int64, error) {
	return func(ctx context.Context) (int64, error) {
		release, ok := tryLock(ctx, pool)
		if !ok {
			return 0, nil // instance lain sedang mengerjakannya
		}
		defer release()

		days := settings.AuditRetentionDays()
		if !settings.ValidAuditRetentionDays(days) {
			// Nilai di luar batas TIDAK memicu penghapusan dengan angka tebakan:
			// yang dipertaruhkan adalah bukti, dan menebak salah di sini tak bisa
			// dibatalkan. Lebih baik tak membersihkan apa pun sampai orang
			// memperbaiki nilainya.
			return 0, fmt.Errorf("retensi audit tak masuk akal (%d hari) — pembersihan dilewati", days)
		}
		cutoff := time.Now().AddDate(0, 0, -days)

		var n int64
		err := db.WithSuper(ctx, pool, func(q *db.Queries) error {
			rows, e := q.PurgeAuditLogsBefore(ctx, pgtype.Timestamptz{Time: cutoff, Valid: true})
			n = rows
			return e
		})
		return n, err
	}
}

// PurgeExpiredTenants menghapus PERMANEN workspace yang masa tenggangnya habis.
//
// Fungsinya (PurgeTenant + ListExpiredTenants) sudah ada dan diuji sejak 0005,
// tapi satu-satunya pemanggilnya adalah test — sehingga workspace terhapus
// menumpuk selamanya dan slug-nya tak pernah benar-benar bebas. Ini yang
// menyambungkannya.
func PurgeExpiredTenants(pool *pgxpool.Pool, grace time.Duration, log *slog.Logger) func(context.Context) (int64, error) {
	return func(ctx context.Context) (int64, error) {
		release, ok := tryLock(ctx, pool)
		if !ok {
			return 0, nil
		}
		defer release()

		cutoff := pgtype.Timestamptz{Time: time.Now().Add(-grace), Valid: true}
		var n int64
		err := db.WithSuper(ctx, pool, func(q *db.Queries) error {
			expired, e := q.ListExpiredTenants(ctx, cutoff)
			if e != nil {
				return e
			}
			for _, t := range expired {
				if ctx.Err() != nil {
					return ctx.Err()
				}
				// Satu per satu, BUKAN satu DELETE massal: purge memicu CASCADE ke
				// memberships/invites/notifications, dan satu baris bermasalah tak
				// boleh menggagalkan pembersihan seluruh sisanya. Gagal → dicatat,
				// lanjut ke berikutnya; ia akan dicoba lagi siklus depan.
				if e := q.PurgeTenant(ctx, t.ID); e != nil {
					// ID saja, tanpa nama/slug — jejak pemeliharaan bukan tempat PII
					// maupun data workspace orang (Rule 12).
					log.Error("maintenance: purge tenant gagal", "tenantID", t.ID, "err", e)
					continue
				}
				n++
			}
			return nil
		})
		return n, err
	}
}
