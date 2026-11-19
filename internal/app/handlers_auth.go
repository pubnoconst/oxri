package app

import (
	"fmt"
	"net"
	"net/http"

	"github.com/oxri/oxri-go/internal/auth"
	"github.com/oxri/oxri-go/internal/pages/login"
)

func handleLogin(authSvc *auth.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		next := req.URL.Query().Get("next")
		if next == "" {
			next = "/"
		}

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
			if redirectURL == "" {
				redirectURL = "/"
			}
			http.SetCookie(w, &http.Cookie{
				Name:     auth.SessionCookieName,
				Value:    sessionID,
				Path:     "/",
				HttpOnly: true,
				SameSite: http.SameSiteLaxMode,
				MaxAge:   int(auth.SessionDuration.Seconds()),
			})
			w.Header().Set("HX-Redirect", redirectURL)
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
			SameSite: http.SameSiteLaxMode,
			MaxAge:   -1,
		})

		http.Redirect(w, req, "/", http.StatusSeeOther)
	}
}
