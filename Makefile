.PHONY: setup tools tailwind dev check build run clean migrate-new migrate-up test css

# Tool CLI (sqlc/goose/air) di-install ke GOPATH/bin, yang belum tentu di PATH
# saat `make` jalan. GNU Make 3.81 (bawaan macOS) meng-exec recipe tanpa
# metacharacter langsung via execvp — melewati shell, jadi `export PATH` tak
# terbaca. Panggil tool via path absolut supaya tak bergantung PATH sama sekali.
GOBIN := $(shell go env GOPATH)/bin
SQLC  := $(GOBIN)/sqlc
GOOSE := $(GOBIN)/goose
AIR   := $(GOBIN)/air

BINARY := app
TEST_DATABASE_URL ?= postgres://bip@localhost:5432/go_stater_test?sslmode=disable

# Versi aset vendored (lihat static/VENDOR.md).
TAILWIND_VERSION := v4.3.2
# Deteksi OS/arch untuk binary Tailwind (macos-arm64 / linux-x64 / linux-x64-musl).
TAILWIND_TARGET := $(shell uname -s | tr A-Z a-z | sed 's/darwin/macos/')-$(shell uname -m | sed 's/x86_64/x64/;s/aarch64/arm64/')

## setup: install tools + download Tailwind CLI (daisyui.js sudah di-commit)
setup: tools tailwind

## tools: install CLI Go ke GOPATH/bin
tools:
	go install github.com/air-verse/air@latest
	go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest
	go install github.com/pressly/goose/v3/cmd/goose@latest

## tailwind: download binary + VERIFIKASI utuh (unduh parsial = SIGKILL di arm64).
## -f gagal saat HTTP error, --retry tahan jaringan labil, exec-test buktikan
## binary lengkap; kalau korup: hapus & error, jangan biarkan file separuh lolos.
tailwind:
	curl -fL --retry 3 --retry-delay 2 -o tailwindcss "https://github.com/tailwindlabs/tailwindcss/releases/download/$(TAILWIND_VERSION)/tailwindcss-$(TAILWIND_TARGET)"
	chmod +x tailwindcss
	@./tailwindcss --help >/dev/null 2>&1 || { rm -f tailwindcss; echo "ERROR: tailwindcss korup/terpotong — unduh ulang gagal"; exit 1; }
	@echo "tailwindcss OK ($$(./tailwindcss --help 2>&1 | head -1))"

## css: generate app.css dari class di file .go (Tailwind v4 + daisyUI plugin)
css:
	./tailwindcss -i static/input.css -o static/app.css --minify

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
dev:
	$(AIR)

## build: single binary (regenerate CSS dulu)
build: css
	$(SQLC) generate
	CGO_ENABLED=0 go build -trimpath -buildvcs=false -ldflags="-s -w" -o $(BINARY) .

## run: build lalu jalankan
run: build
	./$(BINARY)

## migrate-new: buat migration baru (make migrate-new name=add_x)
migrate-new:
	$(GOOSE) -dir migrations create $(name) sql

## migrate-up: jalankan migration ke DB (butuh GOOSE_DBSTRING atau DATABASE_URL)
migrate-up:
	GOOSE_DRIVER=postgres GOOSE_DBSTRING="$(DATABASE_URL)" $(GOOSE) -dir migrations up

clean:
	rm -f $(BINARY) /tmp/$(BINARY)-check
