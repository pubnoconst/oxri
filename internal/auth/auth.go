// Package auth provides RFC 6238 TOTP authentication with mandatory server-side
// confirmation cooldown, single-use replay protection, and HTTP session management
// for Res Imperii.
package auth

import (
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
	totpFailures     []time.Time
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

	return &Service{
		pendingTOTP:      make(map[string]*PendingTOTPVerification),
		pendingWebAuthn:  make(map[string]*pendingWebAuthnCeremony),
		activeSessions:   make(map[string]*UserSession),
		totpSecret:       totpSecret,
		passwordHash:     passwordHash,
		totpConfirmDelay: confirmDelay,
		lastUsedTOTPStep: -1,
		capMinter:        acquireCapMinter(),
	}, nil
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
// before revealing success or failure. Only one pending confirmation may be in-flight at a time,
// capping guesses to 1 per cooldown window globally.
func (s *Service) StartTOTPVerification(password, code, redirectURL string) (verifyID, bindingCookie string, readyAt time.Time, err error) {
	if !s.HasTOTP() {
		return "", "", time.Time{}, ErrTOTPNotConfigured
	}

	now := time.Now().UTC()

	s.mu.Lock()
	defer s.mu.Unlock()

	s.cleanupExpiredLocked()

	// Prevent parallel brute-forcing: reject if another confirmation is currently cooling down
	for _, p := range s.pendingTOTP {
		if now.Before(p.ReadyAt) {
			return "", "", time.Time{}, ErrTOTPPending
		}
	}

	// Prune old failure timestamps outside lockout window
	cutoff := now.Add(-totpLockoutWindow)
	recentFailures := s.totpFailures[:0]
	for _, ts := range s.totpFailures {
		if ts.After(cutoff) {
			recentFailures = append(recentFailures, ts)
		}
	}
	s.totpFailures = recentFailures

	var (
		approved   bool
		failureErr error
	)
	if len(s.totpFailures) >= totpMaxFailures {
		failureErr = ErrTOTPLocked
	} else {
		pwOK := true
		if s.passwordHash != "" {
			pwOK = VerifyPassword(s.passwordHash, password)
		}
		matchedStep, totpOK := VerifyTOTP(s.totpSecret, code, now)
		switch {
		case !pwOK || !totpOK:
			log.Printf("[AUTH] login rejected (pwOK=%t, totpOK=%t, failures=%d)", pwOK, totpOK, len(s.totpFailures)+1)
			s.totpFailures = append(s.totpFailures, now)
			failureErr = ErrTOTPInvalid
		case matchedStep <= s.lastUsedTOTPStep:
			log.Printf("[AUTH] login rejected (replayed TOTP step)")
			failureErr = ErrTOTPReplayed
		default:
			s.lastUsedTOTPStep = matchedStep
			s.totpFailures = nil
			approved = true
		}
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

	readyAt = now.Add(s.totpConfirmDelay)
	s.pendingTOTP[verifyID] = &PendingTOTPVerification{
		ID:          verifyID,
		BindingHash: bindingHash[:],
		CreatedAt:   now,
		ReadyAt:     readyAt,
		ExpiresAt:   readyAt.Add(2 * time.Minute),
		Approved:    approved,
		FailureErr:  failureErr,
		RedirectURL: redirectURL,
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
	now := time.Now().UTC()
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
	s.mu.Lock()
	defer s.mu.Unlock()

	p, exists := s.pendingTOTP[verifyID]
	if !exists {
		return "", "", ErrChallengeNotFound
	}

	now := time.Now().UTC()
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

	sessBytes := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, sessBytes); err != nil {
		return "", "", err
	}
	sessionID = hex.EncodeToString(sessBytes)

	s.activeSessions[sessionID] = &UserSession{
		SessionID: sessionID,
		CreatedAt: now,
		ExpiresAt: now.Add(SessionDuration),
	}

	return sessionID, redirectURL, nil
}

// AuthenticateTOTP verifies a 6-digit RFC 6238 TOTP code immediately with single-use
// replay protection and failed-attempt lockout, returning a new 256-bit session identifier.
func (s *Service) AuthenticateTOTP(code string) (string, error) {
	if !s.HasTOTP() {
		return "", ErrTOTPNotConfigured
	}

	now := time.Now().UTC()

	s.mu.Lock()
	defer s.mu.Unlock()

	s.cleanupExpiredLocked()

	cutoff := now.Add(-totpLockoutWindow)
	recentFailures := s.totpFailures[:0]
	for _, ts := range s.totpFailures {
		if ts.After(cutoff) {
			recentFailures = append(recentFailures, ts)
		}
	}
	s.totpFailures = recentFailures

	if len(s.totpFailures) >= totpMaxFailures {
		return "", ErrTOTPLocked
	}

	matchedStep, ok := VerifyTOTP(s.totpSecret, code, now)
	if !ok {
		s.totpFailures = append(s.totpFailures, now)
		return "", ErrTOTPInvalid
	}

	if matchedStep <= s.lastUsedTOTPStep {
		return "", ErrTOTPReplayed
	}

	s.lastUsedTOTPStep = matchedStep
	s.totpFailures = nil

	sessBytes := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, sessBytes); err != nil {
		return "", err
	}
	sessionID := hex.EncodeToString(sessBytes)

	s.activeSessions[sessionID] = &UserSession{
		SessionID: sessionID,
		CreatedAt: now,
		ExpiresAt: now.Add(SessionDuration),
	}

	return sessionID, nil
}

// Authenticate checks if a session ID is valid and active.
// If valid, it returns an unforgeable domain.AdminCap token.
func (s *Service) Authenticate(sessionID string) (domain.AdminCap, bool) {
	if sessionID == "" {
		return domain.AdminCap{}, false
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	sess, exists := s.activeSessions[sessionID]
	if !exists {
		return domain.AdminCap{}, false
	}

	if time.Now().UTC().After(sess.ExpiresAt) {
		return domain.AdminCap{}, false
	}

	if s.capMinter == nil {
		return domain.AdminCap{}, false
	}

	return s.capMinter(), true
}

// CreateSession mints a new 256-bit random authenticated session and stores it.
func (s *Service) CreateSession() (string, error) {
	sessBytes := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, sessBytes); err != nil {
		return "", err
	}
	sessionID := hex.EncodeToString(sessBytes)
	now := time.Now().UTC()

	s.mu.Lock()
	s.activeSessions[sessionID] = &UserSession{
		SessionID: sessionID,
		CreatedAt: now,
		ExpiresAt: now.Add(SessionDuration),
	}
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
	return nil
}

func (s *Service) cleanupExpiredLocked() {
	now := time.Now().UTC()
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
}
