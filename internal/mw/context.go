package mw

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net/http"
)

type ctxKey int

const requestIDKey ctxKey = iota

// RequestID menyuntikkan ID unik per request ke context + header response.
// Dipakai untuk mengkorelasikan log satu request.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := newRequestID()
		w.Header().Set("X-Request-ID", id)
		ctx := context.WithValue(r.Context(), requestIDKey, id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RequestIDFromContext mengembalikan request ID, atau "" bila tak ada.
func RequestIDFromContext(ctx context.Context) string {
	if id, ok := ctx.Value(requestIDKey).(string); ok {
		return id
	}
	return ""
}

// reqLog mengembalikan logger dengan field request_id terpasang.
func reqLog(r *http.Request, log *slog.Logger) *slog.Logger {
	return log.With("request_id", RequestIDFromContext(r.Context()))
}

func newRequestID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "unknown"
	}
	return hex.EncodeToString(b)
}
