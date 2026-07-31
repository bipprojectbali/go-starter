package handler

import (
	"net/http"
	"strconv"

	"go_starter/internal/db"
	"go_starter/internal/session"
	"go_starter/internal/settings"
)

// dev_settings_retention.go — berapa lama jejak audit disimpan.
//
// Dipisah dari dev_settings.go bukan cuma karena batas baris: yang diatur di
// sini adalah kapan BUKTI dibuang, dan itu tumbuh mengikuti pertanyaan kepatuhan
// & penyelidikan — bukan mengikuti aturan kuota.

// DevSettingsRetention — POST /dev/settings/retention. Ubah masa simpan audit.
//
// Di-gate `platform:settings` yang sama seperti kuota & mode tenancy (lihat
// routes.go): ini keputusan tentang penghapusan permanen yang berlaku bagi
// SEMUA jejak di platform, jadi tak boleh lebih longgar dari yang lain.
func (h *Handler) DevSettingsRetention(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	n, err := strconv.Atoi(r.FormValue("days"))
	// Divalidasi di BACKEND, bukan cuma lewat atribut min/max di form: nilainya
	// user-controlled, dan angka terlalu kecil di sini menghapus bukti yang masih
	// dibutuhkan — kesalahan yang TAK BISA dibatalkan.
	if err != nil || !settings.ValidAuditRetentionDays(n) {
		http.Redirect(w, r, "/dev/settings?err=retention", http.StatusSeeOther)
		return
	}
	uid := session.UserID(ctx)
	value := strconv.Itoa(n)
	if err := h.q(ctx).UpsertSetting(ctx, db.UpsertSettingParams{
		Key: settings.KeyAuditRetentionDays, Value: value, UpdatedBy: &uid,
	}); err != nil {
		h.Log.Error("settings: simpan retensi audit", "err", err)
		http.Redirect(w, r, "/dev/settings?err=failed", http.StatusSeeOther)
		return
	}
	// DB dulu, cache kemudian (pola sama DevSettingsQuota): terbalik = tulis yang
	// gagal meninggalkan cache berbohong sampai proses restart — dan di sini
	// cache yang berbohong berarti pembersihan memakai batas yang tak pernah
	// benar-benar tersimpan.
	settings.Set(settings.KeyAuditRetentionDays, value)
	// Ter-audit: mempersingkat retensi adalah cara paling rapi menghapus jejak,
	// jadi perubahannya sendiri wajib meninggalkan jejak. Catatan ini berumur
	// sama dengan retensi barunya — itu batas yang melekat pada keputusan ini,
	// dan alasan lain kenapa minimumnya tak boleh terlalu pendek.
	h.auditPlatform(ctx, uid, "settings.audit_retention", 0, map[string]string{"value": value})
	http.Redirect(w, r, "/dev/settings?ok=saved", http.StatusSeeOther)
}
