package maintenance

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync/atomic"
	"testing"
	"time"
)

// maintenance_test.go — perilaku Runner. Murni (tanpa DB): yang diuji di sini
// adalah JADWAL & ketahanannya, sementara pekerjaan yang menyentuh Postgres
// diuji di tasks_test.go.

func quietLog() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// TestRunner_MenjalankanSemuaTask: siklus pertama tak boleh menunggu satu
// interval penuh — pemeliharaan yang baru bekerja 24 jam setelah boot berarti
// deployment yang sering di-restart tak pernah dibersihkan sama sekali.
func TestRunner_MenjalankanSemuaTask(t *testing.T) {
	var a, b atomic.Int64
	done := make(chan struct{})

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	r := &Runner{
		Log:      quietLog(),
		Interval: time.Hour, // takkan tercapai; yang diuji siklus PERTAMA
		Delay:    -1,        // tanpa penundaan startup
		Tasks: []Task{
			{Name: "a", Run: func(context.Context) (int64, error) { a.Add(1); return 1, nil }},
			{Name: "b", Run: func(context.Context) (int64, error) {
				b.Add(1)
				close(done)
				return 0, nil
			}},
		},
	}
	go r.Start(ctx)

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("siklus pertama tak berjalan — pemeliharaan yang menunggu satu interval " +
			"penuh tak pernah jalan di deployment yang sering restart")
	}
	if a.Load() != 1 || b.Load() != 1 {
		t.Errorf("semua task harus jalan sekali, got a=%d b=%d", a.Load(), b.Load())
	}
}

// TestRunner_TaskGagalTakMenghentikanSisanya: pemeliharaan adalah kerja latar,
// dan matinya pembersihan karena satu kegagalan sesaat berarti tabel tumbuh
// diam-diam sampai ada yang sadar berbulan-bulan kemudian.
func TestRunner_TaskGagalTakMenghentikanSisanya(t *testing.T) {
	var kedua atomic.Bool
	done := make(chan struct{})

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	r := &Runner{
		Log: quietLog(), Interval: time.Hour, Delay: -1,
		Tasks: []Task{
			{Name: "gagal", Run: func(context.Context) (int64, error) {
				return 0, errors.New("sengaja gagal")
			}},
			{Name: "kedua", Run: func(context.Context) (int64, error) {
				kedua.Store(true)
				close(done)
				return 0, nil
			}},
		},
	}
	go r.Start(ctx)

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("task setelah yang gagal tak dijalankan")
	}
	if !kedua.Load() {
		t.Error("kegagalan satu task tak boleh membatalkan sisanya")
	}
}

// TestRunner_BerhentiSaatShutdown: runner ikut ctx SIGTERM. Tanpa ini, purge
// yang sedang berjalan menahan proses mati atau — lebih buruk — dibunuh di
// tengah penghapusan.
func TestRunner_BerhentiSaatShutdown(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	stopped := make(chan struct{})

	r := &Runner{
		Log: quietLog(), Interval: 10 * time.Millisecond, Delay: -1,
		Tasks: []Task{{Name: "noop", Run: func(context.Context) (int64, error) { return 0, nil }}},
	}
	go func() { r.Start(ctx); close(stopped) }()

	time.Sleep(30 * time.Millisecond)
	cancel()

	select {
	case <-stopped:
	case <-time.After(2 * time.Second):
		t.Fatal("runner tak berhenti setelah ctx dibatalkan")
	}
}

// TestRunner_PenundaanStartupDihormati: boot adalah saat tersibuk (migrasi,
// pemanasan pool, lonjakan request pertama), dan DELETE besar di sana bersaing
// dengan hal yang sedang ditunggu orang.
func TestRunner_PenundaanStartupDihormati(t *testing.T) {
	var jalan atomic.Bool
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	r := &Runner{
		Log: quietLog(), Interval: time.Hour, Delay: time.Hour, // takkan tercapai di test
		Tasks: []Task{{Name: "noop", Run: func(context.Context) (int64, error) {
			jalan.Store(true)
			return 0, nil
		}}},
	}
	go r.Start(ctx)
	time.Sleep(50 * time.Millisecond)

	if jalan.Load() {
		t.Error("task tak boleh jalan sebelum penundaan startup lewat")
	}
}

// TestRunner_ShutdownSaatMenungguPenundaan: proses yang di-restart cepat harus
// tetap bisa mati bersih walau siklus pertamanya belum sempat jalan.
func TestRunner_ShutdownSaatMenungguPenundaan(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	stopped := make(chan struct{})

	r := &Runner{
		Log: quietLog(), Delay: time.Hour,
		Tasks: []Task{{Name: "noop", Run: func(context.Context) (int64, error) { return 0, nil }}},
	}
	go func() { r.Start(ctx); close(stopped) }()

	cancel()
	select {
	case <-stopped:
	case <-time.After(2 * time.Second):
		t.Fatal("runner tersangkut di penundaan startup saat shutdown")
	}
}
