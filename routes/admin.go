package routes

import (
	"errors"
	"net/http"

	"github.com/oxri/oxri-go/internal/auth"
	"github.com/oxri/oxri-go/internal/domain"
	"github.com/oxri/oxri-go/serverfns"
	"github.com/oxri/oxri-go/views/errorpage"
)

// adminHandler is a handler that runs only with verified administrative
// authority. It receives the domain.AdminCap that every administrative store
// write demands, so an admin page cannot be written without the check: the
// only way to obtain the cap is to be wrapped by adminOnly / freshAdminOnly.
type adminHandler func(w http.ResponseWriter, r *http.Request, cap domain.AdminCap)

// adminOnly verifies the session and passes its capability to h.
// Unauthenticated GETs are sent to sign in and come back; anything else is 403.
func adminOnly(authSvc *auth.Service, h adminHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cap, err := serverfns.RequireAuth(authSvc, r)
		if err != nil {
			if r.Method == http.MethodGet {
				next := serverfns.SanitizeRedirect(r.URL.Path)
				http.Redirect(w, r, "/login?next="+next, http.StatusSeeOther)
				return
			}
			errorpage.Render(w, http.StatusForbidden, "")
			return
		}
		h(w, r, cap)
	}
}

// freshDenied decides how a stale or missing fresh sign-in is answered.
type freshDenied func(w http.ResponseWriter, r *http.Request, stale bool)

// freshAdminOnly additionally requires the session to be minted within
// auth.FreshAuthWindow (step-up for credential management).
func freshAdminOnly(authSvc *auth.Service, deny freshDenied, h adminHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cap, err := serverfns.RequireFreshAuth(authSvc, r, auth.FreshAuthWindow)
		if err != nil {
			deny(w, r, errors.Is(err, serverfns.ErrStaleSession))
			return
		}
		h(w, r, cap)
	}
}

// denyAPI answers JSON/fetch endpoints (passkey enrolment ceremony).
func denyAPI(w http.ResponseWriter, _ *http.Request, stale bool) {
	if stale {
		http.Error(w, "Recent sign-in required. Please re-verify your identity to manage passkeys.", http.StatusForbidden)
		return
	}
	http.Error(w, "Unauthorized", http.StatusUnauthorized)
}

// denyToSettingsSignIn answers form posts from /settings with a (re)sign-in.
func denyToSettingsSignIn(w http.ResponseWriter, r *http.Request, stale bool) {
	if stale {
		http.Redirect(w, r, "/login?next=/settings&reauth=1", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/login?next=/settings", http.StatusSeeOther)
}
