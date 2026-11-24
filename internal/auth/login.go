package auth

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"log"
	"time"
)

// Login policy for the password + TOTP fallback. Passkeys never pass through
// here, so nothing below can lock the administrator out of their own site.
//
// Asymmetric by design: a correct password and code sign in immediately; only
// failures cost time. Revealing success instantly leaks nothing (a successful
// attacker holds a session anyway), and every failure looks the same whether
// the password or the code was wrong, so neither factor can be probed alone.
//
// Defences, checked in this order before any expensive Argon2id work:
//  1. Global budget: more than globalMaxFailures failures from all addresses
//     within globalFailureWindow pauses password+code sign-in (botnets that
//     spread guesses across many IPs).
//  2. Per-IP lockout: totpMaxFailures failures within totpLockoutWindow.
//  3. Per-IP hold: after the n-th recent failure the address must wait
//     failureDelay(n) (2s, 4s, 8s, 16s) before its next attempt is evaluated.
//
// All state is in memory (no database writes on failure); the only write on
// this path is the replay watermark, once per successful sign-in.
const (
	failureBaseDelay    = 2 * time.Second
	globalMaxFailures   = 30
	globalFailureWindow = 10 * time.Minute
)

var (
	// ErrTOTPCooling means this address must wait before its next attempt is evaluated.
	ErrTOTPCooling = errors.New("auth: please wait before trying again")
	// ErrTOTPPaused means password+code sign-in is paused by the global failure budget.
	ErrTOTPPaused = errors.New("auth: password sign-in is temporarily paused")
)

// LoginError wraps a sign-in failure with how long the client must wait before
// another attempt will be evaluated (zero when it may retry at once).
type LoginError struct {
	Err        error
	RetryAfter time.Duration
}

func (e *LoginError) Error() string { return e.Err.Error() }
func (e *LoginError) Unwrap() error { return e.Err }

// RetryAfter extracts the wait carried by a LoginError (zero otherwise).
func RetryAfter(err error) time.Duration {
	var le *LoginError
	if errors.As(err, &le) {
		return le.RetryAfter
	}
	return 0
}

// failureDelay is the hold after the n-th recent failure: 2s, 4s, 8s, 16s.
func failureDelay(n int) time.Duration {
	if n <= 0 {
		return 0
	}
	if n > 4 {
		n = 4
	}
	return failureBaseDelay << (n - 1)
}

// LoginTOTP verifies the Argon2id password (when configured) and a 6-digit
// RFC 6238 code for clientIP, returning a new session ID on success. Failures
// return a *LoginError carrying RetryAfter.
func (s *Service) LoginTOTP(clientIP, password, code string) (string, error) {
	return s.login(clientIP, password, code)
}

func (s *Service) login(clientIP, password, code string) (string, error) {
	if !s.HasTOTP() {
		return "", ErrTOTPNotConfigured
	}

	sessBytes := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, sessBytes); err != nil {
		return "", err
	}
	sessionID := hex.EncodeToString(sessBytes)

	ip := normalizeClientIP(clientIP)
	now := s.now()

	s.mu.Lock()
	s.cleanupExpiredLocked()

	// 1. Global budget (botnets). Rejected before any password work.
	if global := s.pruneGlobalFailuresLocked(now); len(global) >= globalMaxFailures {
		retry := global[len(global)-globalMaxFailures].Add(globalFailureWindow).Sub(now)
		s.mu.Unlock()
		return "", &LoginError{Err: ErrTOTPPaused, RetryAfter: retry}
	}

	// 2. Per-IP lockout.
	recent := s.pruneFailuresLocked(ip, now)
	if len(recent) >= totpMaxFailures {
		retry := recent[len(recent)-totpMaxFailures].Add(totpLockoutWindow).Sub(now)
		s.mu.Unlock()
		return "", &LoginError{Err: ErrTOTPLocked, RetryAfter: retry}
	}

	// 3. Per-IP escalating hold after recent failures.
	if n := len(recent); n > 0 {
		if until := recent[n-1].Add(failureDelay(n)); now.Before(until) {
			s.mu.Unlock()
			return "", &LoginError{Err: ErrTOTPCooling, RetryAfter: until.Sub(now)}
		}
	}

	// 4. Evaluate both factors (both always, so neither can be probed alone).
	pwOK := true
	if s.passwordHash != "" {
		pwOK = VerifyPassword(s.passwordHash, password)
	}
	matchedStep, totpOK := VerifyTOTP(s.totpSecret, code, now)

	if !pwOK || !totpOK {
		count := s.recordFailureLocked(ip, now)
		globalCount := s.recordGlobalFailureLocked(now)
		retry := failureDelay(count)
		if count >= totpMaxFailures {
			retry = totpLockoutWindow
		}
		s.mu.Unlock()
		log.Printf("[AUTH] login rejected (ip=%s, pwOK=%t, totpOK=%t, failures=%d, global=%d)", ip, pwOK, totpOK, count, globalCount)
		if globalCount == globalMaxFailures {
			log.Printf("[AUTH] global failure budget reached (%d in %s): password sign-in paused; passkeys unaffected", globalMaxFailures, globalFailureWindow)
		}
		return "", &LoginError{Err: ErrTOTPInvalid, RetryAfter: retry}
	}

	if matchedStep <= s.lastUsedTOTPStep {
		s.mu.Unlock()
		log.Printf("[AUTH] login rejected (ip=%s, replayed TOTP step)", ip)
		return "", &LoginError{Err: ErrTOTPReplayed}
	}

	s.lastUsedTOTPStep = matchedStep
	delete(s.totpFailuresByIP, ip)
	s.storeSessionLocked(sessionID, now)
	s.mu.Unlock()

	s.persistLastTOTPStep(matchedStep)
	return sessionID, nil
}

// pruneGlobalFailuresLocked drops global failures older than the window.
func (s *Service) pruneGlobalFailuresLocked(now time.Time) []time.Time {
	cutoff := now.Add(-globalFailureWindow)
	i := 0
	for i < len(s.globalFailures) && !s.globalFailures[i].After(cutoff) {
		i++
	}
	s.globalFailures = s.globalFailures[i:]
	return s.globalFailures
}

// recordGlobalFailureLocked notes a failure from any address and returns the
// count within the window. Only the newest globalMaxFailures timestamps are
// kept: that is all the budget check needs, and it bounds memory.
func (s *Service) recordGlobalFailureLocked(now time.Time) int {
	s.pruneGlobalFailuresLocked(now)
	s.globalFailures = append(s.globalFailures, now)
	if extra := len(s.globalFailures) - globalMaxFailures; extra > 0 {
		s.globalFailures = append([]time.Time(nil), s.globalFailures[extra:]...)
	}
	return len(s.globalFailures)
}
