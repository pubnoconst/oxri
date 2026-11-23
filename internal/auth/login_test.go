package auth

import (
	"errors"
	"fmt"
	"testing"
	"time"
)

// newClockedService returns a Service on a controllable clock.
func newClockedService(t *testing.T, password string) (svc *Service, secret string, now *time.Time) {
	t.Helper()
	secret, err := GenerateTOTPSecret()
	if err != nil {
		t.Fatalf("GenerateTOTPSecret: %v", err)
	}
	clock := time.Unix(1700000000, 0).UTC()
	opts := ServiceOptions{TOTPSecret: secret, NowFunc: func() time.Time { return clock }}
	if password != "" {
		hash, err := HashPassword(password)
		if err != nil {
			t.Fatalf("HashPassword: %v", err)
		}
		opts.PasswordHash = hash
	}
	svc, err = NewServiceWithOptions(opts)
	if err != nil {
		t.Fatalf("NewServiceWithOptions: %v", err)
	}
	t.Cleanup(func() { _ = svc.Close() })
	return svc, secret, &clock
}

func codeAt(t *testing.T, secret string, at time.Time) string {
	t.Helper()
	c, err := GenerateTOTPCode(secret, at)
	if err != nil {
		t.Fatalf("GenerateTOTPCode: %v", err)
	}
	return c
}

func expectLoginErr(t *testing.T, err, want error, wantRetry time.Duration, label string) {
	t.Helper()
	if !errors.Is(err, want) {
		t.Fatalf("%s: expected %v, got %v", label, want, err)
	}
	if got := RetryAfter(err); got != wantRetry {
		t.Fatalf("%s: expected RetryAfter %v, got %v", label, wantRetry, got)
	}
}

func TestFailureDelayEscalates(t *testing.T) {
	want := []time.Duration{0, 2 * time.Second, 4 * time.Second, 8 * time.Second, 16 * time.Second, 16 * time.Second}
	for n, d := range want {
		if got := failureDelay(n); got != d {
			t.Fatalf("failureDelay(%d) = %v, want %v", n, got, d)
		}
	}
}

func TestLoginTOTP_SuccessIsInstantAndSingleUse(t *testing.T) {
	const pw = "correct horse battery staple"
	svc, secret, now := newClockedService(t, pw)
	code := codeAt(t, secret, *now)

	sessID, err := svc.LoginTOTP("198.51.100.10", pw, code)
	if err != nil {
		t.Fatalf("expected instant success, got %v", err)
	}
	if _, ok := svc.Authenticate(sessID); !ok {
		t.Fatalf("expected session to authenticate")
	}

	// Same code again (same 30s step) is a replay, not a failure: no hold.
	_, err = svc.LoginTOTP("198.51.100.10", pw, code)
	expectLoginErr(t, err, ErrTOTPReplayed, 0, "replay")
}

func TestLoginTOTP_EscalatingHoldsThenLockout(t *testing.T) {
	const pw = "correct horse battery staple"
	const ip = "203.0.113.7"
	svc, secret, now := newClockedService(t, pw)

	// Failures 1..4 hold the address for 2s, 4s, 8s, 16s.
	for i, hold := range []time.Duration{2 * time.Second, 4 * time.Second, 8 * time.Second, 16 * time.Second} {
		_, err := svc.LoginTOTP(ip, pw, "000000")
		expectLoginErr(t, err, ErrTOTPInvalid, hold, fmt.Sprintf("failure %d", i+1))

		// While held, even correct credentials are not evaluated.
		*now = now.Add(hold / 2)
		_, err = svc.LoginTOTP(ip, pw, codeAt(t, secret, *now))
		expectLoginErr(t, err, ErrTOTPCooling, hold/2, fmt.Sprintf("held after failure %d", i+1))

		*now = now.Add(hold / 2)
	}

	// Failures landed at t0, +2s, +6s, +14s; this 5th one (+30s) trips the lockout.
	_, err := svc.LoginTOTP(ip, pw, "000000")
	expectLoginErr(t, err, ErrTOTPInvalid, totpLockoutWindow, "5th failure")

	// Locked until the first failure (t0) ages out of the 5-minute window.
	_, err = svc.LoginTOTP(ip, pw, codeAt(t, secret, *now))
	expectLoginErr(t, err, ErrTOTPLocked, 4*time.Minute+30*time.Second, "locked")

	*now = now.Add(4*time.Minute + 30*time.Second)
	if _, err := svc.LoginTOTP(ip, pw, codeAt(t, secret, *now)); err != nil {
		t.Fatalf("expected success once the lockout window passed, got %v", err)
	}
}

func TestLoginTOTP_WrongPasswordAndWrongCodeLookIdentical(t *testing.T) {
	const pw = "correct horse battery staple"
	svc, secret, now := newClockedService(t, pw)
	code := codeAt(t, secret, *now)

	_, errPw := svc.LoginTOTP("198.51.100.1", "wrong password", code)
	_, errCode := svc.LoginTOTP("198.51.100.2", pw, "000000")
	expectLoginErr(t, errPw, ErrTOTPInvalid, 2*time.Second, "wrong password")
	expectLoginErr(t, errCode, ErrTOTPInvalid, 2*time.Second, "wrong code")

	// A wrong-password attempt must not consume the valid code's time step.
	*now = now.Add(2 * time.Second)
	if _, err := svc.LoginTOTP("198.51.100.1", pw, code); err != nil {
		t.Fatalf("expected the same valid code to still work, got %v", err)
	}
}

func TestLoginTOTP_PerIPIsolation(t *testing.T) {
	const pw = "correct horse battery staple"
	svc, secret, now := newClockedService(t, pw)

	for i := 0; i < totpMaxFailures; i++ {
		_, _ = svc.LoginTOTP("203.0.113.66", pw, "000000")
		*now = now.Add(failureDelay(i + 1))
	}
	if _, err := svc.LoginTOTP("203.0.113.66", pw, codeAt(t, secret, *now)); !errors.Is(err, ErrTOTPLocked) {
		t.Fatalf("expected attacker IP to be locked, got %v", err)
	}
	if _, err := svc.LoginTOTP("198.51.100.10", pw, codeAt(t, secret, *now)); err != nil {
		t.Fatalf("expected admin IP to be unaffected by another IP's lockout, got %v", err)
	}
}

func TestLoginTOTP_GlobalBudgetPausesAndRecovers(t *testing.T) {
	const pw = "correct horse battery staple"
	svc, secret, now := newClockedService(t, pw)

	// A botnet: one failure each from globalMaxFailures different addresses.
	start := *now
	for i := 0; i < globalMaxFailures; i++ {
		_, err := svc.LoginTOTP(fmt.Sprintf("10.0.%d.%d", i/250, i%250+1), pw, "000000")
		if !errors.Is(err, ErrTOTPInvalid) {
			t.Fatalf("botnet failure %d: expected ErrTOTPInvalid, got %v", i+1, err)
		}
		*now = now.Add(time.Second)
	}

	// Even a fresh address with correct credentials is paused, before evaluation.
	_, err := svc.LoginTOTP("198.51.100.10", pw, codeAt(t, secret, *now))
	wantRetry := start.Add(globalFailureWindow).Sub(*now)
	expectLoginErr(t, err, ErrTOTPPaused, wantRetry, "global pause")

	// Sessions created by other means (passkeys) are unaffected by the pause.
	if _, err := svc.CreateSession(); err != nil {
		t.Fatalf("expected passkey-style session creation to keep working, got %v", err)
	}

	// Once the oldest failure ages out, sign-in resumes.
	*now = now.Add(wantRetry)
	if _, err := svc.LoginTOTP("198.51.100.10", pw, codeAt(t, secret, *now)); err != nil {
		t.Fatalf("expected sign-in to resume after the window, got %v", err)
	}
}
