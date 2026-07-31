package handler

import (
	"net/http"
	"strconv"
	"strings"

	"go_starter/internal/appmode"
	"go_starter/internal/authz"
	"go_starter/internal/db"
	"go_starter/internal/session"
	"go_starter/internal/ui/pages/dev"

	"github.com/starfederation/datastar-go/datastar"
)

// dev_users_flash.go — balasan SSE panel /dev/users: render ulang baris + toast.
// Dipisah dari dev_users.go (yang berisi keputusan & mutasi) karena ini soal
// BAGAIMANA hasilnya disampaikan, bukan apa yang terjadi.

// devRowUpdated me-render ulang baris user (kontrol menampilkan nilai baru) +
// flash sukses. Ini yang menggantikan reload penuh: perubahan langsung terlihat.
func (h *Handler) devRowUpdated(w http.ResponseWriter, r *http.Request, targetID int64, msg string) {
	u, err := h.q(r.Context()).GetUser(r.Context(), targetID)
	if err != nil {
		h.Log.Error("dev users: reload row", "err", err)
		h.devFlash(w, r, false, "Tersimpan, tapi gagal memuat ulang baris")
		return
	}
	canManageSuper := session.IsRoot(r.Context()) ||
		authz.ParseRole(session.Role(r.Context())) >= authz.RoleSuperAdmin
	one := []db.User{u}
	row := toUserRows(one, h.membershipsByUser(r.Context(), one))[0]

	sse := datastar.NewSSE(w, r)
	var sb strings.Builder
	if err := dev.UserRowNode(row, authz.AssignableRoles(appmode.IsSingle()), canManageSuper).Render(&sb); err != nil {
		h.Log.Error("dev users: render row", "err", err)
		return
	}
	// Ganti baris berdasarkan id (mode default outer, morph by id).
	_ = sse.PatchElements(sb.String())
	patchFlash(sse, true, msg)
}

// devFlash mengirim hanya toast (tanpa mengubah baris).
func (h *Handler) devFlash(w http.ResponseWriter, r *http.Request, ok bool, msg string) {
	sse := datastar.NewSSE(w, r)
	patchFlash(sse, ok, msg)
}

// devFlashErr memetakan error guard ke pesan toast yang ramah.
func (h *Handler) devFlashErr(w http.ResponseWriter, r *http.Request, err error) {
	msg := "Aksi ditolak"
	switch err {
	case authz.ErrProtectedRoot:
		msg = "User ini root (env) — tak bisa diubah"
	case authz.ErrSelfLockout:
		msg = "Tidak bisa melakukan aksi ini pada akun sendiri"
	case authz.ErrForbidden:
		msg = "Anda tidak berwenang untuk aksi ini"
	default:
		h.Log.Error("dev users: guard/load", "err", err)
		msg = "Terjadi kesalahan"
	}
	h.devFlash(w, r, false, msg)
}

// patchFlash mem-patch toast notifikasi (id "flash") via SSE. Toast auto-hilang
// lewat skrip kecil di komponen. ok=true → hijau, false → merah.
func patchFlash(sse *datastar.ServerSentEventGenerator, ok bool, msg string) {
	var sb strings.Builder
	if err := dev.Flash(ok, msg).Render(&sb); err == nil {
		_ = sse.PatchElements(sb.String())
	}
}

// devRowRemoved menghapus baris dari tabel + flash sukses (soft-delete user).
func (h *Handler) devRowRemoved(w http.ResponseWriter, r *http.Request, targetID int64, msg string) {
	sse := datastar.NewSSE(w, r)
	_ = sse.RemoveElement("#user-" + strconv.FormatInt(targetID, 10))
	patchFlash(sse, true, msg)
}
