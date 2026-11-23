package auth

import (
	"strings"
	"testing"
)

func TestHashAndVerifyPassword(t *testing.T) {
	if _, err := HashPassword("short"); err == nil {
		t.Fatalf("expected error for short password")
	}

	pw := "correct-horse-battery-staple-42"
	hash, err := HashPassword(pw)
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	if !strings.HasPrefix(hash, "$argon2id$v=19$") {
		t.Fatalf("unexpected hash format: %q", hash)
	}
	if err := ValidatePasswordHash(hash); err != nil {
		t.Fatalf("ValidatePasswordHash: %v", err)
	}

	if !VerifyPassword(hash, pw) {
		t.Fatalf("expected VerifyPassword to accept matching password")
	}
	if VerifyPassword(hash, "wrong-horse-battery-staple-42") {
		t.Fatalf("expected VerifyPassword to reject wrong password")
	}
	if VerifyPassword(hash, "") {
		t.Fatalf("expected VerifyPassword to reject empty password")
	}
}
