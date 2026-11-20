package app

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/oxri/oxri-go/internal/auth"
	"github.com/oxri/oxri-go/internal/pages/login"
)

// sanitizeNextPath validates that a login redirect destination is a safe local
// application path on our allowlist, preventing open-redirect attacks.
func sanitizeNextPath(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" || !strings.HasPrefix(trimmed, "/") || strings.HasPrefix(trimmed, "//") {
		return "/"
	}
	if strings.Contains(trimmed, "\\") || strings.Contains(trimmed, "://") {
		return "/"
	}
	for _, r := range trimmed {
		if r < 0x20 || r == 0x7F {
			return "/"
		}
	}

	u, err := url.Parse(trimmed)
	if err != nil || u.Scheme != "" || u.Host != "" || u.Opaque != "" || u.User != nil {
		return "/"
	}

	cleanPath := u.Path
	allowed := cleanPath == "/" ||
		cleanPath == "/book-i" ||
		cleanPath == "/journals" ||
		strings.HasPrefix(cleanPath, "/journals/") ||
		cleanPath == "/pilot" ||
		strings.HasPrefix(cleanPath, "/pilot/")
	if !allowed {
		return "/"
	}

	if u.RawQuery != "" {
		return cleanPath + "?" + u.RawQuery
	}
	return cleanPath
}

// shouldSetSecureCookie enables the Secure attribute whenever served over HTTPS,
// behind an HTTPS reverse proxy, on any non-loopback domain, or when COOKIE_SECURE=true.
func shouldSetSecureCookie(req *http.Request) bool {
	if req == nil {
		return true
	}
	if req.TLS != nil {
		return true
	}
	if strings.EqualFold(strings.TrimSpace(req.Header.Get("X-Forwarded-Proto")), "https") {
		return true
	}
	envSec := strings.ToLower(strings.TrimSpace(os.Getenv("COOKIE_SECURE")))
	if envSec == "1" || envSec == "true" {
		return true
	}
	return !isLoopbackHost(req.Host)
}

func isLoopbackHost(hostPort string) bool {
	h := strings.TrimSpace(hostPort)
	if h == "" {
		return true
	}
	if host, _, err := net.SplitHostPort(h); err == nil {
		h = host
	}
	h = strings.Trim(h, "[]")
	return strings.EqualFold(h, "localhost") || h == "127.0.0.1" || h == "::1"
}

func handleLogin(authSvc *auth.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		next := sanitizeNextPath(req.URL.Query().Get("next"))

		if auth.IsAuthenticated(req.Context()) {
			http.Redirect(w, req, next, http.StatusSeeOther)
			return
		}

		if authSvc == nil {
			RenderError(w, req, http.StatusServiceUnavailable)
			return
		}

		token, err := authSvc.CreatePendingLogin(next)
		if err != nil {
			if errors.Is(err, auth.ErrRateLimited) {
				RenderErrorWithCause(w, req, http.StatusTooManyRequests, err)
				return
			}
			RenderErrorWithCause(w, req, http.StatusInternalServerError, err)
			return
		}

		sshPort := "2222"
		if addr := authSvc.SSHAddr(); addr != "" {
			if _, p, err := net.SplitHostPort(addr); err == nil && p != "" {
				sshPort = p
			}
		}

		sshCommand := fmt.Sprintf("ssh -p %s %s@localhost", sshPort, token)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_ = login.Page(token, sshCommand, next, sshPort).Render(req.Context(), w)
	}
}

func handleAuthStatus(authSvc *auth.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		if authSvc == nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}

		token := req.URL.Query().Get("token")
		if token == "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		sessionID, redirectURL, ok := authSvc.ConsumeApprovedLogin(token)
		if ok {
			safeRedirect := sanitizeNextPath(redirectURL)
			http.SetCookie(w, &http.Cookie{
				Name:     auth.SessionCookieName,
				Value:    sessionID,
				Path:     "/",
				HttpOnly: true,
				Secure:   shouldSetSecureCookie(req),
				SameSite: http.SameSiteLaxMode,
				MaxAge:   int(auth.SessionDuration.Seconds()),
			})
			w.Header().Set("HX-Redirect", safeRedirect)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("authorized"))
			return
		}

		approved, _, found := authSvc.CheckPendingLogin(token)
		if found && !approved {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("waiting"))
			return
		}

		w.Header().Set("HX-Refresh", "true")
		w.WriteHeader(http.StatusGone)
		_, _ = w.Write([]byte("expired"))
	}
}

func handleLogout(authSvc *auth.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		if authSvc != nil {
			cookie, err := req.Cookie(auth.SessionCookieName)
			if err == nil && cookie.Value != "" {
				authSvc.RevokeSession(cookie.Value)
			}
		}

		http.SetCookie(w, &http.Cookie{
			Name:     auth.SessionCookieName,
			Value:    "",
			Path:     "/",
			HttpOnly: true,
			Secure:   shouldSetSecureCookie(req),
			SameSite: http.SameSiteLaxMode,
			MaxAge:   -1,
		})

		http.Redirect(w, req, "/", http.StatusSeeOther)
	}
}
