package authz

// Role adalah tingkat otoritas sebagai ordinal — perbandingan hierarki O(1)
// (RoleAdmin > RoleUser). Nilai string map ke kolom users.role & subject Casbin.
type Role int8

const (
	RoleUser Role = iota
	RoleAdmin
	RoleSuperAdmin
)

// Nama role di DB / policy Casbin.
const (
	RoleNameUser       = "user"
	RoleNameAdmin      = "admin"
	RoleNameSuperAdmin = "super_admin"
)

// ParseRole memetakan string DB ke Role. Nilai tak dikenal → RoleUser (aman).
func ParseRole(s string) Role {
	switch s {
	case RoleNameSuperAdmin:
		return RoleSuperAdmin
	case RoleNameAdmin:
		return RoleAdmin
	default:
		return RoleUser
	}
}

// String mengembalikan nama role untuk DB/policy.
func (r Role) String() string {
	switch r {
	case RoleSuperAdmin:
		return RoleNameSuperAdmin
	case RoleAdmin:
		return RoleNameAdmin
	default:
		return RoleNameUser
	}
}

// ValidRoleName melaporkan apakah s adalah nama role yang sah (untuk validasi input).
func ValidRoleName(s string) bool {
	return s == RoleNameUser || s == RoleNameAdmin || s == RoleNameSuperAdmin
}
