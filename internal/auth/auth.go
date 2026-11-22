// Package auth provides cryptographic loopback SSH challenge verification
// and HTTP session authentication for Res Imperii.
package auth

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/oxri/oxri-go/internal/domain"
	"golang.org/x/crypto/ssh"
)

const (
	// SessionCookieName is the HTTP cookie name holding the authenticated session token.
	SessionCookieName = "res_imperii_session"

	// BindingCookieName is the HTTP cookie binding a login challenge to the specific browser that initiated it.
	BindingCookieName = "res_imperii_login_binding"

	// PendingLoginDuration is how long an authentication challenge remains valid.
	PendingLoginDuration = 5 * time.Minute

	// SessionDuration is how long an authenticated HTTP session remains valid.
	SessionDuration = 24 * time.Hour

	// MaxConcurrentSSHConns limits simultaneous SSH handshakes.
	MaxConcurrentSSHConns = 10
)

var (
	ErrUnauthorized     = errors.New("auth: unauthorized")
	ErrChallengeExpired = errors.New("auth: challenge has expired")
	ErrChallengeNotFound = errors.New("auth: challenge not found")
	ErrNotApproved      = errors.New("auth: challenge not yet approved")
	ErrInvalidBinding   = errors.New("auth: challenge binding verification failed")
)

// PendingLogin tracks an unauthenticated challenge waiting for SSH signature.
type PendingLogin struct {
	Token       string
	BindingHash []byte
	CreatedAt   time.Time
	ExpiresAt   time.Time
	Approved    bool
	ApprovedBy  string
	RedirectURL string
}

// UserSession represents an active authenticated administrative HTTP session.
type UserSession struct {
	SessionID string
	CreatedAt time.Time
	ExpiresAt time.Time
}

// Service manages cryptographic authentication challenges and sessions.
type Service struct {
	mu                   sync.RWMutex
	pendingLogins        map[string]*PendingLogin
	activeSessions       map[string]*UserSession
	authorizedKey        ssh.PublicKey
	authorizedKeyComment string
	authorizedKeyType    string
	hostSigner           ssh.Signer
	sshAddr              string
	listener             net.Listener
	capMinter            domain.CapMinter
	closed               bool
}

// NewService initializes the authentication service.
func NewService(authorizedKeyStr, hostKeyPath, sshAddr string) (*Service, error) {
	if strings.TrimSpace(authorizedKeyStr) == "" {
		return nil, errors.New("ADMIN_SSH_PUBKEY is required")
	}
	if sshAddr == "" {
		sshAddr = "127.0.0.1:2222"
	}

	pubKey, comment, _, _, err := ssh.ParseAuthorizedKey([]byte(authorizedKeyStr))
	if err != nil {
		return nil, fmt.Errorf("invalid authorized public key: %w", err)
	}

	hostSigner, err := loadOrGenerateHostKey(hostKeyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare host key: %w", err)
	}

	// Mint capability creator from domain (single-use across process lifetime)
	minter := domain.SetupCapMinter()

	return &Service{
		pendingLogins:        make(map[string]*PendingLogin),
		activeSessions:       make(map[string]*UserSession),
		authorizedKey:        pubKey,
		authorizedKeyComment: comment,
		authorizedKeyType:    pubKey.Type(),
		hostSigner:           hostSigner,
		sshAddr:              sshAddr,
		capMinter:            minter,
	}, nil
}

// CreateChallenge generates a new 6-digit challenge code and an unguessable 256-bit
// browser binding cookie. The server only stores the SHA-256 hash of the binding cookie,
// guaranteeing that only the initiating browser can ever redeem the approved session.
func (s *Service) CreateChallenge(clientIP, redirectURL string) (token string, bindingCookie string, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.cleanupExpiredLocked()

	// Generate 6-digit challenge code (100000 - 999999)
	n, err := rand.Int(rand.Reader, big.NewInt(900000))
	if err != nil {
		return "", "", err
	}
	token = fmt.Sprintf("%06d", n.Int64()+100000)

	// Generate 256-bit random binding secret
	bindingBytes := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, bindingBytes); err != nil {
		return "", "", err
	}
	bindingCookie = hex.EncodeToString(bindingBytes)

	bindingHash := sha256.Sum256([]byte(bindingCookie))

	now := time.Now().UTC()
	s.pendingLogins[token] = &PendingLogin{
		Token:       token,
		BindingHash: bindingHash[:],
		CreatedAt:   now,
		ExpiresAt:   now.Add(PendingLoginDuration),
		RedirectURL: redirectURL,
	}

	return token, bindingCookie, nil
}

// RedeemChallenge verifies the browser binding and checks if the challenge was cryptographically
// signed by the authorized SSH key. Upon success, the challenge is consumed immediately
// and an authenticated administrative session is created.
func (s *Service) RedeemChallenge(token, bindingCookie string) (sessionID string, redirectURL string, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	challenge, exists := s.pendingLogins[token]
	if !exists {
		return "", "", ErrChallengeNotFound
	}

	if time.Now().UTC().After(challenge.ExpiresAt) {
		delete(s.pendingLogins, token)
		return "", "", ErrChallengeExpired
	}

	// Verify that the browser presenting this request is the one that initiated the challenge
	providedHash := sha256.Sum256([]byte(bindingCookie))
	if subtle.ConstantTimeCompare(providedHash[:], challenge.BindingHash) != 1 {
		return "", "", ErrInvalidBinding
	}

	if !challenge.Approved {
		return "", "", ErrNotApproved
	}

	// Challenge successfully redeemed: delete challenge so it can never be reused
	redirectURL = challenge.RedirectURL
	delete(s.pendingLogins, token)

	// Create authenticated session
	sessBytes := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, sessBytes); err != nil {
		return "", "", err
	}
	sessionID = hex.EncodeToString(sessBytes)

	now := time.Now().UTC()
	s.activeSessions[sessionID] = &UserSession{
		SessionID: sessionID,
		CreatedAt: now,
		ExpiresAt: now.Add(SessionDuration),
	}

	return sessionID, redirectURL, nil
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

// RevokeSession invalidates an active session.
func (s *Service) RevokeSession(sessionID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.activeSessions, sessionID)
}

// KeyType returns the public key algorithm (e.g. ssh-ed25519) of the configured admin key.
func (s *Service) KeyType() string {
	return s.authorizedKeyType
}

// SSHAddr returns the configured SSH bind address.
func (s *Service) SSHAddr() string {
	return s.sshAddr
}

// StartSSHListener starts the loopback SSH challenge receiver.
func (s *Service) StartSSHListener() error {
	config := &ssh.ServerConfig{
		PublicKeyCallback: func(conn ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
			if !bytes.Equal(key.Marshal(), s.authorizedKey.Marshal()) {
				return nil, errors.New("unauthorized public key")
			}
			return &ssh.Permissions{
				Extensions: map[string]string{
					"pubkey-fp": ssh.FingerprintSHA256(key),
				},
			}, nil
		},
		MaxAuthTries: 3,
	}
	config.AddHostKey(s.hostSigner)

	listener, err := net.Listen("tcp", s.sshAddr)
	if err != nil {
		return fmt.Errorf("failed to listen on SSH address %s: %w", s.sshAddr, err)
	}

	s.mu.Lock()
	s.listener = listener
	s.mu.Unlock()

	go s.acceptLoop(listener, config)
	return nil
}

// Close stops the SSH listener and terminates active sessions.
func (s *Service) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.closed = true
	if s.listener != nil {
		return s.listener.Close()
	}
	return nil
}

func (s *Service) acceptLoop(listener net.Listener, config *ssh.ServerConfig) {
	sem := make(chan struct{}, MaxConcurrentSSHConns)

	for {
		conn, err := listener.Accept()
		if err != nil {
			s.mu.RLock()
			closed := s.closed
			s.mu.RUnlock()
			if closed {
				return
			}
			continue
		}

		sem <- struct{}{}
		go func(c net.Conn) {
			defer func() { <-sem }()
			s.handleSSHConn(c, config)
		}(conn)
	}
}

func (s *Service) handleSSHConn(netConn net.Conn, config *ssh.ServerConfig) {
	// Set 10-second handshake deadline to prevent slow connection resource exhaustion
	_ = netConn.SetDeadline(time.Now().Add(10 * time.Second))

	sshConn, chans, reqs, err := ssh.NewServerConn(netConn, config)
	if err != nil {
		_ = netConn.Close()
		return
	}
	defer func() { _ = sshConn.Close() }()

	// Clear deadline after handshake succeeds
	_ = netConn.SetDeadline(time.Time{})

	go ssh.DiscardRequests(reqs)

	for newChannel := range chans {
		// Reject everything except session channels
		if newChannel.ChannelType() != "session" {
			_ = newChannel.Reject(ssh.UnknownChannelType, "only session channels supported")
			continue
		}

		channel, requests, err := newChannel.Accept()
		if err != nil {
			continue
		}

		go s.handleSessionChannel(channel, requests, sshConn.User())
	}
}

func (s *Service) handleSessionChannel(ch ssh.Channel, reqs <-chan *ssh.Request, user string) {
	defer func() {
		_, _ = ch.SendRequest("exit-status", false, []byte{0, 0, 0, 0})
		_ = ch.Close()
	}()

	for req := range reqs {
		switch req.Type {
		case "pty-req", "env", "window-change":
			// Silently acknowledge client terminal metadata requests as a no-op
			// so OpenSSH never prints "PTY allocation request failed on channel 0".
			_ = req.Reply(true, nil)

		case "exec":
			var payload struct {
				Command string
			}
			// Command payload is 4-byte length prefix followed by command string
			if len(req.Payload) >= 4 {
				length := int(req.Payload[0])<<24 | int(req.Payload[1])<<16 | int(req.Payload[2])<<8 | int(req.Payload[3])
				if len(req.Payload) >= 4+length {
					payload.Command = string(req.Payload[4 : 4+length])
				}
			}

			_ = req.Reply(true, nil)
			s.processSSHAction(ch, user, strings.TrimSpace(payload.Command))
			return

		case "shell":
			_ = req.Reply(true, nil)
			s.processSSHAction(ch, user, "")
			return

		default:
			_ = req.Reply(false, nil)
		}
	}
}

// processSSHAction matches and approves exact challenge tokens.
// Generic shortcuts like "admin" or "root" are strictly forbidden to eliminate login hijacking.
func (s *Service) processSSHAction(ch ssh.Channel, user, command string) {
	token := strings.TrimSpace(command)
	if token == "" {
		token = strings.TrimSpace(user)
	}

	if token == "" || len(token) != 6 {
		_, _ = fmt.Fprintf(ch, "Invalid challenge format. Usage: ssh -p <port> <6-digit-token>@localhost\r\n")
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	challenge, exists := s.pendingLogins[token]
	if !exists {
		_, _ = fmt.Fprintf(ch, "Challenge %s not found or expired.\r\n", token)
		return
	}

	if time.Now().UTC().After(challenge.ExpiresAt) {
		delete(s.pendingLogins, token)
		_, _ = fmt.Fprintf(ch, "Challenge %s has expired.\r\n", token)
		return
	}

	challenge.Approved = true
	challenge.ApprovedBy = user

	_, _ = fmt.Fprintf(ch, "\r\n[+] Authentication verified for challenge %s.\r\n    Return to your browser to proceed.\r\n\r\n", token)
}

func (s *Service) cleanupExpiredLocked() {
	now := time.Now().UTC()
	for token, challenge := range s.pendingLogins {
		if now.After(challenge.ExpiresAt) {
			delete(s.pendingLogins, token)
		}
	}
	for sessID, sess := range s.activeSessions {
		if now.After(sess.ExpiresAt) {
			delete(s.activeSessions, sessID)
		}
	}
}

func loadOrGenerateHostKey(path string) (ssh.Signer, error) {
	if path != "" {
		if data, err := os.ReadFile(path); err == nil {
			return ssh.ParsePrivateKey(data)
		}
	}

	// Generate ED25519 host key
	_, privKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}

	pkcs8, err := x509.MarshalPKCS8PrivateKey(privKey)
	if err != nil {
		return nil, err
	}

	pemBlock := &pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: pkcs8,
	}
	pemData := pem.EncodeToMemory(pemBlock)

	if path != "" {
		_ = os.MkdirAll(filepath.Dir(path), 0700)
		_ = os.WriteFile(path, pemData, 0600)
	}

	return ssh.ParsePrivateKey(pemData)
}
