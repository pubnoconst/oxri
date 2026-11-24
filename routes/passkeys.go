package routes

import (
	"encoding/json"
	"errors"
	"github.com/oxri/oxri-go/internal/domain"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode"

	"github.com/go-chi/chi/v5"
	"github.com/oxri/oxri-go/internal/auth"
	"github.com/oxri/oxri-go/internal/db"
	"github.com/oxri/oxri-go/serverfns"
	"github.com/oxri/oxri-go/views/confirm"
)

func sanitizePasskeyLabel(raw string) string {
	var b strings.Builder
	for _, r := range strings.TrimSpace(raw) {
		if b.Len() >= 32 {
			break
		}
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == ' ' || r == '-' || r == '_' || r == '.' {
			b.WriteRune(r)
		}
	}
	out := strings.TrimSpace(b.String())
	if out == "" {
		return "Passkey"
	}
	return out
}

func loadWebAuthnCreds(r *http.Request, store *db.Store, rpID string) ([]db.PasskeyRecord, []string, error) {
	if store == nil {
		return nil, nil, errors.New("database store unavailable")
	}
	recs, err := store.ListPasskeys(r.Context(), rpID)
	if err != nil {
		return nil, nil, err
	}
	rawJSON := make([]string, 0, len(recs))
	for _, rec := range recs {
		rawJSON = append(rawJSON, rec.CredentialJSON)
	}
	return recs, rawJSON, nil
}

func handlePasskeyRegisterBegin(store *db.Store, authSvc *auth.Service) http.HandlerFunc {
	return freshAdminOnly(authSvc, denyAPI, func(w http.ResponseWriter, r *http.Request, cap domain.AdminCap) {
		if store == nil || authSvc == nil {
			http.Error(w, "Passkey storage is unavailable.", http.StatusServiceUnavailable)
			return
		}

		secure := isHTTPSRequest(r)
		rpID, origins, supported := auth.ResolveWebAuthnOrigin(r, secure)
		if !supported {
			http.Error(w, "Passkeys require HTTPS or localhost.", http.StatusBadRequest)
			return
		}

		_, rawJSON, err := loadWebAuthnCreds(r, store, rpID)
		if err != nil {
			http.Error(w, "Failed to load existing passkeys.", http.StatusInternalServerError)
			return
		}

		ceremonyID, opts, err := authSvc.BeginPasskeyRegistration(rpID, origins, auth.UnmarshalCredentials(rawJSON))
		if err != nil {
			http.Error(w, "Failed to initiate passkey registration.", http.StatusInternalServerError)
			return
		}

		http.SetCookie(w, &http.Cookie{
			Name:     auth.PasskeyCeremonyCookieName,
			Value:    ceremonyID,
			Path:     "/",
			HttpOnly: true,
			Secure:   secure,
			SameSite: http.SameSiteLaxMode,
			MaxAge:   300,
		})

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(opts)
	})
}

func handlePasskeyRegisterFinish(store *db.Store, authSvc *auth.Service) http.HandlerFunc {
	return freshAdminOnly(authSvc, denyAPI, func(w http.ResponseWriter, r *http.Request, cap domain.AdminCap) {
		if store == nil || authSvc == nil {
			http.Error(w, "Passkey storage is unavailable.", http.StatusServiceUnavailable)
			return
		}

		secure := isHTTPSRequest(r)
		rpID, origins, supported := auth.ResolveWebAuthnOrigin(r, secure)
		if !supported {
			http.Error(w, "Passkeys require HTTPS or localhost.", http.StatusBadRequest)
			return
		}

		ceremonyCookie, err := r.Cookie(auth.PasskeyCeremonyCookieName)
		if err != nil || ceremonyCookie.Value == "" {
			http.Error(w, "Passkey ceremony expired. Please try again.", http.StatusBadRequest)
			return
		}

		_, rawJSON, err := loadWebAuthnCreds(r, store, rpID)
		if err != nil {
			http.Error(w, "Failed to load existing passkeys.", http.StatusInternalServerError)
			return
		}

		cred, err := authSvc.FinishPasskeyRegistration(ceremonyCookie.Value, rpID, origins, auth.UnmarshalCredentials(rawJSON), r)
		// Always clear ceremony cookie
		http.SetCookie(w, &http.Cookie{
			Name:     auth.PasskeyCeremonyCookieName,
			Value:    "",
			Path:     "/",
			HttpOnly: true,
			Secure:   secure,
			SameSite: http.SameSiteLaxMode,
			MaxAge:   -1,
		})
		if err != nil {
			log.Printf("[PASSKEY] registration finish failed (rpID=%s): %v", rpID, err)
			http.Error(w, "Passkey verification failed.", http.StatusBadRequest)
			return
		}

		credJSON, err := auth.MarshalCredential(cred)
		if err != nil {
			http.Error(w, "Failed to serialize passkey.", http.StatusInternalServerError)
			return
		}

		label := sanitizePasskeyLabel(r.URL.Query().Get("name"))
		rec := db.PasskeyRecord{
			ID:             auth.CredentialIDString(cred.ID),
			RPID:           rpID,
			Name:           label,
			CredentialJSON: credJSON,
			CreatedAt:      time.Now().UTC(),
		}
		if err := store.SavePasskey(r.Context(), cap, rec); err != nil {
			http.Error(w, "Failed to save passkey.", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok", "id": rec.ID})
	})
}

func handlePasskeyDelete(store *db.Store, authSvc *auth.Service) http.HandlerFunc {
	return freshAdminOnly(authSvc, denyToSettingsSignIn, func(w http.ResponseWriter, r *http.Request, cap domain.AdminCap) {
		if store == nil {
			http.Redirect(w, r, "/settings", http.StatusSeeOther)
			return
		}
		id := strings.TrimSpace(chi.URLParam(r, "id"))
		if !confirm.Confirmed(r) {
			http.Redirect(w, r, "/settings/passkeys/"+url.PathEscape(id)+"/delete", http.StatusSeeOther)
			return
		}
		rpID, _, supported := auth.ResolveWebAuthnOrigin(r, isHTTPSRequest(r))
		if supported {
			if id != "" {
				_ = store.DeletePasskey(r.Context(), cap, id, rpID)
			}
		}
		http.Redirect(w, r, "/settings", http.StatusSeeOther)
	})
}

func handlePasskeyLoginBegin(store *db.Store, authSvc *auth.Service, limiter *ipRateLimiter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if limiter != nil && !limiter.allow(clientIPFromRequest(r)) {
			http.Error(w, "Too many authentication requests. Please wait briefly.", http.StatusTooManyRequests)
			return
		}
		if store == nil || authSvc == nil {
			http.Error(w, "Passkey authentication is unavailable.", http.StatusServiceUnavailable)
			return
		}

		secure := isHTTPSRequest(r)
		rpID, origins, supported := auth.ResolveWebAuthnOrigin(r, secure)
		if !supported {
			http.Error(w, "Passkeys require HTTPS or localhost.", http.StatusBadRequest)
			return
		}

		_, rawJSON, err := loadWebAuthnCreds(r, store, rpID)
		if err != nil {
			http.Error(w, "Failed to load passkeys.", http.StatusInternalServerError)
			return
		}

		ceremonyID, opts, err := authSvc.BeginPasskeyLogin(rpID, origins, auth.UnmarshalCredentials(rawJSON))
		if err != nil {
			if errors.Is(err, auth.ErrPasskeyNoCreds) {
				http.Error(w, "No passkeys enrolled for this domain.", http.StatusNotFound)
				return
			}
			http.Error(w, "Failed to initiate passkey login.", http.StatusInternalServerError)
			return
		}

		http.SetCookie(w, &http.Cookie{
			Name:     auth.PasskeyCeremonyCookieName,
			Value:    ceremonyID,
			Path:     "/",
			HttpOnly: true,
			Secure:   secure,
			SameSite: http.SameSiteLaxMode,
			MaxAge:   300,
		})

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(opts)
	}
}

func handlePasskeyLoginFinish(store *db.Store, authSvc *auth.Service, limiter *ipRateLimiter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if limiter != nil && !limiter.allow(clientIPFromRequest(r)) {
			http.Error(w, "Too many authentication requests. Please wait briefly.", http.StatusTooManyRequests)
			return
		}
		if store == nil || authSvc == nil {
			http.Error(w, "Passkey authentication is unavailable.", http.StatusServiceUnavailable)
			return
		}

		secure := isHTTPSRequest(r)
		rpID, origins, supported := auth.ResolveWebAuthnOrigin(r, secure)
		if !supported {
			http.Error(w, "Passkeys require HTTPS or localhost.", http.StatusBadRequest)
			return
		}

		ceremonyCookie, err := r.Cookie(auth.PasskeyCeremonyCookieName)
		if err != nil || ceremonyCookie.Value == "" {
			http.Error(w, "Passkey login session expired. Please try again.", http.StatusBadRequest)
			return
		}

		_, rawJSON, err := loadWebAuthnCreds(r, store, rpID)
		if err != nil {
			http.Error(w, "Failed to load passkeys.", http.StatusInternalServerError)
			return
		}

		cred, sessionID, err := authSvc.FinishPasskeyLogin(ceremonyCookie.Value, rpID, origins, auth.UnmarshalCredentials(rawJSON), r)
		http.SetCookie(w, &http.Cookie{
			Name:     auth.PasskeyCeremonyCookieName,
			Value:    "",
			Path:     "/",
			HttpOnly: true,
			Secure:   secure,
			SameSite: http.SameSiteLaxMode,
			MaxAge:   -1,
		})
		if err != nil {
			log.Printf("[PASSKEY] login finish failed (rpID=%s): %v", rpID, err)
			http.Error(w, "Passkey sign-in failed.", http.StatusUnauthorized)
			return
		}

		if updatedJSON, mErr := auth.MarshalCredential(cred); mErr == nil {
			_ = store.UpdatePasskeyCredential(r.Context(), auth.CredentialIDString(cred.ID), rpID, updatedJSON, time.Now().UTC())
		}

		if oldSess, err := r.Cookie(auth.SessionCookieName); err == nil && oldSess.Value != "" {
			authSvc.RevokeSession(oldSess.Value)
		}

		http.SetCookie(w, &http.Cookie{
			Name:     auth.SessionCookieName,
			Value:    sessionID,
			Path:     "/",
			HttpOnly: true,
			Secure:   secure,
			SameSite: http.SameSiteLaxMode,
			MaxAge:   int(auth.SessionDuration.Seconds()),
		})

		target := serverfns.SanitizeRedirect(r.URL.Query().Get("next"))
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(map[string]string{"redirect": target})
	}
}
