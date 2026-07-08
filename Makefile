.PHONY: setup dev check build run clean migrate-new migrate-up test

BINARY := app
TEST_DATABASE_URL ?= postgres://bip@localhost:5432/go_stater_test?sslmode=disable

## setup: install tools (sekali saja)
setup:
	go install github.com/air-verse/air@latest
	go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest
	go install github.com/pressly/goose/v3/cmd/goose@latest
	@echo "TODO: download tailwind standalone CLI ke ./tailwindcss"

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

## build: single binary
build:
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
