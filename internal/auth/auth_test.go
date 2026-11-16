package auth

import (
	"crypto/ed25519"
	"crypto/rand"
	"os"
	"path/filepath"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
)

func TestAuthService_Lifecycle(t *testing.T) {
	// Generate a test authorized key
	pub, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("ed25519.GenerateKey: %v", err)
	}
	sshPub, err := ssh.NewPublicKey(pub)
	if err != nil {
		t.Fatalf("ssh.NewPublicKey: %v", err)
	}
	authKeyStr := string(ssh.MarshalAuthorizedKey(sshPub))

	tmpDir := t.TempDir()
	hostKeyPath := filepath.Join(tmpDir, "host_key")

	svc, err := NewService(authKeyStr, hostKeyPath, "127.0.0.1:0")
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	// 1. Host key should have been saved to disk
	if _, err := os.Stat(hostKeyPath); err != nil {
		t.Fatalf("host key file was not created: %v", err)
	}

	// 2. Create pending login
	token, err := svc.CreatePendingLogin("/journal-editor")
	if err != nil {
		t.Fatalf("CreatePendingLogin: %v", err)
	}
	if len(token) != 6 {
		t.Fatalf("expected 6-digit token, got %q", token)
	}

	approved, redirect, found := svc.CheckPendingLogin(token)
	if !found || approved || redirect != "/journal-editor" {
		t.Fatalf("unexpected pending login state: found=%v, approved=%v, redirect=%v", found, approved, redirect)
	}

	// 3. Approve challenge
	if !svc.ApprovePendingLogin(token, "test-runner") {
		t.Fatalf("failed to approve pending login")
	}

	approved, _, found = svc.CheckPendingLogin(token)
	if !found || !approved {
		t.Fatalf("expected approved pending login")
	}

	// 4. Consume approved challenge
	sessionID, redirectURL, ok := svc.ConsumeApprovedLogin(token)
	if !ok || sessionID == "" || redirectURL != "/journal-editor" {
		t.Fatalf("failed to consume approved login: ok=%v, session=%q, redirect=%q", ok, sessionID, redirectURL)
	}

	// Double consumption must fail
	if _, _, ok := svc.ConsumeApprovedLogin(token); ok {
		t.Fatalf("double consumption should have failed")
	}

	// 5. Session validation
	if !svc.ValidateSession(sessionID) {
		t.Fatalf("session %q should be valid", sessionID)
	}

	// 6. Session revocation
	svc.RevokeSession(sessionID)
	if svc.ValidateSession(sessionID) {
		t.Fatalf("session %q should be revoked", sessionID)
	}
}

func TestAuthService_SSHHandshake(t *testing.T) {
	// Authorized client keypair
	authPub, authPriv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey auth: %v", err)
	}
	authSshPub, err := ssh.NewPublicKey(authPub)
	if err != nil {
		t.Fatalf("NewPublicKey auth: %v", err)
	}
	authKeyStr := string(ssh.MarshalAuthorizedKey(authSshPub))

	// Unauthorized client keypair
	_, unauthPriv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey unauth: %v", err)
	}

	tmpDir := t.TempDir()
	hostKeyPath := filepath.Join(tmpDir, "host_key")

	svc, err := NewService(authKeyStr, hostKeyPath, "127.0.0.1:0")
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	// Start SSH server in background
	go func() {
		_ = svc.StartSSH()
	}()
	defer svc.StopSSH()

	// Wait for listener to bind
	var addr string
	for i := 0; i < 50; i++ {
		svc.mu.RLock()
		l := svc.listener
		svc.mu.RUnlock()
		if l != nil {
			addr = l.Addr().String()
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if addr == "" {
		t.Fatalf("SSH listener did not start in time")
	}

	// Create pending challenge
	token, err := svc.CreatePendingLogin("/journals")
	if err != nil {
		t.Fatalf("CreatePendingLogin: %v", err)
	}

	// Case 1: Unauthorized client key must be rejected
	unauthSigner, err := ssh.NewSignerFromKey(unauthPriv)
	if err != nil {
		t.Fatalf("NewSignerFromKey unauth: %v", err)
	}
	unauthClientConfig := &ssh.ClientConfig{
		User:            token,
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(unauthSigner)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         2 * time.Second,
	}
	_, err = ssh.Dial("tcp", addr, unauthClientConfig)
	if err == nil {
		t.Fatalf("expected dial with unauthorized key to fail, but it succeeded")
	}

	// Challenge should still be unapproved
	approved, _, _ := svc.CheckPendingLogin(token)
	if approved {
		t.Fatalf("token should NOT be approved after failed auth")
	}

	// Case 2: Authorized client key with username = token must approve challenge
	authSigner, err := ssh.NewSignerFromKey(authPriv)
	if err != nil {
		t.Fatalf("NewSignerFromKey auth: %v", err)
	}
	authClientConfig := &ssh.ClientConfig{
		User:            token,
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(authSigner)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         2 * time.Second,
	}

	client, err := ssh.Dial("tcp", addr, authClientConfig)
	if err != nil {
		t.Fatalf("failed to dial with authorized key: %v", err)
	}
	session, err := client.NewSession()
	if err != nil {
		t.Fatalf("client.NewSession: %v", err)
	}
	// Request shell to trigger auth action
	_ = session.Shell()
	_ = session.Wait()
	_ = client.Close()

	// Challenge must now be approved!
	approved, redirect, found := svc.CheckPendingLogin(token)
	if !found || !approved || redirect != "/journals" {
		t.Fatalf("expected token to be approved, got approved=%v, redirect=%v, found=%v", approved, redirect, found)
	}
}

func TestAuthService_SSHAdminUsernameAutoApproval(t *testing.T) {
	authPub, authPriv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey auth: %v", err)
	}
	authSshPub, err := ssh.NewPublicKey(authPub)
	if err != nil {
		t.Fatalf("NewPublicKey auth: %v", err)
	}
	authKeyStr := string(ssh.MarshalAuthorizedKey(authSshPub))

	tmpDir := t.TempDir()
	hostKeyPath := filepath.Join(tmpDir, "host_key")

	svc, err := NewService(authKeyStr, hostKeyPath, "127.0.0.1:0")
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	go func() {
		_ = svc.StartSSH()
	}()
	defer svc.StopSSH()

	var addr string
	for i := 0; i < 50; i++ {
		svc.mu.RLock()
		l := svc.listener
		svc.mu.RUnlock()
		if l != nil {
			addr = l.Addr().String()
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if addr == "" {
		t.Fatalf("SSH listener did not start in time")
	}

	// Create pending challenge
	token, err := svc.CreatePendingLogin("/editor")
	if err != nil {
		t.Fatalf("CreatePendingLogin: %v", err)
	}

	// Connect with username "admin"
	authSigner, err := ssh.NewSignerFromKey(authPriv)
	if err != nil {
		t.Fatalf("NewSignerFromKey auth: %v", err)
	}
	authClientConfig := &ssh.ClientConfig{
		User:            "admin",
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(authSigner)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         2 * time.Second,
	}

	client, err := ssh.Dial("tcp", addr, authClientConfig)
	if err != nil {
		t.Fatalf("failed to dial: %v", err)
	}
	session, err := client.NewSession()
	if err != nil {
		t.Fatalf("client.NewSession: %v", err)
	}
	_ = session.Shell()
	_ = session.Wait()
	_ = client.Close()

	// The single pending challenge must be approved!
	approved, redirect, found := svc.CheckPendingLogin(token)
	if !found || !approved || redirect != "/editor" {
		t.Fatalf("expected pending token to be auto-approved for admin, got approved=%v", approved)
	}
}
