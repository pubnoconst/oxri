package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/oxri/oxri-go/internal/auth"
)

func TestDevAutoLoginOnlyOnLoopback(t *testing.T) {
	svc, err := auth.NewServiceWithOptions(auth.ServiceOptions{TOTPSecret: "JBSWY3DPEHPK3PXP"})
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("RENDER", "")
	t.Setenv("APP_ENV", "")
	for _, addr := range []string{"127.0.0.1:8080", "[::1]:8080", "localhost:8080"} {
		if err := checkDevAutoLogin(addr, svc); err != nil {
			t.Errorf("%s refused: %v", addr, err)
		}
	}
	for _, addr := range []string{"0.0.0.0:8080", ":8080", "192.168.1.5:8080", "example.com:80"} {
		if checkDevAutoLogin(addr, svc) == nil {
			t.Errorf("%s allowed; only loopback may auto sign in", addr)
		}
	}
	if checkDevAutoLogin("127.0.0.1:8080", nil) == nil {
		t.Error("allowed without an auth service")
	}
	t.Setenv("RENDER", "true")
	if checkDevAutoLogin("127.0.0.1:8080", svc) == nil {
		t.Error("allowed on Render")
	}
	t.Setenv("RENDER", "")
	t.Setenv("APP_ENV", "prod")
	if checkDevAutoLogin("127.0.0.1:8080", svc) == nil {
		t.Error("allowed as prod")
	}
}

func TestDevAutoLoginSignsRequestsIn(t *testing.T) {
	svc, err := auth.NewServiceWithOptions(auth.ServiceOptions{TOTPSecret: "JBSWY3DPEHPK3PXP"})
	if err != nil {
		t.Fatal(err)
	}
	var signedIn bool
	h := devAutoLogin(svc, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie(auth.SessionCookieName)
		_, signedIn = svc.Authenticate(func() string {
			if err != nil {
				return ""
			}
			return c.Value
		}())
	}))
	req := httptest.NewRequest(http.MethodGet, "/journals/new", nil)
	req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: "stale"})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if !signedIn {
		t.Fatal("request not signed in")
	}
	if len(rec.Result().Cookies()) != 1 {
		t.Error("browser not given the session cookie")
	}
}
