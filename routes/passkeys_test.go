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

	// 1. Unauthenticated /pilot/passkeys/begin returns 401
	reqUnauth := httptest.NewRequest(http.MethodPost, "http://localhost:8080/pilot/passkeys/begin", nil)
	reqUnauth.Host = "localhost:8080"
	recUnauth := httptest.NewRecorder()
	router.ServeHTTP(recUnauth, reqUnauth)
	if recUnauth.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for unauthenticated passkey registration begin, got %d", recUnauth.Code)
	}

	// 2. Authenticated /pilot/passkeys/begin on localhost returns PublicKeyCredentialCreationOptions JSON + ceremony cookie
	reqBegin := httptest.NewRequest(http.MethodPost, "http://localhost:8080/pilot/passkeys/begin", nil)
	reqBegin.Host = "localhost:8080"
	reqBegin.AddCookie(sessionCookie)
	recBegin := httptest.NewRecorder()
	router.ServeHTTP(recBegin, reqBegin)
	if recBegin.Code != http.StatusOK {
		t.Fatalf("expected 200 on /pilot/passkeys/begin, got %d: %s", recBegin.Code, recBegin.Body.String())
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
		t.Fatalf("expected %s cookie on /pilot/passkeys/begin", auth.PasskeyCeremonyCookieName)
	}

	// 3. Save a passkey record to Turso and verify /login shows "Sign in with Passkey"
	//    and /pilot/user/settings lists it with a Remove action
	if err := store.SavePasskey(ctx, db.PasskeyRecord{
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
	if recLogin.Code != http.StatusOK || !strings.Contains(recLogin.Body.String(), "Sign in with Passkey") {
		t.Fatalf("expected /login to offer Sign in with Passkey when credential is enrolled")
	}

	reqSettings := httptest.NewRequest(http.MethodGet, "http://localhost:8080/pilot/user/settings", nil)
	reqSettings.Host = "localhost:8080"
	reqSettings.AddCookie(sessionCookie)
	recSettings := httptest.NewRecorder()
	router.ServeHTTP(recSettings, reqSettings)
	if recSettings.Code != http.StatusOK || !strings.Contains(recSettings.Body.String(), "Proton Pass") {
		t.Fatalf("expected /pilot/user/settings to list enrolled Proton Pass credential")
	}

	// 4. Deleting the passkey removes it from Turso
	reqDel := httptest.NewRequest(http.MethodPost, "http://localhost:8080/pilot/passkeys/cred-test-1/delete", nil)
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
