package auth

import (
	"strings"
	"testing"
	"time"
)

func TestTOTPRoundTripAndSkew(t *testing.T) {
	secret, err := GenerateTOTPSecret()
	if err != nil {
		t.Fatalf("GenerateTOTPSecret: %v", err)
	}
	if len(secret) != 32 {
		t.Fatalf("expected 32-char Base32 secret for 160 bits, got %d", len(secret))
	}

	uri := BuildTOTPURI(secret, "Res Imperii", "pilot")
	if !strings.HasPrefix(uri, "otpauth://totp/") || !strings.Contains(uri, "secret="+secret) {
		t.Fatalf("unexpected otpauth URI: %s", uri)
	}

	now := time.Unix(1700000015, 0).UTC()
	code, err := GenerateTOTPCode(secret, now)
	if err != nil {
		t.Fatalf("GenerateTOTPCode: %v", err)
	}
	if len(code) != 6 {
		t.Fatalf("expected 6-digit code, got %q", code)
	}

	// Exact window
	if _, ok := VerifyTOTP(secret, code, now); !ok {
		t.Fatalf("expected exact-window code to verify")
	}
	// +1 step skew
	if _, ok := VerifyTOTP(secret, code, now.Add(30*time.Second)); !ok {
		t.Fatalf("expected -1 step skew code to verify")
	}
	// -1 step skew
	if _, ok := VerifyTOTP(secret, code, now.Add(-30*time.Second)); !ok {
		t.Fatalf("expected +1 step skew code to verify")
	}
	// Outside skew window (+2 steps)
	if _, ok := VerifyTOTP(secret, code, now.Add(65*time.Second)); ok {
		t.Fatalf("expected out-of-window code to fail")
	}
	// Non-digits / wrong length
	if _, ok := VerifyTOTP(secret, "abcdef", now); ok {
		t.Fatalf("expected non-digit code to fail")
	}
	if _, ok := VerifyTOTP(secret, "12345", now); ok {
		t.Fatalf("expected short code to fail")
	}
}

func TestServiceAuthenticateTOTP_ReplayAndLockout(t *testing.T) {
	secret, err := GenerateTOTPSecret()
	if err != nil {
		t.Fatalf("GenerateTOTPSecret: %v", err)
	}

	svc, err := NewServiceWithOptions(ServiceOptions{
		TOTPSecret: secret,
	})
	if err != nil {
		t.Fatalf("NewServiceWithOptions: %v", err)
	}
	defer svc.Close()

	if !svc.HasTOTP() {
		t.Fatalf("expected HasTOTP() to be true")
	}
	if svc.HasSSH() {
		t.Fatalf("expected HasSSH() to be false when no SSH pubkey configured")
	}

	// Valid TOTP code produces a session
	code, err := GenerateTOTPCode(secret, time.Now().UTC())
	if err != nil {
		t.Fatalf("GenerateTOTPCode: %v", err)
	}
	sessionID, err := svc.AuthenticateTOTP(code)
	if err != nil {
		t.Fatalf("AuthenticateTOTP failed: %v", err)
	}
	if _, ok := svc.Authenticate(sessionID); !ok {
		t.Fatalf("expected TOTP session to authenticate")
	}

	// Replaying the exact same 6-digit code in the same 30s window must fail
	if _, err := svc.AuthenticateTOTP(code); err != ErrTOTPReplayed {
		t.Fatalf("expected ErrTOTPReplayed on replay, got %v", err)
	}

	// 5 failed attempts must trigger lockout
	for i := 0; i < totpMaxFailures; i++ {
		if _, err := svc.AuthenticateTOTP("000000"); err == nil {
			t.Fatalf("expected wrong code to fail on attempt %d", i+1)
		}
	}
	if _, err := svc.AuthenticateTOTP("000000"); err != ErrTOTPLocked {
		t.Fatalf("expected ErrTOTPLocked after %d failures, got %v", totpMaxFailures, err)
	}
}
