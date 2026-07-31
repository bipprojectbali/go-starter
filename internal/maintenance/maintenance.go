// Package maintenance menjalankan pemeliharaan berkala yang tak boleh ada di
// jalur request: membuang jejak audit kedaluwarsa dan menghapus permanen
// workspace yang masa tenggangnya habis.
//
// Kenapa in-process, bukan cron eksternal: aplikasi ini single-binary (lihat
// § Batasan di CLAUDE.md), dan menuntut cron di host berarti pemeliharaan yang
// "seharusnya sudah dipasang" — yaitu yang tak pernah dipasang. Konsekuensinya
// diterima sadar dan ditangani di bawah (lihat catatan multi-instance).
package maintenance

import (
	"context"
	"log/slog"
	"time"
)

// Interval antar-siklus. Sehari sekali cukup: yang dikerjakan bersatuan HARI
// (retensi audit, tenggang 30 hari), jadi menjalankannya lebih sering hanya
// menambah query tanpa mengubah hasil.
const defaultInterval = 24 * time.Hour

// startupDelay menunda siklus pertama. Boot adalah saat paling sibuk (migrasi,
// pemanasan pool, lonjakan request pertama), dan DELETE besar di sana bersaing
// dengan hal-hal yang sedang ditunggu orang.
const startupDelay = 5 * time.Minute

// Task = satu pekerjaan pemeliharaan. Mengembalikan jumlah baris yang tersentuh
// agar Runner bisa mencatat APA yang terjadi — pekerjaan pembersihan yang diam
// tak bisa dibedakan dari pekerjaan yang tak pernah jalan.
type Task struct {
	Name string
	Run  func(context.Context) (int64, error)
}

// Runner menjalankan daftar task secara berkala sampai ctx dibatalkan.
type Runner struct {
	Log      *slog.Logger
	Tasks    []Task
	Interval time.Duration // 0 = defaultInterval
	Delay    time.Duration // penundaan siklus pertama; -1 = tanpa penundaan
}

// Start menjalankan runner sampai ctx selesai. BLOCKING — pemanggil yang
// memutuskan menaruhnya di goroutine, sebab dialah yang tahu bagaimana
// shutdown-nya diatur.
func (r *Runner) Start(ctx context.Context) {
	interval := r.Interval
	if interval <= 0 {
		interval = defaultInterval
	}
	delay := r.Delay
	if delay == 0 {
		delay = startupDelay
	}
	if delay > 0 {
		select {
		case <-ctx.Done():
			return
		case <-time.After(delay):
		}
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	r.runOnce(ctx)
	for {
		select {
		case <-ctx.Done():
			r.Log.Info("maintenance: berhenti")
			return
		case <-ticker.C:
			r.runOnce(ctx)
		}
	}
}

// runOnce menjalankan semua task sekali.
//
// Satu task yang gagal TIDAK menghentikan sisanya, dan tak pernah menghentikan
// runner: pemeliharaan adalah kerja latar, dan matinya proses pembersihan karena
// satu kegagalan sesaat berarti tabelnya tumbuh diam-diam sampai ada yang sadar
// berbulan-bulan kemudian.
func (r *Runner) runOnce(ctx context.Context) {
	for _, t := range r.Tasks {
		if ctx.Err() != nil {
			return
		}
		n, err := t.Run(ctx)
		switch {
		case err != nil:
			r.Log.Error("maintenance: task gagal", "task", t.Name, "err", err)
		case n > 0:
			// Hanya dicatat saat ADA yang dikerjakan. Log "0 baris" setiap hari
			// melatih orang mengabaikan baris ini, dan yang diabaikan bukan
			// pengaman.
			r.Log.Info("maintenance: selesai", "task", t.Name, "rows", n)
		}
	}
}
