package app

import (
	"net/http"

	"github.com/oxri/oxri-go/internal/auth"
	"github.com/oxri/oxri-go/internal/pages/pilot"
)

func handlePilotRedirect() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		http.Redirect(w, req, "/pilot/user/settings", http.StatusTemporaryRedirect)
	}
}

func handlePilotSettings(authSvc *auth.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		keyType := "ssh-ed25519"
		if authSvc != nil {
			keyType = authSvc.AuthorizedKeyType()
		}
		_ = pilot.SettingsPage(keyType).Render(req.Context(), w)
	}
}
