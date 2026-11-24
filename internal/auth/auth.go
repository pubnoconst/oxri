// Package auth provides Argon2id password + RFC 6238 TOTP sign-in (instant on
// success; escalating per-IP holds, per-IP lockout and a global failure budget
// on failure; see login.go), persistent single-use replay protection,
// FIDO2/WebAuthn passkeys, and HTTP session management for Res Imperii.
package auth

import (
	"context"
	"crypto/rand"
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

	// SessionDuration is how long an authenticated HTTP session remains valid.
	SessionDuration = 24 * time.Hour

	// FreshAuthWindow is the step-up window during which sensitive security actions
	// (such as adding or removing passkeys) are permitted without re-verifying identity.
	FreshAuthWindow = 5 * time.Minute

	maxPendingWebAuthn = 256
	maxActiveSessions  = 64
	maxTrackedFailIPs  = 2048
)

var (
	ErrUnauthorized = errors.New("auth: unauthorized")
)

// UserSession represents an active authenticated administrative HTTP session.
type UserSession struct {
	SessionID string
	CreatedAt time.Time
	ExpiresAt time.Time
}

// ServiceOptions configures the authentication service.
type ServiceOptions struct {
	TOTPSecret   string
	PasswordHash string
	ReplayStore  TOTPReplayStore
	NowFunc      func() time.Time
}

// Service manages TOTP + optional Argon2id password verification, confirmation cooldowns,
// and active sessions.
type Service struct {
	mu               sync.RWMutex
	pendingWebAuthn  map[string]*pendingWebAuthnCeremony
	activeSessions   map[string]*UserSession
	totpSecret       string
	passwordHash     string
	lastUsedTOTPStep int64
	totpFailuresByIP map[string][]time.Time
	globalFailures   []time.Time
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
		pendingWebAuthn:  make(map[string]*pendingWebAuthnCeremony),
		activeSessions:   make(map[string]*UserSession),
		totpSecret:       totpSecret,
		passwordHash:     passwordHash,
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
	clear(s.pendingWebAuthn)
	clear(s.activeSessions)
	clear(s.totpFailuresByIP)
	s.globalFailures = nil
	return nil
}

func (s *Service) cleanupExpiredLocked() {
	now := s.now()
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
