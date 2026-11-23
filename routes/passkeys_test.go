package routes

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/oxri/oxri-go/internal/auth"
	"github.com/oxri/oxri-go/internal/db"
)

func TestPasskeyRoutesAndStorePersistence(t *testing.T) {
	ctx := context.Background()
	store, err := db.Open(ctx, ":memory:", "")
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	defer func() { _ = store.Close() }()

	ver, err := store.SchemaVersion(ctx)
	if err != nil || ver != db.CurrentSchemaVersion {
		t.Fatalf("expected schema version %d, got %d (err=%v)", db.CurrentSchemaVersion, ver, err)
	}

	authSvc, sessionCookie := createTestTOTPAuth(t)
	defer func() { _ = authSvc.Close() }()

	router := NewRouter(Config{
		AssetsDir:   "../assets",
		AuthService: authSvc,
		Store:       store,
	})

	// 1. Unauthenticated /settings/passkeys/begin returns 401
	reqUnauth := httptest.NewRequest(http.MethodPost, "http://localhost:8080/settings/passkeys/begin", nil)
	reqUnauth.Host = "localhost:8080"
	recUnauth := httptest.NewRecorder()
	router.ServeHTTP(recUnauth, reqUnauth)
	if recUnauth.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for unauthenticated passkey registration begin, got %d", recUnauth.Code)
	}

	// 2. Authenticated /settings/passkeys/begin on localhost returns PublicKeyCredentialCreationOptions JSON + ceremony cookie
	reqBegin := httptest.NewRequest(http.MethodPost, "http://localhost:8080/settings/passkeys/begin", nil)
	reqBegin.Host = "localhost:8080"
	reqBegin.AddCookie(sessionCookie)
	recBegin := httptest.NewRecorder()
	router.ServeHTTP(recBegin, reqBegin)
	if recBegin.Code != http.StatusOK {
		t.Fatalf("expected 200 on /settings/passkeys/begin, got %d: %s", recBegin.Code, recBegin.Body.String())
	}
	if !strings.Contains(recBegin.Body.String(), `"publicKey"`) || !strings.Contains(recBegin.Body.String(), `"challenge"`) {
		t.Fatalf("unexpected WebAuthn creation options JSON: %s", recBegin.Body.String())
	}
	hasCeremonyCookie := false
	for _, c := range recBegin.Result().Cookies() {
		if c.Name == auth.PasskeyCeremonyCookieName && c.Value != "" {
			hasCeremonyCookie = true
		}
	}
	if !hasCeremonyCookie {
		t.Fatalf("expected %s cookie on /settings/passkeys/begin", auth.PasskeyCeremonyCookieName)
	}

	// 3. Save a passkey record to Turso and verify /login shows "Sign in with passkey"
	//    and /settings lists it with a Remove action
	adminCap, ok := authSvc.Authenticate(sessionCookie.Value)
	if !ok {
		t.Fatalf("expected the test session to yield an AdminCap")
	}
	if err := store.SavePasskey(ctx, adminCap, db.PasskeyRecord{
		ID:             "cred-test-1",
		RPID:           "localhost",
		Name:           "Proton Pass",
		CredentialJSON: `{"id":"AQID"}`,
		CreatedAt:      time.Now().UTC(),
	}); err != nil {
		t.Fatalf("SavePasskey: %v", err)
	}

	reqLogin := httptest.NewRequest(http.MethodGet, "http://localhost:8080/login", nil)
	reqLogin.Host = "localhost:8080"
	recLogin := httptest.NewRecorder()
	router.ServeHTTP(recLogin, reqLogin)
	if recLogin.Code != http.StatusOK || !strings.Contains(recLogin.Body.String(), "Sign in with passkey") {
		t.Fatalf("expected /login to offer Sign in with passkey when credential is enrolled")
	}

	reqSettings := httptest.NewRequest(http.MethodGet, "http://localhost:8080/settings", nil)
	reqSettings.Host = "localhost:8080"
	reqSettings.AddCookie(sessionCookie)
	recSettings := httptest.NewRecorder()
	router.ServeHTTP(recSettings, reqSettings)
	if recSettings.Code != http.StatusOK || !strings.Contains(recSettings.Body.String(), "Proton Pass") {
		t.Fatalf("expected /settings to list enrolled Proton Pass credential")
	}

	// 4. Deleting the passkey removes it from Turso
	reqDel := httptest.NewRequest(http.MethodPost, "http://localhost:8080/settings/passkeys/cred-test-1/delete", nil)
	reqDel.Host = "localhost:8080"
	reqDel.AddCookie(sessionCookie)
	recDel := httptest.NewRecorder()
	router.ServeHTTP(recDel, reqDel)
	if recDel.Code != http.StatusSeeOther {
		t.Fatalf("expected 303 redirect on passkey delete, got %d", recDel.Code)
	}
	count, err := store.CountPasskeys(ctx, "localhost")
	if err != nil || count != 0 {
		t.Fatalf("expected 0 passkeys after delete, got %d (err=%v)", count, err)
	}
}

func TestPasskeyManagementRequiresFreshAuth(t *testing.T) {
	ctx := context.Background()
	store, err := db.Open(ctx, ":memory:", "")
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	defer func() { _ = store.Close() }()

	secret, err := auth.GenerateTOTPSecret()
	if err != nil {
		t.Fatalf("GenerateTOTPSecret: %v", err)
	}
	now := time.Unix(1700000000, 0).UTC()
	authSvc, err := auth.NewServiceWithOptions(auth.ServiceOptions{
		TOTPSecret: secret,
		NowFunc:    func() time.Time { return now },
	})
	if err != nil {
		t.Fatalf("NewServiceWithOptions: %v", err)
	}
	defer func() { _ = authSvc.Close() }()

	sessID, err := authSvc.CreateSession()
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	sessionCookie := &http.Cookie{
		Name:  auth.SessionCookieName,
		Value: sessID,
		Path:  "/",
	}

	adminCap, ok := authSvc.Authenticate(sessionCookie.Value)
	if !ok {
		t.Fatalf("expected the test session to yield an AdminCap")
	}
	if err := store.SavePasskey(ctx, adminCap, db.PasskeyRecord{
		ID:             "cred-reauth-1",
		RPID:           "localhost",
		Name:           "Hardware Key",
		CredentialJSON: `{"id":"AQID"}`,
		CreatedAt:      now,
	}); err != nil {
		t.Fatalf("SavePasskey: %v", err)
	}

	router := NewRouter(Config{
		AssetsDir:   "../assets",
		AuthService: authSvc,
		Store:       store,
	})

	// 1. Within fresh window (t=0): /settings/passkeys/begin succeeds
	reqFresh := httptest.NewRequest(http.MethodPost, "http://localhost:8080/settings/passkeys/begin", nil)
	reqFresh.Host = "localhost:8080"
	reqFresh.AddCookie(sessionCookie)
	recFresh := httptest.NewRecorder()
	router.ServeHTTP(recFresh, reqFresh)
	if recFresh.Code != http.StatusOK {
		t.Fatalf("expected 200 for fresh passkey registration begin, got %d", recFresh.Code)
	}

	// 2. Advance past FreshAuthWindow (+6m): passkey mutations must be blocked while session remains logged in
	now = now.Add(6 * time.Minute)

	reqStaleBegin := httptest.NewRequest(http.MethodPost, "http://localhost:8080/settings/passkeys/begin", nil)
	reqStaleBegin.Host = "localhost:8080"
	reqStaleBegin.AddCookie(sessionCookie)
	recStaleBegin := httptest.NewRecorder()
	router.ServeHTTP(recStaleBegin, reqStaleBegin)
	if recStaleBegin.Code != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden for stale /settings/passkeys/begin, got %d", recStaleBegin.Code)
	}

	reqStaleFinish := httptest.NewRequest(http.MethodPost, "http://localhost:8080/settings/passkeys/finish", strings.NewReader("{}"))
	reqStaleFinish.Host = "localhost:8080"
	reqStaleFinish.AddCookie(sessionCookie)
	recStaleFinish := httptest.NewRecorder()
	router.ServeHTTP(recStaleFinish, reqStaleFinish)
	if recStaleFinish.Code != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden for stale /settings/passkeys/finish, got %d", recStaleFinish.Code)
	}

	reqStaleDel := httptest.NewRequest(http.MethodPost, "http://localhost:8080/settings/passkeys/cred-reauth-1/delete", nil)
	reqStaleDel.Host = "localhost:8080"
	reqStaleDel.AddCookie(sessionCookie)
	recStaleDel := httptest.NewRecorder()
	router.ServeHTTP(recStaleDel, reqStaleDel)
	if recStaleDel.Code != http.StatusSeeOther || recStaleDel.Header().Get("Location") != "/login?next=/settings&reauth=1" {
		t.Fatalf("expected 303 redirect to reauth on stale delete, got %d loc=%q", recStaleDel.Code, recStaleDel.Header().Get("Location"))
	}
	count, err := store.CountPasskeys(ctx, "localhost")
	if err != nil || count != 1 {
		t.Fatalf("expected passkey to remain intact on stale delete, got count=%d err=%v", count, err)
	}

	// 3. Settings page renders re-verify prompt and /login?reauth=1 renders rather than bouncing away
	reqSettings := httptest.NewRequest(http.MethodGet, "http://localhost:8080/settings", nil)
	reqSettings.Host = "localhost:8080"
	reqSettings.AddCookie(sessionCookie)
	recSettings := httptest.NewRecorder()
	router.ServeHTTP(recSettings, reqSettings)
	if recSettings.Code != http.StatusOK {
		t.Fatalf("expected 200 on stale /settings, got %d", recSettings.Code)
	}
	if !strings.Contains(recSettings.Body.String(), "Re-verify identity") || !strings.Contains(recSettings.Body.String(), "reauth=1") {
		t.Fatalf("expected Re-verify identity link on stale settings page")
	}

	reqReauthLogin := httptest.NewRequest(http.MethodGet, "http://localhost:8080/login?next=/settings&reauth=1", nil)
	reqReauthLogin.Host = "localhost:8080"
	reqReauthLogin.AddCookie(sessionCookie)
	recReauthLogin := httptest.NewRecorder()
	router.ServeHTTP(recReauthLogin, reqReauthLogin)
	if recReauthLogin.Code != http.StatusOK || !strings.Contains(recReauthLogin.Body.String(), "Re-verify your identity to manage security settings") {
		t.Fatalf("expected 200 reauth login screen even with existing session, got %d", recReauthLogin.Code)
	}
}
