.PHONY: setup dev check build run clean migrate-new migrate-up test css

BINARY := app
TEST_DATABASE_URL ?= postgres://bip@localhost:5432/go_stater_test?sslmode=disable

# Versi aset vendored (lihat static/VENDOR.md).
TAILWIND_VERSION := v4.3.2
BASECOAT_VERSION := 1.0.2
# Deteksi OS/arch untuk binary Tailwind (macos-arm64 / linux-x64 / linux-x64-musl).
TAILWIND_TARGET := $(shell uname -s | tr A-Z a-z | sed 's/darwin/macos/')-$(shell uname -m | sed 's/x86_64/x64/;s/aarch64/arm64/')

## setup: install tools + download aset vendored (sekali saja)
setup:
	go install github.com/air-verse/air@latest
	go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest
	go install github.com/pressly/goose/v3/cmd/goose@latest
	curl -sL -o tailwindcss "https://github.com/tailwindlabs/tailwindcss/releases/download/$(TAILWIND_VERSION)/tailwindcss-$(TAILWIND_TARGET)" && chmod +x tailwindcss
	curl -sL -o static/basecoat.css "https://cdn.jsdelivr.net/npm/basecoat-css@$(BASECOAT_VERSION)/dist/basecoat.cdn.min.css"

## css: generate app.css dari class di file .go (Tailwind v4 scan otomatis)
css:
	./tailwindcss -i static/input.css -o static/app.css --minify

## check: WAJIB hijau setelah tiap perubahan
check:
	sqlc generate
	go vet ./...
	gofmt -l $$(find . -name '*.go' -not -path './internal/db/*')
	go build -o /tmp/$(BINARY)-check .
	TEST_DATABASE_URL="$(TEST_DATABASE_URL)" go test ./...

## test: hanya test (butuh TEST_DATABASE_URL + test DB termigrate)
test:
	TEST_DATABASE_URL="$(TEST_DATABASE_URL)" go test ./...

## dev: live reload
dev:
	air

## build: single binary (regenerate CSS dulu)
build: css
	sqlc generate
	CGO_ENABLED=0 go build -trimpath -buildvcs=false -ldflags="-s -w" -o $(BINARY) .

## run: build lalu jalankan
run: build
	./$(BINARY)

## migrate-new: buat migration baru (make migrate-new name=add_x)
migrate-new:
	goose -dir migrations create $(name) sql

## migrate-up: jalankan migration ke DB (butuh GOOSE_DBSTRING atau DATABASE_URL)
migrate-up:
	GOOSE_DRIVER=postgres GOOSE_DBSTRING="$(DATABASE_URL)" goose -dir migrations up

clean:
	rm -f $(BINARY) /tmp/$(BINARY)-check
