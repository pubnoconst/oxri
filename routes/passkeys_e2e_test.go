package routes

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/fxamacker/cbor/v2"
	"github.com/oxri/oxri-go/internal/auth"
	"github.com/oxri/oxri-go/internal/db"
)

// softAuthenticator is a minimal WebAuthn authenticator for tests: one P-256
// credential, "none" attestation, user presence + verification, and a
// signature counter. It produces exactly what a browser would POST.
type softAuthenticator struct {
	t      *testing.T
	key    *ecdsa.PrivateKey
	credID []byte
	count  uint32
	origin string
}

var b64 = base64.RawURLEncoding

func newSoftAuthenticator(t *testing.T, origin string) *softAuthenticator {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	id := make([]byte, 16)
	_, _ = rand.Read(id)
	return &softAuthenticator{t: t, key: key, credID: id, origin: origin}
}

func (a *softAuthenticator) clientData(typ, challenge string) []byte {
	b, _ := json.Marshal(map[string]any{"type": typ, "challenge": challenge, "origin": a.origin, "crossOrigin": false})
	return b
}

func (a *softAuthenticator) authData(rpID string, flags byte, attested []byte) []byte {
	h := sha256.Sum256([]byte(rpID))
	var buf bytes.Buffer
	buf.Write(h[:])
	buf.WriteByte(flags)
	_ = binary.Write(&buf, binary.BigEndian, a.count)
	buf.Write(attested)
	return buf.Bytes()
}

const (
	flagUP = 0x01 // user present
	flagUV = 0x04 // user verified
	flagAT = 0x40 // attested credential data included
)

// register answers a navigator.credentials.create() challenge.
func (a *softAuthenticator) register(rpID, challenge string) []byte {
	a.t.Helper()
	x := a.key.PublicKey.X.FillBytes(make([]byte, 32))
	y := a.key.PublicKey.Y.FillBytes(make([]byte, 32))
	cose, err := cbor.Marshal(map[int]any{1: 2, 3: -7, -1: 1, -2: x, -3: y}) // EC2, ES256, P-256
	if err != nil {
		a.t.Fatal(err)
	}
	var att bytes.Buffer
	att.Write(make([]byte, 16)) // AAGUID
	_ = binary.Write(&att, binary.BigEndian, uint16(len(a.credID)))
	att.Write(a.credID)
	att.Write(cose)

	attObj, err := cbor.Marshal(map[string]any{
		"fmt": "none", "attStmt": map[string]any{}, "authData": a.authData(rpID, flagUP|flagUV|flagAT, att.Bytes()),
	})
	if err != nil {
		a.t.Fatal(err)
	}
	body, _ := json.Marshal(map[string]any{
		"id": b64.EncodeToString(a.credID), "rawId": b64.EncodeToString(a.credID), "type": "public-key",
		"response": map[string]string{
			"clientDataJSON":    b64.EncodeToString(a.clientData("webauthn.create", challenge)),
			"attestationObject": b64.EncodeToString(attObj),
		},
	})
	return body
}

// assert answers a navigator.credentials.get() challenge, bumping the counter.
func (a *softAuthenticator) assert(rpID, challenge string) []byte {
	a.t.Helper()
	a.count++
	authData := a.authData(rpID, flagUP|flagUV, nil)
	cd := a.clientData("webauthn.get", challenge)
	cdHash := sha256.Sum256(cd)
	digest := sha256.Sum256(append(append([]byte(nil), authData...), cdHash[:]...))
	sig, err := ecdsa.SignASN1(rand.Reader, a.key, digest[:])
	if err != nil {
		a.t.Fatal(err)
	}
	body, _ := json.Marshal(map[string]any{
		"id": b64.EncodeToString(a.credID), "rawId": b64.EncodeToString(a.credID), "type": "public-key",
		"response": map[string]string{
			"clientDataJSON":    b64.EncodeToString(cd),
			"authenticatorData": b64.EncodeToString(authData),
			"signature":         b64.EncodeToString(sig),
			"userHandle":        b64.EncodeToString([]byte("res-imperii-pilot")),
		},
	})
	return body
}

// passkeyClient drives the routes like the browser's passkey.js does.
type passkeyClient struct {
	t      *testing.T
	router http.Handler
}

const passkeyOrigin = "http://localhost:8080"

func (c passkeyClient) post(path string, body []byte, cookies ...*http.Cookie) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, passkeyOrigin+path, bytes.NewReader(body))
	req.Host = "localhost:8080"
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Sec-Fetch-Site", "same-origin")
	req.Header.Set("Origin", passkeyOrigin)
	for _, ck := range cookies {
		if ck != nil {
			req.AddCookie(ck)
		}
	}
	rec := httptest.NewRecorder()
	c.router.ServeHTTP(rec, req)
	return rec
}

// begin returns the challenge, the RP ID, and the ceremony cookie.
func (c passkeyClient) begin(path string, cookies ...*http.Cookie) (challenge, rpID string, ceremony *http.Cookie) {
	c.t.Helper()
	rec := c.post(path, nil, cookies...)
	if rec.Code != http.StatusOK {
		c.t.Fatalf("POST %s: %d %s", path, rec.Code, rec.Body.String())
	}
	var opts struct {
		PublicKey struct {
			Challenge string `json:"challenge"`
			RP        struct {
				ID string `json:"id"`
			} `json:"rp"`
			RPID string `json:"rpId"`
		} `json:"publicKey"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &opts); err != nil {
		c.t.Fatalf("POST %s: bad options JSON: %v", path, err)
	}
	rpID = opts.PublicKey.RP.ID
	if rpID == "" {
		rpID = opts.PublicKey.RPID
	}
	return opts.PublicKey.Challenge, rpID, cookieNamed(rec, auth.PasskeyCeremonyCookieName)
}

func cookieNamed(rec *httptest.ResponseRecorder, name string) *http.Cookie {
	for _, ck := range rec.Result().Cookies() {
		if ck.Name == name && ck.Value != "" {
			return ck
		}
	}
	return nil
}

// TestPasskeyRegistrationAndSignInEndToEnd covers the primary sign-in path:
// enrol a passkey through /settings, then sign in with it through /auth.
func TestPasskeyRegistrationAndSignInEndToEnd(t *testing.T) {
	ctx := context.Background()
	store, err := db.Open(ctx, ":memory:", "")
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	defer func() { _ = store.Close() }()
	authSvc, adminSession := createTestTOTPAuth(t)
	defer func() { _ = authSvc.Close() }()

	c := passkeyClient{t: t, router: NewRouter(Config{Store: store, AuthService: authSvc})}
	device := newSoftAuthenticator(t, passkeyOrigin)

	// --- Registration (fresh admin session required) ---
	challenge, rpID, ceremony := c.begin("/settings/passkeys/begin", adminSession)
	if rpID != "localhost" || ceremony == nil {
		t.Fatalf("registration begin: rpID=%q ceremony=%v", rpID, ceremony)
	}
	rec := c.post("/settings/passkeys/finish?name=Test%20Key", device.register(rpID, challenge), adminSession, ceremony)
	if rec.Code != http.StatusOK {
		t.Fatalf("registration finish: %d %s", rec.Code, rec.Body.String())
	}
	if n, err := store.CountPasskeys(ctx, "localhost"); err != nil || n != 1 {
		t.Fatalf("expected 1 stored passkey, got %d (%v)", n, err)
	}

	// --- Sign-in (no session) ---
	challenge, _, ceremony = c.begin("/auth/passkey/begin")
	signIn := device.assert("localhost", challenge)
	rec = c.post("/auth/passkey/finish?next=/journals", signIn, ceremony)
	if rec.Code != http.StatusOK {
		t.Fatalf("sign-in finish: %d %s", rec.Code, rec.Body.String())
	}
	var out struct {
		Redirect string `json:"redirect"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	session := cookieNamed(rec, auth.SessionCookieName)
	if out.Redirect != "/journals" || session == nil {
		t.Fatalf("sign-in: redirect=%q session=%v", out.Redirect, session)
	}
	if _, ok := authSvc.Authenticate(session.Value); !ok {
		t.Fatalf("passkey session does not authenticate")
	}

	// --- Replaying the same signed assertion: its ceremony is single-use ---
	if rec := c.post("/auth/passkey/finish", signIn, ceremony); rec.Code == http.StatusOK || cookieNamed(rec, auth.SessionCookieName) != nil {
		t.Fatalf("replayed assertion must not sign in, got %d", rec.Code)
	}
}

// TestPasskeySignInRejectsForgeries: a wrong signature, a foreign origin, and a
// cloned authenticator (non-increasing counter) must never yield a session.
func TestPasskeySignInRejectsForgeries(t *testing.T) {
	ctx := context.Background()
	store, err := db.Open(ctx, ":memory:", "")
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	defer func() { _ = store.Close() }()
	authSvc, adminSession := createTestTOTPAuth(t)
	defer func() { _ = authSvc.Close() }()

	c := passkeyClient{t: t, router: NewRouter(Config{Store: store, AuthService: authSvc})}
	device := newSoftAuthenticator(t, passkeyOrigin)
	challenge, rpID, ceremony := c.begin("/settings/passkeys/begin", adminSession)
	if rec := c.post("/settings/passkeys/finish", device.register(rpID, challenge), adminSession, ceremony); rec.Code != http.StatusOK {
		t.Fatalf("registration: %d %s", rec.Code, rec.Body.String())
	}

	mustFail := func(label string, body []byte, ceremony *http.Cookie) {
		t.Helper()
		rec := c.post("/auth/passkey/finish", body, ceremony)
		if rec.Code == http.StatusOK || cookieNamed(rec, auth.SessionCookieName) != nil {
			t.Errorf("%s: expected rejection without a session, got %d", label, rec.Code)
		}
	}

	// 1. Signature by a different key (same credential ID).
	challenge, _, ceremony = c.begin("/auth/passkey/begin")
	impostor := newSoftAuthenticator(t, passkeyOrigin)
	impostor.credID = device.credID
	mustFail("wrong key", impostor.assert("localhost", challenge), ceremony)

	// 2. A phishing origin relaying the challenge.
	challenge, _, ceremony = c.begin("/auth/passkey/begin")
	phish := *device
	phish.origin = "https://res-imperii.example.net"
	mustFail("foreign origin", phish.assert("localhost", challenge), ceremony)

	// 3. Clone detection: a genuine sign-in stores counter n; a second device
	// with the same key presenting a counter <= n is refused.
	challenge, _, ceremony = c.begin("/auth/passkey/begin")
	device.count = 5
	if rec := c.post("/auth/passkey/finish", device.assert("localhost", challenge), ceremony); rec.Code != http.StatusOK {
		t.Fatalf("genuine sign-in: %d %s", rec.Code, rec.Body.String())
	}
	clone := *device
	clone.count = 2 // assert() bumps to 3, below the stored 6
	challenge, _, ceremony = c.begin("/auth/passkey/begin")
	mustFail("cloned authenticator", clone.assert("localhost", challenge), ceremony)
}
