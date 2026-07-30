package handler

import (
	"context"
	"net/http"

	"go_starter/internal/db"
	"go_starter/internal/ui/pages/panel"
)

// lifecycle.go — gerbang siklus hidup workspace (keputusan 0005). Ditegakkan di
// SATU tempat (middleware Scope), bukan tersebar di handler: semua route
// ber-workspace melewati Scope, jadi handler baru tak bisa lupa. Cek di handler
// berarti setiap handler baru adalah lubang baru.

// Nilai kolom tenants.status. Bukan string bebas — CHECK constraint di migrasi
// 00010 menjaga sisi DB, konstanta ini menjaga sisi Go.
const (
	TenantActive    = "active"
	TenantSuspended = "suspended"
	TenantArchived  = "archived"
)

// GracePeriodDays = lama masa tenggang sebelum workspace terhapus boleh dipurge
// permanen. Konstanta bernama, bukan angka telanjang di query (Rule 15).
const GracePeriodDays = 30

type tenantStatusKey struct{}
type tenantPrimaryKey struct{}

// withTenantStatus menaruh status workspace di context (dipanggil Scope) agar
// handler bisa merender read-only TANPA query ulang — Scope sudah membacanya.
func withTenantStatus(ctx context.Context, status string) context.Context {
	return context.WithValue(ctx, tenantStatusKey{}, status)
}

// withTenantPrimary menandai bahwa workspace request ini adalah RUMAH APLIKASI.
// Ditumpangkan pada baris tenant yang sudah dibaca Scope — bukan query kedua.
func withTenantPrimary(ctx context.Context, primary bool) context.Context {
	return context.WithValue(ctx, tenantPrimaryKey{}, primary)
}

// IsPrimaryWorkspace melaporkan apakah workspace request ini adalah rumah
// aplikasi. Dipakai view untuk menyembunyikan zona bahaya, DAN handler untuk
// menolaknya — penegakan sebenarnya ada di handler + SQL, view hanya kenyamanan.
//
// Default false pada route tanpa workspace: arah aman, sebab konsekuensi salah
// di sini adalah menampilkan tombol yang akan ditolak, bukan mengizinkan aksi.
func IsPrimaryWorkspace(ctx context.Context) bool {
	p, _ := ctx.Value(tenantPrimaryKey{}).(bool)
	return p
}

// tenantStatus mengembalikan status workspace request ini. Default "active" bila
// tak ada (route non-workspace) — arah aman: jangan mengunci halaman yang memang
// tak punya konsep workspace.
func tenantStatus(ctx context.Context) string {
	if s, ok := ctx.Value(tenantStatusKey{}).(string); ok && s != "" {
		return s
	}
	return TenantActive
}

// IsReadOnly melaporkan apakah workspace request ini menolak perubahan.
// Dipakai view untuk menyembunyikan tombol — tapi penegakan SEBENARNYA ada di
// gateLifecycle (UI = kenyamanan, bukan pengaman).
func IsReadOnly(ctx context.Context) bool {
	return tenantStatus(ctx) != TenantActive
}

// gateLifecycle memutuskan apakah request boleh lanjut, berdasar keadaan
// workspace. Mengembalikan false bila response sudah ditulis (permintaan ditolak).
//
// Aturan (0005 §3) — dipanggil SETELAH keanggotaan terbukti:
//
//	deleted   → 404. Bagi anggota, workspace itu memang sudah berakhir; masa
//	            tenggang adalah jaring operasional (restore lewat panel), bukan
//	            keadaan yang perlu dijelaskan ke mereka.
//	suspended → 403 + halaman penjelasan. BUKAN 404: anggota sah berhak tahu
//	            KENAPA ia tak bisa masuk, kalau tidak ia mengira workspace-nya
//	            hilang lalu menghubungi support tanpa perlu.
//	archived  → GET lolos (read-only), non-GET ditolak 403.
func (h *Handler) gateLifecycle(w http.ResponseWriter, r *http.Request, t db.Tenant) bool {
	if t.DeletedAt.Valid {
		http.NotFound(w, r)
		return false
	}
	switch t.Status {
	case TenantSuspended:
		reason := ""
		if t.SuspendReason != nil {
			reason = *t.SuspendReason
		}
		h.renderLifecycleBlock(w, r, t.Name, reason)
		return false
	case TenantArchived:
		// Hanya membaca yang diizinkan. Penegakan per-METODE, bukan sekadar
		// menyembunyikan tombol — form yang di-POST langsung tetap harus ditolak.
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.Error(w, "workspace diarsipkan (hanya-baca)", http.StatusForbidden)
			return false
		}
	}
	return true
}

// renderLifecycleBlock menampilkan halaman penjelasan workspace tersuspensi.
// 403 + alasan: satu-satunya keadaan di mana kita SENGAJA memberi tahu lebih
// banyak daripada 404, karena penerimanya sudah terbukti anggota.
func (h *Handler) renderLifecycleBlock(w http.ResponseWriter, r *http.Request, name, reason string) {
	w.WriteHeader(http.StatusForbidden)
	h.renderPage(w, r, "Workspace Ditangguhkan", panel.WorkspaceSuspended(name, reason))
}
