package authz

import "errors"

var (
	// ErrProtectedRoot: target adalah super-admin env (root immutable) — tak bisa
	// diubah role/status-nya atau dihapus lewat app.
	ErrProtectedRoot = errors.New("target adalah super-admin root (env) — tak bisa diubah")
	// ErrForbidden: aktor tak berwenang melakukan aksi ini.
	ErrForbidden = errors.New("tidak berwenang")
	// ErrSelfLockout: aktor mencoba mengunci/menghapus dirinya sendiri.
	ErrSelfLockout = errors.New("tidak bisa melakukan aksi ini pada akun sendiri")
	// ErrInvalidRole: nama role tak dikenal.
	ErrInvalidRole = errors.New("role tidak valid")
)

// Actor & Target = subset user yang dibutuhkan guard (dipisah dari db.User agar
// package authz tak bergantung db).
type Actor struct {
	ID     int64
	Role   Role
	IsRoot bool // super-admin env
}

type Target struct {
	ID          int64
	Role        Role
	IsEnvSuperA bool // apakah email target ada di SUPER_ADMIN_EMAILS
}

// GuardSetRole memvalidasi aktor boleh mengubah role target ke newRole.
// Aturan: root env kebal; hanya super-admin boleh menyentuh role super_admin;
// aktor harus punya otoritas >= role yang diberikan; cegah self-demote.
func GuardSetRole(actor Actor, target Target, newRole Role) error {
	if target.IsEnvSuperA {
		return ErrProtectedRoot // root env tak tersentuh siapa pun
	}
	actorRole := effectiveRole(actor)
	// Mengangkat/menurunkan ke atau dari super_admin → hanya super-admin.
	if (newRole == RoleSuperAdmin || target.Role == RoleSuperAdmin) && actorRole < RoleSuperAdmin {
		return ErrForbidden
	}
	// Aktor tak boleh memberi role di atas otoritasnya sendiri.
	if newRole > actorRole {
		return ErrForbidden
	}
	// Cegah self-demote (turunkan diri sendiri → bisa lockout).
	if actor.ID == target.ID && newRole < actorRole {
		return ErrSelfLockout
	}
	return nil
}

// GuardMutateStatus memvalidasi aktor boleh ubah status (disable/block) target.
// Root env kebal; aktor butuh otoritas >= target; block hanya super-admin;
// cegah self-lock.
func GuardMutateStatus(actor Actor, target Target, newStatus string) error {
	if target.IsEnvSuperA {
		return ErrProtectedRoot
	}
	if actor.ID == target.ID {
		return ErrSelfLockout // jangan kunci diri sendiri
	}
	actorRole := effectiveRole(actor)
	if target.Role >= actorRole {
		return ErrForbidden // tak bisa menyentuh setara/di atas
	}
	// "blocked" (ban berat) hanya super-admin.
	if newStatus == "blocked" && actorRole < RoleSuperAdmin {
		return ErrForbidden
	}
	return nil
}

// GuardDelete memvalidasi aktor boleh soft-delete target.
func GuardDelete(actor Actor, target Target) error {
	if target.IsEnvSuperA {
		return ErrProtectedRoot
	}
	if actor.ID == target.ID {
		return ErrSelfLockout
	}
	if target.Role >= effectiveRole(actor) {
		return ErrForbidden
	}
	return nil
}

// effectiveRole: root env selalu super_admin apa pun kolom role.
func effectiveRole(a Actor) Role {
	if a.IsRoot {
		return RoleSuperAdmin
	}
	return a.Role
}
