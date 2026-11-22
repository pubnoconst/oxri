package auth

import (
	"strings"
	"testing"
	"time"
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

func TestServicePending2FA_PasswordAndTOTP(t *testing.T) {
	secret, err := GenerateTOTPSecret()
	if err != nil {
		t.Fatalf("GenerateTOTPSecret: %v", err)
	}
	pw := "super-secret-pilot-passphrase"
	pwHash, err := HashPassword(pw)
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}

	svc, err := NewServiceWithOptions(ServiceOptions{
		TOTPSecret:       secret,
		PasswordHash:     pwHash,
		TOTPConfirmDelay: 50 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("NewServiceWithOptions: %v", err)
	}
	defer svc.Close()

	if !svc.HasPassword() {
		t.Fatalf("expected HasPassword() to be true")
	}

	validCode, err := GenerateTOTPCode(secret, time.Now().UTC())
	if err != nil {
		t.Fatalf("GenerateTOTPCode: %v", err)
	}

	// 1. Valid TOTP + wrong password must fail after cooldown without consuming the TOTP step
	badID, badBind, _, err := svc.StartTOTPVerification("wrong-password-123", validCode, "/journals")
	if err != nil {
		t.Fatalf("StartTOTPVerification(bad pw): %v", err)
	}
	time.Sleep(65 * time.Millisecond)
	if _, _, err := svc.RedeemTOTPVerification(badID, badBind); err != ErrTOTPInvalid {
		t.Fatalf("expected ErrTOTPInvalid for wrong password, got %v", err)
	}

	// 2. Valid password + valid TOTP succeeds after cooldown
	goodID, goodBind, _, err := svc.StartTOTPVerification(pw, validCode, "/journals")
	if err != nil {
		t.Fatalf("StartTOTPVerification(good 2fa): %v", err)
	}
	time.Sleep(65 * time.Millisecond)
	sessID, nextURL, err := svc.RedeemTOTPVerification(goodID, goodBind)
	if err != nil {
		t.Fatalf("RedeemTOTPVerification(good 2fa): %v", err)
	}
	if nextURL != "/journals" || sessID == "" {
		t.Fatalf("unexpected 2FA result: sessID=%q nextURL=%q", sessID, nextURL)
	}
}
