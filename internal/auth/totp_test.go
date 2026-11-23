package auth

import (
	"context"
	"errors"
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

	if _, err := svc2.AuthenticateTOTPForIP("198.51.100.10", code); !errors.Is(err, ErrTOTPReplayed) {
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
