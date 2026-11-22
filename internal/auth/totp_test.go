package auth

import (
	"context"
	"strings"
	"testing"
	"time"
)

type memoryReplayStore struct {
	step int64
}

func (m *memoryReplayStore) LoadLastTOTPStep(context.Context) (int64, error) {
	return m.step, nil
}

func (m *memoryReplayStore) SaveLastTOTPStep(_ context.Context, step int64) error {
	if step > m.step {
		m.step = step
	}
	return nil
}

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

func TestServicePendingTOTPVerification_CooldownAndSingleInFlight(t *testing.T) {
	secret, err := GenerateTOTPSecret()
	if err != nil {
		t.Fatalf("GenerateTOTPSecret: %v", err)
	}

	// Default confirm delay must be 30 seconds
	defaultSvc, err := NewServiceWithOptions(ServiceOptions{TOTPSecret: secret})
	if err != nil {
		t.Fatalf("NewServiceWithOptions(default): %v", err)
	}
	defer defaultSvc.Close()
	if defaultSvc.TOTPConfirmDelay() != 30*time.Second {
		t.Fatalf("expected default TOTPConfirmDelay of 30s, got %v", defaultSvc.TOTPConfirmDelay())
	}

	// Fast cooldown for unit testing
	svc, err := NewServiceWithOptions(ServiceOptions{
		TOTPSecret:       secret,
		TOTPConfirmDelay: 60 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("NewServiceWithOptions: %v", err)
	}
	defer svc.Close()

	// 1. Invalid code still enters cooldown and does not reveal failure early
	badID, badBind, _, err := svc.StartTOTPVerification("198.51.100.10", "", "000000", "/journals")
	if err != nil {
		t.Fatalf("StartTOTPVerification(bad): %v", err)
	}
	if _, _, err := svc.RedeemTOTPVerification(badID, badBind); err != ErrTOTPConfirming {
		t.Fatalf("expected ErrTOTPConfirming before cooldown elapses, got %v", err)
	}
	// Parallel attempt from the same IP while cooling down must be rejected
	if _, _, _, err := svc.StartTOTPVerification("198.51.100.10", "", "000000", "/journals"); err != ErrTOTPPending {
		t.Fatalf("expected ErrTOTPPending for concurrent attempt, got %v", err)
	}
	time.Sleep(75 * time.Millisecond)
	if _, _, err := svc.RedeemTOTPVerification(badID, badBind); err != ErrTOTPInvalid {
		t.Fatalf("expected ErrTOTPInvalid after cooldown, got %v", err)
	}

	// 2. Valid code enters cooldown, blocks early redemption, and succeeds after cooldown
	validCode, err := GenerateTOTPCode(secret, time.Now().UTC())
	if err != nil {
		t.Fatalf("GenerateTOTPCode: %v", err)
	}
	goodID, goodBind, _, err := svc.StartTOTPVerification("198.51.100.10", "", validCode, "/journals")
	if err != nil {
		t.Fatalf("StartTOTPVerification(good): %v", err)
	}
	if _, _, err := svc.RedeemTOTPVerification(goodID, goodBind); err != ErrTOTPConfirming {
		t.Fatalf("expected ErrTOTPConfirming before cooldown elapses, got %v", err)
	}
	time.Sleep(75 * time.Millisecond)
	sessID, nextURL, err := svc.RedeemTOTPVerification(goodID, goodBind)
	if err != nil {
		t.Fatalf("RedeemTOTPVerification(good): %v", err)
	}
	if nextURL != "/journals" || sessID == "" {
		t.Fatalf("unexpected result: sessID=%q nextURL=%q", sessID, nextURL)
	}
	if _, ok := svc.Authenticate(sessID); !ok {
		t.Fatalf("expected session from confirmed TOTP to authenticate")
	}
}

func TestServicePerIPLockoutAndCooldownIsolation(t *testing.T) {
	secret, err := GenerateTOTPSecret()
	if err != nil {
		t.Fatalf("GenerateTOTPSecret: %v", err)
	}

	now := time.Unix(1700000015, 0).UTC()
	svc, err := NewServiceWithOptions(ServiceOptions{
		TOTPSecret:       secret,
		TOTPConfirmDelay: 50 * time.Millisecond,
		NowFunc:          func() time.Time { return now },
	})
	if err != nil {
		t.Fatalf("NewServiceWithOptions: %v", err)
	}
	defer svc.Close()

	attackerIP := "203.0.113.55"
	adminIP := "198.51.100.10"

	for i := 0; i < totpMaxFailures; i++ {
		if _, err := svc.AuthenticateTOTPForIP(attackerIP, "000000"); err != ErrTOTPInvalid {
			t.Fatalf("expected ErrTOTPInvalid on attempt %d, got %v", i+1, err)
		}
	}
	if _, err := svc.AuthenticateTOTPForIP(attackerIP, "000000"); err != ErrTOTPLocked {
		t.Fatalf("expected ErrTOTPLocked for attacker IP, got %v", err)
	}

	// Attacker holding an active pending cooldown must not block the admin's IP
	if _, _, _, err := svc.StartTOTPVerification(attackerIP, "", "000000", "/"); err != nil {
		t.Fatalf("StartTOTPVerification(attacker): %v", err)
	}
	if _, _, _, err := svc.StartTOTPVerification(attackerIP, "", "000000", "/"); err != ErrTOTPPending {
		t.Fatalf("expected ErrTOTPPending for parallel attacker attempt, got %v", err)
	}

	validCode, err := GenerateTOTPCode(secret, now)
	if err != nil {
		t.Fatalf("GenerateTOTPCode: %v", err)
	}
	adminID, adminBind, _, err := svc.StartTOTPVerification(adminIP, "", validCode, "/journals")
	if err != nil {
		t.Fatalf("expected admin IP to remain unblocked by attacker IP, got %v", err)
	}
	now = now.Add(60 * time.Millisecond)
	sessID, _, err := svc.RedeemTOTPVerification(adminID, adminBind)
	if err != nil || sessID == "" {
		t.Fatalf("expected admin login to succeed, got err=%v", err)
	}
}

func TestServicePersistentTOTPReplay(t *testing.T) {
	secret, err := GenerateTOTPSecret()
	if err != nil {
		t.Fatalf("GenerateTOTPSecret: %v", err)
	}

	now := time.Unix(1700000015, 0).UTC()
	replayStore := &memoryReplayStore{step: -1}

	svc1, err := NewServiceWithOptions(ServiceOptions{
		TOTPSecret:  secret,
		ReplayStore: replayStore,
		NowFunc:     func() time.Time { return now },
	})
	if err != nil {
		t.Fatalf("NewServiceWithOptions(1): %v", err)
	}

	code, err := GenerateTOTPCode(secret, now)
	if err != nil {
		t.Fatalf("GenerateTOTPCode: %v", err)
	}
	if _, err := svc1.AuthenticateTOTPForIP("198.51.100.10", code); err != nil {
		t.Fatalf("AuthenticateTOTPForIP(1): %v", err)
	}
	_ = svc1.Close()

	// Simulated restart loading watermark from persisted ReplayStore
	svc2, err := NewServiceWithOptions(ServiceOptions{
		TOTPSecret:  secret,
		ReplayStore: replayStore,
		NowFunc:     func() time.Time { return now },
	})
	if err != nil {
		t.Fatalf("NewServiceWithOptions(2): %v", err)
	}
	defer svc2.Close()

	if _, err := svc2.AuthenticateTOTPForIP("198.51.100.10", code); err != ErrTOTPReplayed {
		t.Fatalf("expected ErrTOTPReplayed after restart in same 30s window, got %v", err)
	}
}

func TestServiceAuthenticateFreshWindow(t *testing.T) {
	secret, err := GenerateTOTPSecret()
	if err != nil {
		t.Fatalf("GenerateTOTPSecret: %v", err)
	}

	now := time.Unix(1700000000, 0).UTC()
	svc, err := NewServiceWithOptions(ServiceOptions{
		TOTPSecret: secret,
		NowFunc:    func() time.Time { return now },
	})
	if err != nil {
		t.Fatalf("NewServiceWithOptions: %v", err)
	}
	defer svc.Close()

	sessID, err := svc.CreateSession()
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	if _, valid, fresh := svc.AuthenticateFresh(sessID, FreshAuthWindow); !valid || !fresh {
		t.Fatalf("expected valid=true, fresh=true at t=0, got valid=%t fresh=%t", valid, fresh)
	}

	now = now.Add(2 * time.Minute)
	if _, valid, fresh := svc.AuthenticateFresh(sessID, FreshAuthWindow); !valid || !fresh {
		t.Fatalf("expected valid=true, fresh=true at t=2m, got valid=%t fresh=%t", valid, fresh)
	}

	now = now.Add(4 * time.Minute) // total +6m > 5m FreshAuthWindow
	if _, valid, fresh := svc.AuthenticateFresh(sessID, FreshAuthWindow); !valid || fresh {
		t.Fatalf("expected valid=true, fresh=false at t=6m, got valid=%t fresh=%t", valid, fresh)
	}
}
