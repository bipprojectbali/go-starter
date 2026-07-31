package handler

import (
	"context"
	"encoding/json"

	"go_starter/internal/db"
	"go_starter/internal/session"
)

// audit.go — penulisan jejak audit. Dipisah dari dev_users.go: jejak ditulis
// dari SELURUH handler (workspace, anggota, auth, platform), jadi menaruhnya di
// file satu panel membuatnya tampak milik panel itu.

// audit menulis jejak aksi admin atas USER lain (metadata TANPA PII — id saja).
//
// Untuk aksi yang sasarannya BUKAN user — workspace, platform — pakai
// auditWorkspace/auditPlatform. `target_type` menentukan tabel mana yang boleh
// di-JOIN saat jejaknya dibaca, jadi salah menyebutnya bukan cuma label keliru:
// panel akan menampilkan NAMA ORANG yang id-nya kebetulan sama dengan id
// workspace. Salah, dan terlihat meyakinkan.
func (h *Handler) audit(ctx context.Context, actorID int64, action string, targetID int64, meta map[string]string) {
	h.auditLog(ctx, actorID, action, auditTargetUser, targetID, meta)
}

// auditWorkspace = jejak aksi yang sasarannya WORKSPACE (buat, ganti nama,
// arsip, hapus, suspend, undang). target_id = tenants.id.
func (h *Handler) auditWorkspace(ctx context.Context, actorID int64, action string, tenantID int64, meta map[string]string) {
	h.auditLog(ctx, actorID, action, auditTargetWorkspace, tenantID, meta)
}

// auditPlatform = jejak aksi yang tak menyasar satu baris pun — pengaturan yang
// berlaku bagi semua orang (kuota global, kenaikan mode tenancy). target_id
// diisi 0/id acuan dan TIDAK dipakai untuk mencari nama.
func (h *Handler) auditPlatform(ctx context.Context, actorID int64, action string, targetID int64, meta map[string]string) {
	h.auditLog(ctx, actorID, action, auditTargetPlatform, targetID, meta)
}

// Nilai target_type yang dikenal. Dipakai bersama SQL (memilih JOIN) & view
// (memilih kalimat) — konstanta agar keduanya tak bisa berbeda diam-diam.
const (
	auditTargetUser      = "user"
	auditTargetWorkspace = "workspace"
	auditTargetPlatform  = "platform"
	auditTargetSession   = "session"
)

// auditLog menulis satu baris audit dengan target_type eksplisit. Dasar untuk
// audit() (target_type="user") & event auth (target_type="session"). Error
// di-log, TAK menggagalkan aksi utama. metadata TANPA PII (id/method saja).
func (h *Handler) auditLog(ctx context.Context, actorID int64, action, targetType string, targetID int64, meta map[string]string) {
	raw := []byte("{}")
	if meta != nil {
		if b, err := json.Marshal(meta); err == nil {
			raw = b
		}
	}
	// Audit di tx WithSuper TERPISAH (bukan Scope tx) — fail-soft struktural:
	// kegagalan tulis jejak TAK mengabort aksi utama (tx berbeda). tenant_id =
	// tenant aktor (selalu >0; semua user punya tenant), jadi jejak ter-atribusi
	// ke home-tenant aktor walau aksinya lintas-tenant (platform). Bypass RLS
	// perlu karena aktor platform menulis audit untuk target di tenant lain.
	// tenant_id kini NULLABLE (migrasi 00010): FK-nya ON DELETE SET NULL agar
	// jejak audit selamat saat workspace dipurge — bukti tak boleh lenyap bersama
	// yang dibuktikan. 0 → NULL, sebab "tenant nol" bukan tenant.
	tenantID := session.TenantID(ctx)
	tenantRef := &tenantID
	if tenantID == 0 {
		tenantRef = nil
	}
	err := db.WithSuper(ctx, h.Pool, func(q *db.Queries) error {
		_, e := q.CreateAuditLog(ctx, db.CreateAuditLogParams{
			ActorUserID: &actorID,
			Action:      action,
			TargetType:  targetType,
			TargetID:    &targetID,
			Metadata:    raw,
			TenantID:    tenantRef,
		})
		return e
	})
	if err != nil {
		h.Log.Error("audit log", "action", action, "err", err) // jangan gagalkan aksi utama
	}
}

// auditAuth mencatat event autentikasi (login/logout) — actor = user sendiri,
// target_type = "session". Fail-soft (via auditLog).
func (h *Handler) auditAuth(ctx context.Context, userID int64, action, method string) {
	var meta map[string]string
	if method != "" {
		meta = map[string]string{"method": method}
	}
	h.auditLog(ctx, userID, action, auditTargetSession, userID, meta)
}
