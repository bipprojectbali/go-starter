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
	// Mengangkat/menurunkan ke atau dari owner (puncak tenant) → butuh owner+
	// (platform mewarisi karena otoritas lebih tinggi).
	if (newRole == RoleOwner || target.Role == RoleOwner) && actorRole < RoleOwner {
		return ErrForbidden
	}
	// Aktor tak boleh memberi role SETARA atau di atas otoritasnya sendiri.
	//
	// `>=`, bukan `>`. Dengan `>` dulu, admin bisa mengangkat member jadi admin —
	// yakni MEMPERBANYAK DIRINYA SENDIRI, sehingga atasannya kehilangan kendali
	// atas siapa saja yang memegang wewenang itu. Di model multi-tenant hal ini
	// tersamar karena masih ada owner di atas admin; di mode single-app (0006)
	// admin adalah jabatan tertinggi yang bisa DIANGKAT, jadi celahnya terbuka
	// penuh: siapa pun yang sekali diangkat bisa mengangkat sisanya.
	//
	// Aturannya: yang menentukan siapa jadi admin haruslah pihak DI ATAS admin —
	// owner di mode multi, super_admin (env) di mode single.
	if newRole >= actorRole {
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
	// "blocked" (ban berat) butuh owner+ (owner di tenant, platform lintas-tenant).
	if newStatus == "blocked" && actorRole < RoleOwner {
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
