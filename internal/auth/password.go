// Package auth mengurus hashing password (argon2id) & verifikasi.
package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

// Parameter argon2id — baseline minimum OWASP 2026 (m=19456 KiB, t=2, p=1).
// Naikkan memory bila kapasitas server memungkinkan. Parameter disimpan di
// dalam string PHC, jadi menaikkan cost tidak mematahkan hash lama.
const (
	argonMemory  uint32 = 19456 // KiB (19 MiB)
	argonTime    uint32 = 2     // jumlah pass/iterasi
	argonThreads uint8  = 1     // degree of parallelism
	argonSaltLen uint32 = 16    // byte
	argonKeyLen  uint32 = 32    // byte
)

var (
	ErrInvalidHash         = errors.New("auth: format hash PHC tidak valid")
	ErrIncompatibleVersion = errors.New("auth: versi argon2 tidak kompatibel")
)

// HashPassword menghasilkan hash argon2id dalam format PHC string siap simpan di DB:
// $argon2id$v=19$m=<mem>,t=<time>,p=<par>$<salt>$<hash>
func HashPassword(plain string) (string, error) {
	salt := make([]byte, argonSaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("auth: gagal generate salt: %w", err)
	}

	hash := argon2.IDKey([]byte(plain), salt, argonTime, argonMemory, argonThreads, argonKeyLen)

	b64Salt := base64.RawStdEncoding.EncodeToString(salt)
	b64Hash := base64.RawStdEncoding.EncodeToString(hash)

	return fmt.Sprintf(
		"$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, argonMemory, argonTime, argonThreads, b64Salt, b64Hash,
	), nil
}

// VerifyPassword membandingkan plaintext dengan hash PHC secara constant-time.
// Recompute memakai parameter dari hash tersimpan, bukan konstanta — hash lama
// tetap terverifikasi walau cost dinaikkan di masa depan.
func VerifyPassword(plain, encoded string) (bool, error) {
	mem, t, threads, salt, hash, err := decodePHC(encoded)
	if err != nil {
		return false, err
	}
	other := argon2.IDKey([]byte(plain), salt, t, mem, threads, uint32(len(hash)))
	return subtle.ConstantTimeCompare(hash, other) == 1, nil
}

// decodePHC memparse string PHC argon2id menjadi parameter, salt, dan hash.
func decodePHC(encoded string) (mem, t uint32, threads uint8, salt, hash []byte, err error) {
	parts := strings.Split(encoded, "$")
	// ["", "argon2id", "v=19", "m=...,t=...,p=...", "<salt>", "<hash>"]
	if len(parts) != 6 || parts[1] != "argon2id" {
		return 0, 0, 0, nil, nil, ErrInvalidHash
	}

	var version int
	if _, err = fmt.Sscanf(parts[2], "v=%d", &version); err != nil {
		return 0, 0, 0, nil, nil, ErrInvalidHash
	}
	if version != argon2.Version {
		return 0, 0, 0, nil, nil, ErrIncompatibleVersion
	}
	if _, err = fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &mem, &t, &threads); err != nil {
		return 0, 0, 0, nil, nil, ErrInvalidHash
	}

	salt, err = base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return 0, 0, 0, nil, nil, fmt.Errorf("auth: gagal decode salt: %w", err)
	}
	hash, err = base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return 0, 0, 0, nil, nil, fmt.Errorf("auth: gagal decode hash: %w", err)
	}
	return mem, t, threads, salt, hash, nil
}
