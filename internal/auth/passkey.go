package auth

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
)

const (
	// PasskeyCeremonyCookieName binds an in-flight WebAuthn registration or login ceremony
	// to the browser that initiated it.
	PasskeyCeremonyCookieName = "res_imperii_webauthn"

	webAuthnCeremonyTTL = 5 * time.Minute
)

var (
	ErrPasskeyNoCreds  = errors.New("auth: no passkeys enrolled for this domain")
	ErrPasskeyCeremony = errors.New("auth: passkey ceremony not found or expired")
	ErrPasskeyCloned   = errors.New("auth: authenticator appears to be cloned")
)

type pendingWebAuthnCeremony struct {
	ID        string
	RPID      string
	Kind      string // "register" or "login"
	Session   *webauthn.SessionData
	ExpiresAt time.Time
}

// pilotWebAuthnUser implements webauthn.User for the single Res Imperii administrator.
type pilotWebAuthnUser struct {
	creds []webauthn.Credential
}

func (u pilotWebAuthnUser) WebAuthnID() []byte                         { return []byte("res-imperii-pilot") }
func (u pilotWebAuthnUser) WebAuthnName() string                       { return "pilot" }
func (u pilotWebAuthnUser) WebAuthnDisplayName() string                { return "Pilot" }
func (u pilotWebAuthnUser) WebAuthnCredentials() []webauthn.Credential { return u.creds }

// ResolveWebAuthnOrigin determines the WebAuthn Relying Party ID and allowed origins
// from environment overrides (WEBAUTHN_RPID / WEBAUTHN_ORIGIN) or the incoming request.
func ResolveWebAuthnOrigin(r *http.Request, isHTTPS bool) (rpID string, origins []string, supported bool) {
	if envRPID := strings.TrimSpace(os.Getenv("WEBAUTHN_RPID")); envRPID != "" {
		rpID = envRPID
		if envOrig := strings.TrimSpace(os.Getenv("WEBAUTHN_ORIGIN")); envOrig != "" {
			for _, part := range strings.Split(envOrig, ",") {
				if p := strings.TrimSpace(part); p != "" {
					origins = append(origins, p)
				}
			}
		} else {
			origins = []string{"https://" + rpID}
		}
		return rpID, origins, true
	}

	rawHost := strings.TrimSpace(r.Host)
	if rawHost == "" {
		return "", nil, false
	}

	host := rawHost
	if h, _, err := net.SplitHostPort(rawHost); err == nil {
		host = h
	}
	host = strings.ToLower(strings.TrimSpace(host))

	// WebAuthn explicitly allows "localhost" over plain HTTP for local development.
	if host == "localhost" {
		return "localhost", []string{"http://" + rawHost, "https://" + rawHost}, true
	}

	// Raw IP addresses (e.g. 127.0.0.1) are rejected by the WebAuthn specification.
	if net.ParseIP(host) != nil {
		return "", nil, false
	}

	if isHTTPS {
		return host, []string{"https://" + rawHost}, true
	}

	return "", nil, false
}

func newWebAuthnInstance(rpID string, origins []string) (*webauthn.WebAuthn, error) {
	return webauthn.New(&webauthn.Config{
		RPDisplayName: "Res Imperii",
		RPID:          rpID,
		RPOrigins:     origins,
		Timeouts: webauthn.TimeoutsConfig{
			Login: webauthn.TimeoutConfig{
				Enforce:    true,
				Timeout:    2 * time.Minute,
				TimeoutUVD: 2 * time.Minute,
			},
			Registration: webauthn.TimeoutConfig{
				Enforce:    true,
				Timeout:    2 * time.Minute,
				TimeoutUVD: 2 * time.Minute,
			},
		},
	})
}

// CredentialIDString encodes a raw WebAuthn credential ID to URL-safe base64url.
func CredentialIDString(id []byte) string {
	return base64.RawURLEncoding.EncodeToString(id)
}

// MarshalCredential serializes a WebAuthn credential to JSON for Turso storage.
func MarshalCredential(cred *webauthn.Credential) (string, error) {
	b, err := json.Marshal(cred)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// UnmarshalCredentials decodes persisted JSON credentials, skipping any malformed rows.
func UnmarshalCredentials(rawList []string) []webauthn.Credential {
	out := make([]webauthn.Credential, 0, len(rawList))
	for _, raw := range rawList {
		var c webauthn.Credential
		if err := json.Unmarshal([]byte(raw), &c); err == nil && len(c.ID) > 0 {
			out = append(out, c)
		}
	}
	return out
}

func (s *Service) saveWebAuthnCeremony(rpID, kind string, session *webauthn.SessionData) (string, error) {
	idBytes := make([]byte, 24)
	if _, err := io.ReadFull(rand.Reader, idBytes); err != nil {
		return "", err
	}
	id := hex.EncodeToString(idBytes)

	s.mu.Lock()
	defer s.mu.Unlock()
	s.cleanupExpiredLocked()
	for len(s.pendingWebAuthn) >= maxPendingWebAuthn {
		var oldestID string
		var oldestExp time.Time
		for existingID, c := range s.pendingWebAuthn {
			if oldestID == "" || c.ExpiresAt.Before(oldestExp) {
				oldestID = existingID
				oldestExp = c.ExpiresAt
			}
		}
		if oldestID == "" {
			break
		}
		delete(s.pendingWebAuthn, oldestID)
	}
	s.pendingWebAuthn[id] = &pendingWebAuthnCeremony{
		ID:        id,
		RPID:      rpID,
		Kind:      kind,
		Session:   session,
		ExpiresAt: s.now().Add(webAuthnCeremonyTTL),
	}
	return id, nil
}

func (s *Service) consumeWebAuthnCeremony(id, rpID, kind string) (*webauthn.SessionData, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	c, exists := s.pendingWebAuthn[id]
	if !exists {
		return nil, ErrPasskeyCeremony
	}
	delete(s.pendingWebAuthn, id)

	if s.now().After(c.ExpiresAt) || c.RPID != rpID || c.Kind != kind || c.Session == nil {
		return nil, ErrPasskeyCeremony
	}
	return c.Session, nil
}

// BeginPasskeyRegistration starts a WebAuthn credential creation ceremony for an authenticated pilot.
func (s *Service) BeginPasskeyRegistration(rpID string, origins []string, existingCreds []webauthn.Credential) (string, *protocol.CredentialCreation, error) {
	wa, err := newWebAuthnInstance(rpID, origins)
	if err != nil {
		return "", nil, fmt.Errorf("failed to configure WebAuthn: %w", err)
	}

	user := pilotWebAuthnUser{creds: existingCreds}
	exclude := make([]protocol.CredentialDescriptor, 0, len(existingCreds))
	for _, c := range existingCreds {
		exclude = append(exclude, protocol.CredentialDescriptor{
			Type:         protocol.PublicKeyCredentialType,
			CredentialID: c.ID,
		})
	}

	opts, session, err := wa.BeginRegistration(
		user,
		webauthn.WithExclusions(exclude),
		webauthn.WithResidentKeyRequirement(protocol.ResidentKeyRequirementPreferred),
		webauthn.WithAuthenticatorSelection(protocol.AuthenticatorSelection{
			UserVerification: protocol.VerificationPreferred,
		}),
	)
	if err != nil {
		return "", nil, err
	}

	ceremonyID, err := s.saveWebAuthnCeremony(rpID, "register", session)
	if err != nil {
		return "", nil, err
	}
	return ceremonyID, opts, nil
}

// FinishPasskeyRegistration validates the browser's attestation response and returns the new credential.
func (s *Service) FinishPasskeyRegistration(ceremonyID, rpID string, origins []string, existingCreds []webauthn.Credential, r *http.Request) (*webauthn.Credential, error) {
	session, err := s.consumeWebAuthnCeremony(ceremonyID, rpID, "register")
	if err != nil {
		return nil, err
	}

	wa, err := newWebAuthnInstance(rpID, origins)
	if err != nil {
		return nil, err
	}

	user := pilotWebAuthnUser{creds: existingCreds}
	cred, err := wa.FinishRegistration(user, *session, r)
	if err != nil {
		return nil, err
	}
	return cred, nil
}

// BeginPasskeyLogin starts a WebAuthn assertion ceremony for enrolled credentials on rpID.
func (s *Service) BeginPasskeyLogin(rpID string, origins []string, existingCreds []webauthn.Credential) (string, *protocol.CredentialAssertion, error) {
	if len(existingCreds) == 0 {
		return "", nil, ErrPasskeyNoCreds
	}

	wa, err := newWebAuthnInstance(rpID, origins)
	if err != nil {
		return "", nil, fmt.Errorf("failed to configure WebAuthn: %w", err)
	}

	user := pilotWebAuthnUser{creds: existingCreds}
	opts, session, err := wa.BeginLogin(
		user,
		webauthn.WithUserVerification(protocol.VerificationPreferred),
	)
	if err != nil {
		return "", nil, err
	}

	ceremonyID, err := s.saveWebAuthnCeremony(rpID, "login", session)
	if err != nil {
		return "", nil, err
	}
	return ceremonyID, opts, nil
}

// FinishPasskeyLogin verifies the browser's WebAuthn assertion, checks for cloned authenticators,
// and mints a fresh 256-bit authenticated HTTP session.
func (s *Service) FinishPasskeyLogin(ceremonyID, rpID string, origins []string, existingCreds []webauthn.Credential, r *http.Request) (*webauthn.Credential, string, error) {
	session, err := s.consumeWebAuthnCeremony(ceremonyID, rpID, "login")
	if err != nil {
		return nil, "", err
	}

	wa, err := newWebAuthnInstance(rpID, origins)
	if err != nil {
		return nil, "", err
	}

	user := pilotWebAuthnUser{creds: existingCreds}
	cred, err := wa.FinishLogin(user, *session, r)
	if err != nil {
		return nil, "", err
	}
	if cred.Authenticator.CloneWarning {
		return nil, "", ErrPasskeyCloned
	}

	sessionID, err := s.CreateSession()
	if err != nil {
		return nil, "", err
	}
	return cred, sessionID, nil
}
