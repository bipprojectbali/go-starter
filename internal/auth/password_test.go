package auth

import (
	"strings"
	"testing"
)

func TestHashPassword_RoundTrip(t *testing.T) {
	hash, err := HashPassword("rahasia123")
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	// Format PHC argon2id.
	if !strings.HasPrefix(hash, "$argon2id$v=19$") {
		t.Errorf("format PHC salah: %q", hash)
	}

	ok, err := VerifyPassword("rahasia123", hash)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if !ok {
		t.Error("password benar seharusnya cocok")
	}
}

func TestVerifyPassword_WrongPassword(t *testing.T) {
	hash, _ := HashPassword("rahasia123")
	ok, err := VerifyPassword("salah", hash)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if ok {
		t.Error("password salah seharusnya TIDAK cocok")
	}
}

func TestHashPassword_UniqueSalt(t *testing.T) {
	// Dua hash dari password sama harus berbeda (salt acak).
	h1, _ := HashPassword("sama")
	h2, _ := HashPassword("sama")
	if h1 == h2 {
		t.Error("dua hash password sama seharusnya berbeda (salt acak)")
	}
}

func TestVerifyPassword_InvalidHash(t *testing.T) {
	if _, err := VerifyPassword("x", "bukan-phc"); err != ErrInvalidHash {
		t.Errorf("want ErrInvalidHash, got %v", err)
	}
}
