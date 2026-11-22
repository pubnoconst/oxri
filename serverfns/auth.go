package serverfns

import (
	"errors"
	"net/http"
	"net/url"
	"strings"

	"github.com/oxri/oxri-go/internal/auth"
	"github.com/oxri/oxri-go/internal/domain"
)

var (
	ErrUnauthenticated = errors.New("serverfns: unauthenticated session")
	ErrInvalidRedirect = errors.New("serverfns: invalid redirect URL")
)

// Allowed redirect targets to prevent open-redirect vulnerabilities
var allowedRedirectPrefixes = []string{
	"/",
	"/pilot/user/settings",
	"/book-i",
	"/journals",
}

// SanitizeRedirect validates that a destination path is a safe relative application path.
func SanitizeRedirect(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "/"
	}

	u, err := url.Parse(raw)
	if err != nil || u.IsAbs() || u.Host != "" {
		return "/"
	}

	p := u.Path
	if !strings.HasPrefix(p, "/") || strings.HasPrefix(p, "//") || strings.Contains(p, `\`) {
		return "/"
	}

	for _, allowed := range allowedRedirectPrefixes {
		if p == allowed || strings.HasPrefix(p, allowed+"/") {
			return p
		}
	}

	return "/"
}

// RequireAuth inspects the incoming HTTP request for a valid session cookie.
// If valid, it returns an unforgeable domain.AdminCap token.
func RequireAuth(svc *auth.Service, r *http.Request) (domain.AdminCap, error) {
	if svc == nil {
		return domain.AdminCap{}, ErrUnauthenticated
	}

	cookie, err := r.Cookie(auth.SessionCookieName)
	if err != nil || cookie.Value == "" {
		return domain.AdminCap{}, ErrUnauthenticated
	}

	cap, ok := svc.Authenticate(cookie.Value)
	if !ok || cap.IsZero() {
		return domain.AdminCap{}, ErrUnauthenticated
	}

	return cap, nil
}
