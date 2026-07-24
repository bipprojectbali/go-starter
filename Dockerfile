# syntax=docker/dockerfile:1

# ── Stage build ─────────────────────────────────────────────────────────────
# static/app.css & internal/db/*.sql.go SUDAH di-commit ke git → build cukup
# `go build`, TIDAK perlu Tailwind CLI / sqlc / Node di dalam image.
FROM golang:1.26 AS build
WORKDIR /src

# Cache layer deps: unduh module dulu (invalidasi hanya bila go.mod/go.sum berubah).
COPY go.mod go.sum ./
RUN go mod download

COPY . .
# CGO_ENABLED=0: binary statis (jalan di distroless tanpa libc). Flag sama dgn Makefile.
RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath -buildvcs=false -ldflags="-s -w" \
    -o /out/app .

# ── Stage runtime ───────────────────────────────────────────────────────────
# distroless/static: tanpa shell/libc/package manager (attack surface minimal),
# non-root (uid 65532) default. tzdata TIDAK perlu — `_ "time/tzdata"` di main.go
# sudah meng-embed database zona waktu ke binary.
FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app
COPY --from=build /out/app /app/app

EXPOSE 8080

# ENTRYPOINT tanpa CMD: service `app` → server (default); service `migrate` di
# compose override lewat `command: ["migrate"]` → `/app/app migrate` lalu exit.
ENTRYPOINT ["/app/app"]
