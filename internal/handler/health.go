package handler

import (
	"net/http"
)

// Liveness — GET /healthz. Proses hidup; selalu 200 bila server jalan.
func (h *Handler) Liveness(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

// Readiness — GET /readyz. 200 bila DB reachable, 503 bila belum siap.
func (h *Handler) Readiness(w http.ResponseWriter, r *http.Request) {
	if err := h.Pool.Ping(r.Context()); err != nil {
		h.Log.Warn("readiness: db unreachable", "err", err)
		http.Error(w, "not ready", http.StatusServiceUnavailable)
		return
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ready"))
}
