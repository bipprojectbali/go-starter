.PHONY: help setup tools tailwind dev check build run clean migrate-new migrate-up test css rename doctor

# `make` polos (tanpa target) → tampilkan daftar perintah, BUKAN jalankan setup
# (yang men-download Tailwind 76MB). Default goal wajib sebelum target apa pun.
.DEFAULT_GOAL := help

# Tool CLI (sqlc/goose/air) di-install ke GOPATH/bin, yang belum tentu di PATH
# saat `make` jalan. GNU Make 3.81 (bawaan macOS) meng-exec recipe tanpa
# metacharacter langsung via execvp — melewati shell, jadi `export PATH` tak
# terbaca. Panggil tool via path absolut supaya tak bergantung PATH sama sekali.
GOBIN := $(shell go env GOPATH)/bin
SQLC  := $(GOBIN)/sqlc
GOOSE := $(GOBIN)/goose
AIR   := $(GOBIN)/air

BINARY := app

# Nama module = nama project. Diturunkan dari go.mod, bukan ditulis ulang di
# sini: dua tempat yang menyimpan nama yang sama pasti berbeda suatu saat, dan
# `make rename` hanya perlu menyentuh satu.
PROJECT := $(shell head -1 go.mod | awk '{print $$2}')

# User Postgres default = user OS yang menjalankan make ($(USER)), BUKAN nama
# yang di-hardcode. Sebelumnya baris ini memuat username penulis template, jadi
# setiap orang yang meng-clone mewarisi kredensial yang bukan miliknya dan
# gagal dengan "role does not exist". Override kapan pun lewat env.
TEST_DATABASE_URL ?= postgres://$(USER)@localhost:5432/$(PROJECT)_test?sslmode=disable

# Versi aset vendored (lihat static/VENDOR.md).
TAILWIND_VERSION := v4.3.2
# Deteksi OS/arch untuk binary Tailwind (macos-arm64 / linux-x64 / linux-x64-musl).
TAILWIND_TARGET := $(shell uname -s | tr A-Z a-z | sed 's/darwin/macos/')-$(shell uname -m | sed 's/x86_64/x64/;s/aarch64/arm64/')

## help: tampilkan daftar perintah (default saat `make` tanpa argumen)
help:
	@echo "$(PROJECT) — perintah tersedia:"
	@echo ""
	@grep -E '^## [a-z-]+:' $(MAKEFILE_LIST) | sed 's/## /  /' | awk -F': ' '{printf "  \033[36m%-14s\033[0m %s\n", $$1, $$2}' | sed 's/^  //'
	@echo ""

## setup: install tools + download Tailwind CLI (daisyui.js sudah di-commit)
setup: tools tailwind

## tools: install CLI Go ke GOPATH/bin
tools:
	go install github.com/air-verse/air@latest
	go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest
	go install github.com/pressly/goose/v3/cmd/goose@latest

## tailwind: unduh Tailwind CLI bila belum ada (idempoten, resume-able, verifikasi utuh)
## Detail: unduh parsial = SIGKILL di arm64. Kalau tailwindcss sudah ada & valid, skip download (76MB) — hindari
## unduh ulang tiap `make setup`. Tahan jaringan labil: -C - (resume dari byte
## terakhir bila putus di tengah, bukan mulai ulang), --retry-all-errors +
## --retry 5 + --max-time. exec-test buktikan binary lengkap; korup → hapus & error.
tailwind:
	@if ./tailwindcss --help >/dev/null 2>&1; then \
		echo "tailwindcss sudah ada & valid — skip download ($$(./tailwindcss --help 2>&1 | head -1))"; \
	else \
		echo "mengunduh tailwindcss (resume-able, tahan koneksi putus)..."; \
		curl -fL -C - --retry 5 --retry-all-errors --retry-delay 2 --max-time 900 \
			-o tailwindcss "https://github.com/tailwindlabs/tailwindcss/releases/download/$(TAILWIND_VERSION)/tailwindcss-$(TAILWIND_TARGET)" && \
		chmod +x tailwindcss && \
		{ ./tailwindcss --help >/dev/null 2>&1 || { rm -f tailwindcss; echo "ERROR: tailwindcss korup/terpotong — unduh ulang gagal"; exit 1; }; } && \
		echo "tailwindcss OK ($$(./tailwindcss --help 2>&1 | head -1))"; \
	fi

## css: generate app.css dari class di file .go (Tailwind v4 + daisyUI plugin)
##
## Bergantung `tailwind` dengan alasan yang sama seperti `dev`: binary-nya tak
## di-commit, dan setiap target yang memanggilnya langsung akan gagal di clone
## baru dengan pesan yang menyesatkan.
css: tailwind
	./tailwindcss -i static/input.css -o static/app.css --minify

# Test berjalan PARALEL lagi: tiap paket punya schema Postgres sendiri
# (internal/testdb), jadi TRUNCATE & purge di satu paket tak menyentuh yang lain.
# Sebelumnya semuanya berbagi `public` dan harus dijalankan `-p 1`.

## check: WAJIB hijau setelah tiap perubahan
check:
	$(SQLC) generate
	go vet ./...
	gofmt -l $$(find . -name '*.go' -not -path './internal/db/*')
	go build -o /tmp/$(BINARY)-check .
	TEST_DATABASE_URL="$(TEST_DATABASE_URL)" go test ./...

## test: hanya test (butuh TEST_DATABASE_URL + test DB termigrate)
test:
	TEST_DATABASE_URL="$(TEST_DATABASE_URL)" go test ./...

## dev: live reload
##
## Bergantung `tailwind` supaya clone baru langsung jalan. Binary Tailwind (76MB)
## sengaja TIDAK di-commit, jadi tanpa ini `make dev` gagal di baris pertama build
## dengan "./tailwindcss: No such file or directory" — pesan yang tak menyebut
## `make setup` sama sekali, sehingga penerimanya harus menebak. Target tailwind
## idempotent (skip bila binary sudah valid), jadi ongkosnya nol setelah unduhan
## pertama.
dev: tailwind
	$(AIR)

## build: single binary (regenerate CSS dulu)
build: css
	$(SQLC) generate
	CGO_ENABLED=0 go build -trimpath -buildvcs=false -ldflags="-s -w" -o $(BINARY) .

## run: build lalu jalankan
run: build
	./$(BINARY)

## doctor: periksa lingkungan (.env, Postgres, Redis, database, nama project)
##
## Melaporkan SEMUA yang kurang sekaligus beserta perintah perbaikannya — bukan
## berhenti di masalah pertama, sebab yang ingin diketahui adalah "apa saja yang
## kurang", dan menemukannya satu per satu berarti menjalankan perintah yang sama
## berkali-kali. Memakai pemeriksaan yang SAMA dengan jalur boot.
doctor:
	@go run . doctor

## rename: ganti nama project setelah clone (make rename name=nama-baru)
##
## Mengubah module path + seluruh import + nama DB contoh sekaligus. Tanpa ini,
## tiap clone menuntut penyisiran manual di ratusan tempat — pekerjaan yang
## mudah setengah selesai, dan sisanya baru ketahuan berbulan-bulan kemudian.
rename:
	@./scripts/rename.sh $(name)

## migrate-new: buat migration baru (make migrate-new name=add_x)
migrate-new:
	$(GOOSE) -dir migrations create $(name) sql

## migrate-up: jalankan migration ke DB (butuh GOOSE_DBSTRING atau DATABASE_URL)
migrate-up:
	GOOSE_DRIVER=postgres GOOSE_DBSTRING="$(DATABASE_URL)" $(GOOSE) -dir migrations up

clean:
	rm -f $(BINARY) /tmp/$(BINARY)-check
