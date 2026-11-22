// Package auth provides RFC 6238 TOTP authentication with mandatory server-side
// confirmation cooldown, persistent single-use replay protection, per-IP lockout,
// FIDO2/WebAuthn passkeys, and HTTP session management for Res Imperii.
package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/oxri/oxri-go/internal/domain"
)

const (
	// SessionCookieName is the HTTP cookie name holding the authenticated session token.
	SessionCookieName = "res_imperii_session"

	// BindingCookieName is the HTTP cookie binding a pending TOTP confirmation to the
	// specific browser that submitted the 6-digit code.
	BindingCookieName = "res_imperii_login_binding"

	// SessionDuration is how long an authenticated HTTP session remains valid.
	SessionDuration = 24 * time.Hour

	// FreshAuthWindow is the step-up window during which sensitive security actions
	// (such as adding or removing passkeys) are permitted without re-verifying identity.
	FreshAuthWindow = 5 * time.Minute

	maxPendingTOTP     = 256
	maxPendingWebAuthn = 256
	maxActiveSessions  = 64
	maxTrackedFailIPs  = 2048
)

var (
	ErrUnauthorized      = errors.New("auth: unauthorized")
	ErrChallengeExpired  = errors.New("auth: confirmation has expired")
	ErrChallengeNotFound = errors.New("auth: confirmation not found")
	ErrInvalidBinding    = errors.New("auth: confirmation binding verification failed")
)

// PendingTOTPVerification tracks a pure TOTP login attempt during its mandatory
// server-side confirmation cooldown so success/failure is never revealed early.
type PendingTOTPVerification struct {
	ID          string
	ClientIP    string
	BindingHash []byte
	CreatedAt   time.Time
	ReadyAt     time.Time
	ExpiresAt   time.Time
	Approved    bool
	FailureErr  error
	RedirectURL string
}

// UserSession represents an active authenticated administrative HTTP session.
type UserSession struct {
	SessionID string
	CreatedAt time.Time
	ExpiresAt time.Time
}

// ServiceOptions configures the authentication service.
type ServiceOptions struct {
	TOTPSecret       string
	PasswordHash     string
	TOTPConfirmDelay time.Duration
	ReplayStore      TOTPReplayStore
	NowFunc          func() time.Time
}

// Service manages TOTP + optional Argon2id password verification, confirmation cooldowns,
// and active sessions.
type Service struct {
	mu               sync.RWMutex
	pendingTOTP      map[string]*PendingTOTPVerification
	pendingWebAuthn  map[string]*pendingWebAuthnCeremony
	activeSessions   map[string]*UserSession
	totpSecret       string
	passwordHash     string
	totpConfirmDelay time.Duration
	lastUsedTOTPStep int64
	totpFailuresByIP map[string][]time.Time
	replayStore      TOTPReplayStore
	nowFunc          func() time.Time
	capMinter        domain.CapMinter
}

var (
	capMinterOnce sync.Once
	pkgCapMinter  domain.CapMinter
)

func acquireCapMinter() domain.CapMinter {
	capMinterOnce.Do(func() {
		pkgCapMinter = domain.SetupCapMinter()
	})
	return pkgCapMinter
}

// NewService initializes the authentication service with an RFC 6238 Base32 TOTP secret.
func NewService(totpSecret string) (*Service, error) {
	return NewServiceWithOptions(ServiceOptions{
		TOTPSecret: totpSecret,
	})
}

// NewServiceWithOptions initializes the authentication service with explicit options.
func NewServiceWithOptions(opts ServiceOptions) (*Service, error) {
	totpSecret := NormalizeTOTPSecret(opts.TOTPSecret)
	if totpSecret == "" {
		return nil, errors.New("ADMIN_TOTP_SECRET is required")
	}
	if _, err := decodeTOTPSecret(totpSecret); err != nil {
		return nil, fmt.Errorf("invalid ADMIN_TOTP_SECRET: %w", err)
	}

	passwordHash := strings.TrimSpace(opts.PasswordHash)
	if passwordHash != "" {
		if err := ValidatePasswordHash(passwordHash); err != nil {
			return nil, fmt.Errorf("invalid ADMIN_PASSWORD_HASH: %w", err)
		}
	}

	confirmDelay := opts.TOTPConfirmDelay
	if confirmDelay <= 0 {
		confirmDelay = DefaultTOTPConfirmDelay
	}

	lastUsedStep := int64(-1)
	if opts.ReplayStore != nil {
		loadCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		persistedStep, err := opts.ReplayStore.LoadLastTOTPStep(loadCtx)
		cancel()
		if err != nil {
			log.Printf("[AUTH] warning: failed to load persisted TOTP replay watermark: %v", err)
		} else if persistedStep > -1 {
			lastUsedStep = persistedStep
		}
	}

	return &Service{
		pendingTOTP:      make(map[string]*PendingTOTPVerification),
		pendingWebAuthn:  make(map[string]*pendingWebAuthnCeremony),
		activeSessions:   make(map[string]*UserSession),
		totpSecret:       totpSecret,
		passwordHash:     passwordHash,
		totpConfirmDelay: confirmDelay,
		lastUsedTOTPStep: lastUsedStep,
		totpFailuresByIP: make(map[string][]time.Time),
		replayStore:      opts.ReplayStore,
		nowFunc:          opts.NowFunc,
		capMinter:        acquireCapMinter(),
	}, nil
}

func (s *Service) now() time.Time {
	if s != nil && s.nowFunc != nil {
		return s.nowFunc().UTC()
	}
	return time.Now().UTC()
}

func normalizeClientIP(ip string) string {
	trimmed := strings.TrimSpace(ip)
	if trimmed == "" {
		return "unknown"
	}
	return trimmed
}

func (s *Service) persistLastTOTPStep(step int64) {
	if s == nil || s.replayStore == nil {
		return
	}
	saveCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := s.replayStore.SaveLastTOTPStep(saveCtx, step); err != nil {
		log.Printf("[AUTH] warning: failed to persist TOTP replay watermark: %v", err)
	}
}

func (s *Service) pruneFailuresLocked(ip string, now time.Time) []time.Time {
	cutoff := now.Add(-totpLockoutWindow)
	list := s.totpFailuresByIP[ip]
	if len(list) == 0 {
		return nil
	}
	recent := list[:0]
	for _, ts := range list {
		if ts.After(cutoff) {
			recent = append(recent, ts)
		}
	}
	if len(recent) == 0 {
		delete(s.totpFailuresByIP, ip)
		return nil
	}
	s.totpFailuresByIP[ip] = recent
	return recent
}

func (s *Service) sweepExpiredFailuresLocked(now time.Time) {
	cutoff := now.Add(-totpLockoutWindow)
	for ip, list := range s.totpFailuresByIP {
		recent := list[:0]
		for _, ts := range list {
			if ts.After(cutoff) {
				recent = append(recent, ts)
			}
		}
		if len(recent) == 0 {
			delete(s.totpFailuresByIP, ip)
		} else {
			s.totpFailuresByIP[ip] = recent
		}
	}
}

func (s *Service) recordFailureLocked(ip string, now time.Time) int {
	if _, exists := s.totpFailuresByIP[ip]; !exists && len(s.totpFailuresByIP) >= maxTrackedFailIPs {
		s.sweepExpiredFailuresLocked(now)
		for len(s.totpFailuresByIP) >= maxTrackedFailIPs {
			var oldestIP string
			var oldestLast time.Time
			for k, times := range s.totpFailuresByIP {
				var latest time.Time
				if len(times) > 0 {
					latest = times[len(times)-1]
				}
				if oldestIP == "" || latest.Before(oldestLast) {
					oldestIP = k
					oldestLast = latest
				}
			}
			if oldestIP == "" {
				break
			}
			delete(s.totpFailuresByIP, oldestIP)
		}
	}

	recent := append(s.pruneFailuresLocked(ip, now), now)
	s.totpFailuresByIP[ip] = recent
	return len(recent)
}

func (s *Service) storeSessionLocked(sessionID string, now time.Time) {
	s.cleanupExpiredLocked()
	for len(s.activeSessions) >= maxActiveSessions {
		var oldestID string
		var oldestTime time.Time
		for id, sess := range s.activeSessions {
			if oldestID == "" || sess.CreatedAt.Before(oldestTime) {
				oldestID = id
				oldestTime = sess.CreatedAt
			}
		}
		if oldestID == "" {
			break
		}
		delete(s.activeSessions, oldestID)
	}
	s.activeSessions[sessionID] = &UserSession{
		SessionID: sessionID,
		CreatedAt: now,
		ExpiresAt: now.Add(SessionDuration),
	}
}

// HasTOTP reports whether RFC 6238 TOTP login is configured.
func (s *Service) HasTOTP() bool {
	return s != nil && s.totpSecret != ""
}

// HasPassword reports whether an Argon2id password hash is configured alongside TOTP.
func (s *Service) HasPassword() bool {
	return s != nil && s.passwordHash != ""
}

// TOTPConfirmDelay returns the configured server-side confirmation wait for pure TOTP logins.
func (s *Service) TOTPConfirmDelay() time.Duration {
	if s == nil || s.totpConfirmDelay <= 0 {
		return DefaultTOTPConfirmDelay
	}
	return s.totpConfirmDelay
}

// StartTOTPVerification evaluates the optional Argon2id password and 6-digit TOTP code
// at submission time and holds the outcome behind a mandatory server-side cooldown (default 30s)
// before revealing success or failure. Only one pending confirmation per client IP may be
// in-flight at a time, and failed-attempt lockouts are isolated per client IP so an attacker
// cannot lock out the legitimate administrator from another address.
func (s *Service) StartTOTPVerification(clientIP, password, code, redirectURL string) (verifyID, bindingCookie string, readyAt time.Time, err error) {
	if !s.HasTOTP() {
		return "", "", time.Time{}, ErrTOTPNotConfigured
	}

	idBytes := make([]byte, 16)
	if _, err := io.ReadFull(rand.Reader, idBytes); err != nil {
		return "", "", time.Time{}, err
	}
	verifyID = hex.EncodeToString(idBytes)

	bindingBytes := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, bindingBytes); err != nil {
		return "", "", time.Time{}, err
	}
	bindingCookie = hex.EncodeToString(bindingBytes)
	bindingHash := sha256.Sum256([]byte(bindingCookie))

	ip := normalizeClientIP(clientIP)
	now := s.now()

	var (
		approved      bool
		stepToPersist int64
	)

	s.mu.Lock()
	s.cleanupExpiredLocked()

	// Prevent parallel brute-forcing per client IP without blocking other IPs
	for _, p := range s.pendingTOTP {
		if p.ClientIP == ip && now.Before(p.ReadyAt) {
			s.mu.Unlock()
			return "", "", time.Time{}, ErrTOTPPending
		}
	}
	if len(s.pendingTOTP) >= maxPendingTOTP {
		s.mu.Unlock()
		return "", "", time.Time{}, ErrTOTPPending
	}

	recentFailures := s.pruneFailuresLocked(ip, now)

	var failureErr error
	if len(recentFailures) >= totpMaxFailures {
		failureErr = ErrTOTPLocked
	} else {
		pwOK := true
		if s.passwordHash != "" {
			pwOK = VerifyPassword(s.passwordHash, password)
		}
		matchedStep, totpOK := VerifyTOTP(s.totpSecret, code, now)
		switch {
		case !pwOK || !totpOK:
			count := s.recordFailureLocked(ip, now)
			log.Printf("[AUTH] login rejected (ip=%s, pwOK=%t, totpOK=%t, failures=%d)", ip, pwOK, totpOK, count)
			failureErr = ErrTOTPInvalid
		case matchedStep <= s.lastUsedTOTPStep:
			log.Printf("[AUTH] login rejected (ip=%s, replayed TOTP step)", ip)
			failureErr = ErrTOTPReplayed
		default:
			s.lastUsedTOTPStep = matchedStep
			delete(s.totpFailuresByIP, ip)
			approved = true
			stepToPersist = matchedStep
		}
	}

	readyAt = now.Add(s.totpConfirmDelay)
	s.pendingTOTP[verifyID] = &PendingTOTPVerification{
		ID:          verifyID,
		ClientIP:    ip,
		BindingHash: bindingHash[:],
		CreatedAt:   now,
		ReadyAt:     readyAt,
		ExpiresAt:   readyAt.Add(2 * time.Minute),
		Approved:    approved,
		FailureErr:  failureErr,
		RedirectURL: redirectURL,
	}
	s.mu.Unlock()

	if approved {
		s.persistLastTOTPStep(stepToPersist)
	}

	return verifyID, bindingCookie, readyAt, nil
}

// PendingTOTPStatus verifies browser binding and returns how much cooldown time remains.
func (s *Service) PendingTOTPStatus(verifyID, bindingCookie string) (remaining time.Duration, err error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	p, exists := s.pendingTOTP[verifyID]
	if !exists {
		return 0, ErrChallengeNotFound
	}
	now := s.now()
	if now.After(p.ExpiresAt) {
		return 0, ErrChallengeExpired
	}
	providedHash := sha256.Sum256([]byte(bindingCookie))
	if subtle.ConstantTimeCompare(providedHash[:], p.BindingHash) != 1 {
		return 0, ErrInvalidBinding
	}
	if now.Before(p.ReadyAt) {
		return p.ReadyAt.Sub(now), nil
	}
	return 0, nil
}

// RedeemTOTPVerification completes a pending TOTP verification once its mandatory
// cooldown has elapsed. Before ReadyAt, it returns ErrTOTPConfirming without leaking
// whether the code succeeded or failed.
func (s *Service) RedeemTOTPVerification(verifyID, bindingCookie string) (sessionID, redirectURL string, err error) {
	sessBytes := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, sessBytes); err != nil {
		return "", "", err
	}
	sessionID = hex.EncodeToString(sessBytes)

	s.mu.Lock()
	defer s.mu.Unlock()

	p, exists := s.pendingTOTP[verifyID]
	if !exists {
		return "", "", ErrChallengeNotFound
	}

	now := s.now()
	if now.After(p.ExpiresAt) {
		delete(s.pendingTOTP, verifyID)
		return "", "", ErrChallengeExpired
	}

	providedHash := sha256.Sum256([]byte(bindingCookie))
	if subtle.ConstantTimeCompare(providedHash[:], p.BindingHash) != 1 {
		return "", "", ErrInvalidBinding
	}

	if now.Before(p.ReadyAt) {
		return "", "", ErrTOTPConfirming
	}

	redirectURL = p.RedirectURL
	delete(s.pendingTOTP, verifyID)

	if !p.Approved {
		if p.FailureErr != nil {
			return "", redirectURL, p.FailureErr
		}
		return "", redirectURL, ErrTOTPInvalid
	}

	s.storeSessionLocked(sessionID, now)

	return sessionID, redirectURL, nil
}

// AuthenticateTOTP verifies a 6-digit RFC 6238 TOTP code immediately with single-use
// replay protection and per-IP failed-attempt lockout, returning a new 256-bit session identifier.
func (s *Service) AuthenticateTOTP(code string) (string, error) {
	return s.AuthenticateTOTPForIP("local", code)
}

// AuthenticateTOTPForIP verifies a 6-digit RFC 6238 TOTP code immediately for clientIP
// with persistent replay protection and per-IP lockout.
func (s *Service) AuthenticateTOTPForIP(clientIP, code string) (string, error) {
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

	recentFailures := s.pruneFailuresLocked(ip, now)
	if len(recentFailures) >= totpMaxFailures {
		s.mu.Unlock()
		return "", ErrTOTPLocked
	}

	matchedStep, ok := VerifyTOTP(s.totpSecret, code, now)
	if !ok {
		s.recordFailureLocked(ip, now)
		s.mu.Unlock()
		return "", ErrTOTPInvalid
	}

	if matchedStep <= s.lastUsedTOTPStep {
		s.mu.Unlock()
		return "", ErrTOTPReplayed
	}

	s.lastUsedTOTPStep = matchedStep
	delete(s.totpFailuresByIP, ip)
	s.storeSessionLocked(sessionID, now)
	s.mu.Unlock()

	s.persistLastTOTPStep(matchedStep)

	return sessionID, nil
}

// Authenticate checks if a session ID is valid and active.
// If valid, it returns an unforgeable domain.AdminCap token.
func (s *Service) Authenticate(sessionID string) (domain.AdminCap, bool) {
	cap, valid, _ := s.AuthenticateFresh(sessionID, FreshAuthWindow)
	return cap, valid
}

// AuthenticateFresh checks if sessionID is valid and whether it was minted within maxAge.
func (s *Service) AuthenticateFresh(sessionID string, maxAge time.Duration) (cap domain.AdminCap, valid bool, fresh bool) {
	if s == nil || sessionID == "" {
		return domain.AdminCap{}, false, false
	}

	if maxAge <= 0 {
		maxAge = FreshAuthWindow
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	sess, exists := s.activeSessions[sessionID]
	if !exists {
		return domain.AdminCap{}, false, false
	}

	now := s.now()
	if now.After(sess.ExpiresAt) {
		return domain.AdminCap{}, false, false
	}

	if s.capMinter == nil {
		return domain.AdminCap{}, false, false
	}

	isFresh := now.Sub(sess.CreatedAt) <= maxAge
	return s.capMinter(), true, isFresh
}

// CreateSession mints a new 256-bit random authenticated session and stores it.
func (s *Service) CreateSession() (string, error) {
	sessBytes := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, sessBytes); err != nil {
		return "", err
	}
	sessionID := hex.EncodeToString(sessBytes)
	now := s.now()

	s.mu.Lock()
	s.storeSessionLocked(sessionID, now)
	s.mu.Unlock()
	return sessionID, nil
}

// RevokeSession invalidates an active session.
func (s *Service) RevokeSession(sessionID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.activeSessions, sessionID)
}

// Close clears in-memory authentication state on shutdown.
func (s *Service) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	clear(s.pendingTOTP)
	clear(s.pendingWebAuthn)
	clear(s.activeSessions)
	clear(s.totpFailuresByIP)
	return nil
}

func (s *Service) cleanupExpiredLocked() {
	now := s.now()
	for id, pending := range s.pendingTOTP {
		if now.After(pending.ExpiresAt) {
			delete(s.pendingTOTP, id)
		}
	}
	for id, ceremony := range s.pendingWebAuthn {
		if now.After(ceremony.ExpiresAt) {
			delete(s.pendingWebAuthn, id)
		}
	}
	for sessID, sess := range s.activeSessions {
		if now.After(sess.ExpiresAt) {
			delete(s.activeSessions, sessID)
		}
	}
	s.sweepExpiredFailuresLocked(now)
}
