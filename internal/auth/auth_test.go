package auth_test

import (
	"crypto/ed25519"
	"crypto/rand"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/oxri/oxri-go/internal/auth"
	"golang.org/x/crypto/ssh"
)

func generateTestKeys(t *testing.T) (ssh.Signer, string) {
	t.Helper()
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("failed to generate ed25519 key: %v", err)
	}

	signer, err := ssh.NewSignerFromKey(priv)
	if err != nil {
		t.Fatalf("failed to create signer: %v", err)
	}

	pubKeyStr := string(ssh.MarshalAuthorizedKey(signer.PublicKey()))
	return signer, pubKeyStr
}

func getFreePort(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to get free port: %v", err)
	}
	defer func() { _ = l.Close() }()
	return l.Addr().String()
}

func TestAuthService_FullLifecycle(t *testing.T) {
	signer, pubKeyStr := generateTestKeys(t)
	sshAddr := getFreePort(t)

	svc, err := auth.NewService(pubKeyStr, "", sshAddr)
	if err != nil {
		t.Fatalf("NewService failed: %v", err)
	}
	defer func() { _ = svc.Close() }()

	if err := svc.StartSSHListener(); err != nil {
		t.Fatalf("StartSSHListener failed: %v", err)
	}

	// 1. Create Challenge
	token, binding, err := svc.CreateChallenge("127.0.0.1", "/pilot/user/settings")
	if err != nil {
		t.Fatalf("CreateChallenge failed: %v", err)
	}
	if len(token) != 6 {
		t.Errorf("expected 6-digit token, got %q", token)
	}

	// 2. Redemption fails if binding cookie is wrong
	_, _, err = svc.RedeemChallenge(token, "wrong-binding-cookie")
	if err != auth.ErrInvalidBinding {
		t.Errorf("expected ErrInvalidBinding, got %v", err)
	}

	// 3. Redemption fails if not yet approved
	_, _, err = svc.RedeemChallenge(token, binding)
	if err != auth.ErrNotApproved {
		t.Errorf("expected ErrNotApproved, got %v", err)
	}

	// 4. SSH Approval via client
	clientConfig := &ssh.ClientConfig{
		User: token,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         5 * time.Second,
	}

	client, err := ssh.Dial("tcp", sshAddr, clientConfig)
	if err != nil {
		t.Fatalf("ssh.Dial failed: %v", err)
	}
	defer func() { _ = client.Close() }()

	session, err := client.NewSession()
	if err != nil {
		t.Fatalf("client.NewSession failed: %v", err)
	}
	_ = session.Run(token)
	_ = session.Close()

	// 5. Redemption succeeds with correct binding
	sessionID, redirectURL, err := svc.RedeemChallenge(token, binding)
	if err != nil {
		t.Fatalf("RedeemChallenge failed after approval: %v", err)
	}
	if redirectURL != "/pilot/user/settings" {
		t.Errorf("expected redirect /pilot/user/settings, got %q", redirectURL)
	}
	if sessionID == "" {
		t.Fatalf("expected non-empty sessionID")
	}

	// 6. Challenge cannot be redeemed a second time
	_, _, err = svc.RedeemChallenge(token, binding)
	if err != auth.ErrChallengeNotFound {
		t.Errorf("expected ErrChallengeNotFound on replay, got %v", err)
	}

	// 7. Authenticate returns valid AdminCap
	cap, ok := svc.Authenticate(sessionID)
	if !ok {
		t.Fatalf("Authenticate returned false for valid session")
	}
	if cap.IsZero() {
		t.Errorf("Authenticate returned zero AdminCap")
	}

	// 8. RevokeSession terminates session
	svc.RevokeSession(sessionID)
	_, ok = svc.Authenticate(sessionID)
	if ok {
		t.Errorf("Authenticate returned true after RevokeSession")
	}
}

func TestAuthService_RejectsGenericUsernames(t *testing.T) {
	signer, pubKeyStr := generateTestKeys(t)
	sshAddr := getFreePort(t)

	svc, err := auth.NewService(pubKeyStr, "", sshAddr)
	if err != nil {
		t.Fatalf("NewService failed: %v", err)
	}
	defer func() { _ = svc.Close() }()

	if err := svc.StartSSHListener(); err != nil {
		t.Fatalf("StartSSHListener failed: %v", err)
	}

	// Try approving with generic "admin" username
	clientConfig := &ssh.ClientConfig{
		User: "admin",
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         5 * time.Second,
	}

	client, err := ssh.Dial("tcp", sshAddr, clientConfig)
	if err != nil {
		t.Fatalf("ssh.Dial failed: %v", err)
	}
	defer func() { _ = client.Close() }()

	session, err := client.NewSession()
	if err != nil {
		t.Fatalf("client.NewSession failed: %v", err)
	}
	out, _ := session.CombinedOutput("")
	_ = session.Close()

	if !strings.Contains(string(out), "Invalid challenge format") {
		t.Errorf("expected rejection of generic admin username, got: %s", string(out))
	}
}
