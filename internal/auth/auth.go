package auth

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
)

const (
	// SessionCookieName is the HTTP cookie holding the authenticated session token.
	SessionCookieName = "res_imperii_session"

	// PendingLoginDuration is how long a challenge token remains valid.
	PendingLoginDuration = 5 * time.Minute

	// SessionDuration is how long an authenticated HTTP session remains valid.
	SessionDuration = 7 * 24 * time.Hour
)

// PendingLogin tracks an unauthenticated challenge waiting for SSH verification.
type PendingLogin struct {
	Token        string
	CreatedAt    time.Time
	ExpiresAt    time.Time
	Approved     bool
	ApprovedBy   string
	RedirectURL  string
	SessionID    string
}

// UserSession represents an active authenticated HTTP session.
type UserSession struct {
	SessionID string
	CreatedAt time.Time
	ExpiresAt time.Time
}

// Service manages loopback SSH authentication and HTTP sessions.
type Service struct {
	mu             sync.RWMutex
	pendingLogins  map[string]*PendingLogin
	activeSessions map[string]*UserSession

	authorizedKey        ssh.PublicKey
	authorizedKeyComment string
	hostSigner           ssh.Signer

	sshAddr  string
	listener net.Listener
	closed   bool
}

// NewService initializes the authentication service with an authorized public key
// and host signer (loaded from hostKeyPath or auto-generated).
func NewService(authorizedKeyStr, hostKeyPath, sshAddr string) (*Service, error) {
	if strings.TrimSpace(authorizedKeyStr) == "" {
		return nil, fmt.Errorf("ADMIN_SSH_PUBKEY is required")
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

	return &Service{
		pendingLogins:        make(map[string]*PendingLogin),
		activeSessions:       make(map[string]*UserSession),
		authorizedKey:        pubKey,
		authorizedKeyComment: comment,
		hostSigner:           hostSigner,
		sshAddr:              sshAddr,
	}, nil
}

// CreatePendingLogin generates a new 6-digit challenge token.
func (s *Service) CreatePendingLogin(redirectURL string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.cleanupExpiredLocked()

	if redirectURL == "" {
		redirectURL = "/"
	}

	// Generate a 6-digit challenge code
	var codeNum uint32
	var b [4]byte
	if _, err := io.ReadFull(rand.Reader, b[:]); err != nil {
		return "", err
	}
	codeNum = (uint32(b[0])<<24 | uint32(b[1])<<16 | uint32(b[2])<<8 | uint32(b[3])) % 900000 + 100000
	token := fmt.Sprintf("%06d", codeNum)

	now := time.Now()
	s.pendingLogins[token] = &PendingLogin{
		Token:       token,
		CreatedAt:   now,
		ExpiresAt:   now.Add(PendingLoginDuration),
		Approved:    false,
		RedirectURL: redirectURL,
	}

	return token, nil
}

// CheckPendingLogin checks if a challenge token has been verified.
func (s *Service) CheckPendingLogin(token string) (approved bool, redirectURL string, found bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	p, ok := s.pendingLogins[token]
	if !ok || time.Now().After(p.ExpiresAt) {
		return false, "", false
	}
	return p.Approved, p.RedirectURL, true
}

// ApprovePendingLogin approves a specific pending challenge token.
func (s *Service) ApprovePendingLogin(token, authorizedBy string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	p, ok := s.pendingLogins[token]
	if !ok || time.Now().After(p.ExpiresAt) {
		return false
	}

	p.Approved = true
	p.ApprovedBy = authorizedBy
	return true
}

// ApproveLatestPendingLogin finds the most recently created unexpired pending login and approves it.
func (s *Service) ApproveLatestPendingLogin(authorizedBy string) (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var latest *PendingLogin
	now := time.Now()
	for _, p := range s.pendingLogins {
		if now.After(p.ExpiresAt) {
			continue
		}
		if !p.Approved {
			if latest == nil || p.CreatedAt.After(latest.CreatedAt) {
				latest = p
			}
		}
	}

	if latest == nil {
		return "", false
	}

	latest.Approved = true
	latest.ApprovedBy = authorizedBy
	return latest.Token, true
}

// ConsumeApprovedLogin exchanges an approved challenge for an active HTTP session ID.
func (s *Service) ConsumeApprovedLogin(token string) (sessionID string, redirectURL string, ok bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	p, exists := s.pendingLogins[token]
	if !exists || !p.Approved || time.Now().After(p.ExpiresAt) {
		return "", "", false
	}

	redirectURL = p.RedirectURL
	delete(s.pendingLogins, token)

	// Generate session ID
	b := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, b); err != nil {
		return "", "", false
	}
	sessionID = hex.EncodeToString(b)

	now := time.Now()
	s.activeSessions[sessionID] = &UserSession{
		SessionID: sessionID,
		CreatedAt: now,
		ExpiresAt: now.Add(SessionDuration),
	}

	return sessionID, redirectURL, true
}

// ValidateSession verifies whether a given HTTP session ID is active and unexpired.
func (s *Service) ValidateSession(sessionID string) bool {
	if sessionID == "" {
		return false
	}
	s.mu.RLock()
	defer s.mu.RUnlock()

	sess, ok := s.activeSessions[sessionID]
	if !ok || time.Now().After(sess.ExpiresAt) {
		return false
	}
	return true
}

// RevokeSession terminates an active session.
func (s *Service) RevokeSession(sessionID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.activeSessions, sessionID)
}

// AuthorizedKeyComment returns the comment or fingerprint of the authorized key.
func (s *Service) AuthorizedKeyComment() string {
	if s.authorizedKeyComment != "" && !strings.Contains(s.authorizedKeyComment, "@") {
		return s.authorizedKeyComment
	}
	return ssh.FingerprintSHA256(s.authorizedKey)
}

// SSHAddr returns the configured SSH listener address.
func (s *Service) SSHAddr() string {
	return s.sshAddr
}

// StartSSH starts the embedded SSH server on the configured address.
func (s *Service) StartSSH() error {
	sshConfig := &ssh.ServerConfig{
		PublicKeyCallback: func(conn ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
			if bytes.Equal(key.Marshal(), s.authorizedKey.Marshal()) {
				return &ssh.Permissions{
					Extensions: map[string]string{
						"pubkey-fp": ssh.FingerprintSHA256(key),
					},
				}, nil
			}
			return nil, fmt.Errorf("public key not authorized")
		},
	}
	sshConfig.AddHostKey(s.hostSigner)

	l, err := net.Listen("tcp", s.sshAddr)
	if err != nil {
		return fmt.Errorf("failed to bind SSH listener on %s: %w", s.sshAddr, err)
	}

	s.mu.Lock()
	s.listener = l
	s.sshAddr = l.Addr().String()
	s.mu.Unlock()

	for {
		conn, err := l.Accept()
		if err != nil {
			s.mu.RLock()
			closed := s.closed
			s.mu.RUnlock()
			if closed {
				return nil
			}
			return err
		}

		go s.handleSSHConn(conn, sshConfig)
	}
}

// StopSSH cleanly closes the SSH listener.
func (s *Service) StopSSH() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = true
	if s.listener != nil {
		return s.listener.Close()
	}
	return nil
}

func (s *Service) handleSSHConn(netConn net.Conn, config *ssh.ServerConfig) {
	defer netConn.Close()

	sshConn, chans, reqs, err := ssh.NewServerConn(netConn, config)
	if err != nil {
		return
	}
	defer sshConn.Close()

	go ssh.DiscardRequests(reqs)

	user := strings.TrimSpace(sshConn.User())
	comment := s.AuthorizedKeyComment()

	for newChannel := range chans {
		if newChannel.ChannelType() != "session" {
			_ = newChannel.Reject(ssh.UnknownChannelType, "unsupported channel type")
			continue
		}

		channel, requests, err := newChannel.Accept()
		if err != nil {
			return
		}

		go s.handleSessionChannel(channel, requests, user, comment)
	}
}

func (s *Service) handleSessionChannel(channel ssh.Channel, requests <-chan *ssh.Request, user, comment string) {
	defer channel.Close()

	for req := range requests {
		switch req.Type {
		case "pty-req":
			_ = req.Reply(true, nil)
		case "shell":
			_ = req.Reply(true, nil)
			s.processSSHAuthAction(channel, user, "", comment)
			sendExitStatus(channel, 0)
			return
		case "exec":
			_ = req.Reply(true, nil)
			cmd := ""
			if len(req.Payload) >= 4 {
				// SSH string payload has 4-byte length prefix
				length := int(req.Payload[0])<<24 | int(req.Payload[1])<<16 | int(req.Payload[2])<<8 | int(req.Payload[3])
				if len(req.Payload) >= 4+length {
					cmd = string(req.Payload[4 : 4+length])
				}
			}
			s.processSSHAuthAction(channel, user, strings.TrimSpace(cmd), comment)
			sendExitStatus(channel, 0)
			return
		case "env":
			_ = req.Reply(true, nil)
		default:
			_ = req.Reply(false, nil)
		}
	}
}

func (s *Service) processSSHAuthAction(w io.Writer, user, cmd, comment string) {
	// Determine token:
	// 1. From exec command argument
	// 2. From username (if 6-digit or non-admin)
	// 3. Fallback: approve latest pending login if username is admin/root/oxri
	targetToken := cmd
	if targetToken == "" && user != "admin" && user != "root" && user != "oxri" && user != "" {
		targetToken = user
	}

	if targetToken != "" {
		if s.ApprovePendingLogin(targetToken, comment) {
			_, _ = fmt.Fprintf(w, "Authenticated. Session %s approved.\r\n", targetToken)
			return
		}
		_, _ = fmt.Fprintf(w, "Challenge token %s not found or expired.\r\n", targetToken)
		return
	}

	// Try approving latest pending
	if token, ok := s.ApproveLatestPendingLogin(comment); ok {
		_, _ = fmt.Fprintf(w, "Authenticated. Session %s approved.\r\n", token)
		return
	}

	_, _ = io.WriteString(w, "No pending login session found. Open /login first.\r\n")
}

func sendExitStatus(channel ssh.Channel, status uint32) {
	b := []byte{
		byte(status >> 24),
		byte(status >> 16),
		byte(status >> 8),
		byte(status),
	}
	_, _ = channel.SendRequest("exit-status", false, b)
}

// AuthorizedKeyType returns the algorithm type of the configured authorized public key (e.g. "ssh-ed25519").
func (s *Service) AuthorizedKeyType() string {
	if s == nil || s.authorizedKey == nil {
		return "ssh-ed25519"
	}
	return s.authorizedKey.Type()
}

func (s *Service) cleanupExpiredLocked() {
	now := time.Now()
	for k, v := range s.pendingLogins {
		if now.After(v.ExpiresAt) {
			delete(s.pendingLogins, k)
		}
	}
	for k, v := range s.activeSessions {
		if now.After(v.ExpiresAt) {
			delete(s.activeSessions, k)
		}
	}
}

func loadOrGenerateHostKey(path string) (ssh.Signer, error) {
	if path != "" {
		if data, err := os.ReadFile(path); err == nil {
			signer, err := ssh.ParsePrivateKey(data)
			if err == nil {
				return signer, nil
			}
		}
	}

	// Generate a fresh Ed25519 host key
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("failed to generate host key: %w", err)
	}

	pkcs8Bytes, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal host key: %w", err)
	}

	pemBlock := &pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: pkcs8Bytes,
	}
	pemBytes := pem.EncodeToMemory(pemBlock)

	if path != "" {
		_ = os.MkdirAll(filepath.Dir(path), 0700)
		_ = os.WriteFile(path, pemBytes, 0600)
	}

	return ssh.ParsePrivateKey(pemBytes)
}
