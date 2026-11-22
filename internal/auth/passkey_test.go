package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestResolveWebAuthnOrigin(t *testing.T) {
	// localhost over HTTP is allowed by WebAuthn spec
	reqLocal := httptest.NewRequest(http.MethodGet, "http://localhost:8080/login", nil)
	reqLocal.Host = "localhost:8080"
	rpID, origins, ok := ResolveWebAuthnOrigin(reqLocal, false)
	if !ok || rpID != "localhost" || len(origins) == 0 {
		t.Fatalf("expected localhost to be supported, got ok=%v rpID=%q origins=%v", ok, rpID, origins)
	}

	// Raw IP (127.0.0.1) is rejected
	reqIP := httptest.NewRequest(http.MethodGet, "http://127.0.0.1:8080/login", nil)
	reqIP.Host = "127.0.0.1:8080"
	if _, _, ok := ResolveWebAuthnOrigin(reqIP, false); ok {
		t.Fatalf("expected raw IP 127.0.0.1 to be unsupported for WebAuthn")
	}

	// Public domain over HTTPS (Render) is supported
	reqRender := httptest.NewRequest(http.MethodGet, "https://res-imperii.onrender.com/login", nil)
	reqRender.Host = "res-imperii.onrender.com"
	rpID, origins, ok = ResolveWebAuthnOrigin(reqRender, true)
	if !ok || rpID != "res-imperii.onrender.com" || len(origins) != 1 || origins[0] != "https://res-imperii.onrender.com" {
		t.Fatalf("expected Render HTTPS origin to be supported, got ok=%v rpID=%q origins=%v", ok, rpID, origins)
	}

	// Public domain over plain HTTP is rejected
	if _, _, ok := ResolveWebAuthnOrigin(reqRender, false); ok {
		t.Fatalf("expected public domain over plain HTTP to be unsupported")
	}
}
