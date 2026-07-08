// Package authz membungkus otorisasi berbasis Casbin (RBAC hierarkis).
//
// Model & policy di-embed (in-memory, tanpa DB adapter) — role fixed & policy
// statis, versionable di git, single-binary murni. Subject Casbin = ROLE (bukan
// userID): ubah role user = satu UPDATE, Casbin tak tersentuh. Escape hatch ke
// DB adapter didokumentasikan di docs/decisions/0001.
package authz

import (
	"bufio"
	"context"
	"fmt"
	"log/slog"
	"strings"

	"go_stater/internal/session"

	"github.com/casbin/casbin/v2"
	"github.com/casbin/casbin/v2/model"
)

// enf di-inject saat startup via Init (pola sama seperti session.Init).
var enf *casbin.SyncedEnforcer

// Init menyimpan enforcer global agar Can() bisa dipakai handler/middleware.
func Init(e *casbin.SyncedEnforcer) { enf = e }

// New membangun SyncedEnforcer (aman untuk handler konkuren) dari teks model
// dan policy CSV yang di-embed. Policy di-parse manual → AddPolicies/
// AddGroupingPolicies (in-memory, tanpa adapter file/DB).
func New(modelText, policyCSV string) (*casbin.SyncedEnforcer, error) {
	m, err := model.NewModelFromString(modelText)
	if err != nil {
		return nil, fmt.Errorf("authz: parse model: %w", err)
	}
	e, err := casbin.NewSyncedEnforcer(m)
	if err != nil {
		return nil, fmt.Errorf("authz: new enforcer: %w", err)
	}

	var pRules, gRules [][]string
	sc := bufio.NewScanner(strings.NewReader(policyCSV))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := splitCSV(line)
		if len(fields) < 2 {
			continue
		}
		switch fields[0] {
		case "p":
			pRules = append(pRules, fields[1:])
		case "g":
			gRules = append(gRules, fields[1:])
		default:
			return nil, fmt.Errorf("authz: baris policy tak dikenal: %q", line)
		}
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("authz: baca policy: %w", err)
	}
	if len(gRules) > 0 {
		if _, err := e.AddGroupingPolicies(gRules); err != nil {
			return nil, fmt.Errorf("authz: load grouping: %w", err)
		}
	}
	if len(pRules) > 0 {
		if _, err := e.AddPolicies(pRules); err != nil {
			return nil, fmt.Errorf("authz: load policy: %w", err)
		}
	}
	return e, nil
}

// Can melaporkan apakah user login boleh melakukan act pada obj.
//
// Root (super-admin env) selalu diizinkan — short-circuit SEBELUM Enforce, jadi
// owner tak pernah terkunci walau kolom role/status di DB salah. Error Enforce =
// fail-CLOSED (deny) — jangan pernah izinkan saat authz tak pasti.
func Can(ctx context.Context, obj, act string) bool {
	if session.IsRoot(ctx) {
		return true
	}
	if enf == nil {
		return false
	}
	role := session.Role(ctx)
	if role == "" {
		return false
	}
	ok, err := enf.Enforce(role, obj, act)
	if err != nil {
		slog.Error("authz enforce", "obj", obj, "act", act, "err", err)
		return false // fail-closed
	}
	return ok
}

// splitCSV memecah baris policy "p, sub, obj, act" jadi field ter-trim.
func splitCSV(line string) []string {
	parts := strings.Split(line, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		out = append(out, strings.TrimSpace(p))
	}
	return out
}
