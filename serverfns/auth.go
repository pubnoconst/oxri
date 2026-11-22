package serverfns

import (
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/oxri/oxri-go/internal/auth"
	"github.com/oxri/oxri-go/internal/domain"
)

var (
	ErrUnauthenticated = errors.New("serverfns: unauthenticated session")
	ErrStaleSession    = errors.New("serverfns: recent reauthentication required")
	ErrInvalidRedirect = errors.New("serverfns: invalid redirect URL")
)

// Allowed redirect targets to prevent open-redirect vulnerabilities
var allowedRedirectPrefixes = []string{
	"/",
	"/pilot/user/settings",
	"/book-i",
	"/book-ii",
	"/book-iii",
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

// RequireFreshAuth verifies both an active authenticated session and that the session was
// minted within maxAge, protecting sensitive credential management endpoints against
// long-lived stolen session cookies.
func RequireFreshAuth(svc *auth.Service, r *http.Request, maxAge time.Duration) (domain.AdminCap, error) {
	if svc == nil {
		return domain.AdminCap{}, ErrUnauthenticated
	}

	cookie, err := r.Cookie(auth.SessionCookieName)
	if err != nil || cookie.Value == "" {
		return domain.AdminCap{}, ErrUnauthenticated
	}

	cap, valid, fresh := svc.AuthenticateFresh(cookie.Value, maxAge)
	if !valid || cap.IsZero() {
		return domain.AdminCap{}, ErrUnauthenticated
	}
	if !fresh {
		return domain.AdminCap{}, ErrStaleSession
	}

	return cap, nil
}
