package mw

import (
	"log/slog"
	"net/http"
	"time"
)

// statusRecorder membungkus ResponseWriter untuk menangkap status code.
// PENTING: implement Unwrap() agar http.ResponseController (dipakai Datastar SSE
// untuk Flush) bisa menembus ke writer asli — lihat bug scs+SSE di STATER §4.9.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (sr *statusRecorder) WriteHeader(code int) {
	sr.status = code
	sr.ResponseWriter.WriteHeader(code)
}

// Unwrap mengembalikan writer asli untuk http.ResponseController.
func (sr *statusRecorder) Unwrap() http.ResponseWriter { return sr.ResponseWriter }

// RequestLog mencatat tiap request selesai: metode, path, status, durasi, request_id.
// Skip logging untuk health check agar log tidak berisik.
func RequestLog(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/healthz" || r.URL.Path == "/readyz" {
				next.ServeHTTP(w, r)
				return
			}
			start := time.Now()
			rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(rec, r)
			reqLog(r, log).Info("request",
				"method", r.Method,
				"path", r.URL.Path,
				"status", rec.status,
				"duration_ms", time.Since(start).Milliseconds(),
			)
		})
	}
}
