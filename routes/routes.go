// Package routes wires the application HTTP router, middleware, and handlers.
package routes

import (
	"errors"
	"log"
	"net/http"
	"net/url"
	"runtime/debug"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/oxri/oxri-go/internal/auth"
	"github.com/oxri/oxri-go/internal/db"
	"github.com/oxri/oxri-go/serverfns"
	"github.com/oxri/oxri-go/views/booki"
	"github.com/oxri/oxri-go/views/editor"
	"github.com/oxri/oxri-go/views/errorpage"
	"github.com/oxri/oxri-go/views/journals"
	"github.com/oxri/oxri-go/views/landing"
	"github.com/oxri/oxri-go/views/loading"
	"github.com/oxri/oxri-go/views/login"
	"github.com/oxri/oxri-go/views/pilot"
)

// Config configures the application router dependencies and static asset paths.
type Config struct {
	AssetsDir   string
	AuthService *auth.Service
	Store       *db.Store
}

// NewRouter builds and returns the top-level Chi HTTP handler.
func NewRouter(cfg Config) http.Handler {
	r := chi.NewRouter()

	// 1. Resilience & Security Hardening Middleware
	r.Use(safeRecoverer)
	r.Use(securityHeadersMiddleware)
	r.Use(bodySizeCapMiddleware)
	r.Use(csrfOriginMiddleware)
	r.Use(middleware.Timeout(30 * time.Second))

	// 2. Custom 404 & 405 Handlers
	r.NotFound(func(w http.ResponseWriter, r *http.Request) {
		errorpage.Render(w, http.StatusNotFound, "")
	})
	r.MethodNotAllowed(func(w http.ResponseWriter, r *http.Request) {
		errorpage.Render(w, http.StatusMethodNotAllowed, "")
	})

	// 3. Component CSS Routes (each component owns its own CSS route)
	r.Get(landing.StylePath, serveCSS(landing.StyleCSS))
	r.Get(errorpage.StylePath, serveCSS(errorpage.StyleCSS))
	r.Get(booki.StylePath, serveCSS(booki.StyleCSS))
	r.Get(login.StylePath, serveCSS(login.StyleCSS))
	r.Get(pilot.StylePath, serveCSS(pilot.StyleCSS))
	r.Get(journals.StylePath, serveCSS(journals.StyleCSS))
	r.Get(editor.StylePath, serveCSS(editor.StyleCSS))
	r.Get(loading.StylePath, serveCSS(loading.StyleCSS))

	// 4. Static Assets (/assets/*) — Directory listings strictly disabled
	assetsDir := cfg.AssetsDir
	if assetsDir == "" {
		assetsDir = "assets"
	}
	r.Handle("/assets/*", noDirListingAssetHandler(assetsDir))

	// 5. Public Page & Health Routes
	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	r.Get("/", handleLanding(cfg.AuthService))
	r.Get("/book-i", handleBookI())
	r.Get("/loading", handleLoadingPage())

	// 6. Journals & Editor Routes
	r.Get("/journals", handleJournalsCatalog(cfg.AuthService))
	r.Get("/journals/content", handleJournalsCatalogContent(cfg.Store, cfg.AuthService))
	r.Get("/journals/new", handleJournalNew(cfg.Store, cfg.AuthService))
	r.Post("/journals", handleJournalCreate(cfg.Store, cfg.AuthService))
	r.Post("/journals/batch", handleJournalBatch(cfg.Store, cfg.AuthService))
	r.Get("/journals/vacate-space", handleJournalVacateSpacePage(cfg.Store, cfg.AuthService))
	r.Get("/journals/vacate-space/", handleJournalVacateSpacePage(cfg.Store, cfg.AuthService))
	r.Post("/journals/vacate-space", handleJournalVacateSpaceSubmit(cfg.Store, cfg.AuthService))
	r.Post("/journals/vacate-space/", handleJournalVacateSpaceSubmit(cfg.Store, cfg.AuthService))
	r.Get("/journals/{id}", handleJournalEntry(cfg.Store, cfg.AuthService))
	r.Get("/journals/{id}/edit", handleJournalEditForm(cfg.Store, cfg.AuthService))
	r.Post("/journals/{id}", handleJournalUpdate(cfg.Store, cfg.AuthService))
	r.Post("/journals/{id}/delete", handleJournalDelete(cfg.Store, cfg.AuthService))
	r.Post("/journals/{id}/publish", handleJournalPublish(cfg.Store, cfg.AuthService, true))
	r.Post("/journals/{id}/unpublish", handleJournalPublish(cfg.Store, cfg.AuthService, false))
	r.Post("/journals/{id}/bookmark", handleJournalBookmark(cfg.Store, cfg.AuthService))

	// 7. Authentication Routes (with per-IP rate limiters)
	loginLimiter := newIPRateLimiter(30, time.Minute)
	totpSubmitLimiter := newIPRateLimiter(15, 5*time.Minute)
	statusLimiter := newIPRateLimiter(120, time.Minute)
	r.Get("/login", handleLogin(cfg.Store, cfg.AuthService, loginLimiter))
	r.Post("/login", handleLoginSubmit(cfg.Store, cfg.AuthService, totpSubmitLimiter))
	r.Get("/login/confirm", handleTOTPConfirm(cfg.AuthService))
	r.Get("/auth/totp-status", handleTOTPStatus(cfg.AuthService, statusLimiter))
	r.Post("/auth/passkey/begin", handlePasskeyLoginBegin(cfg.Store, cfg.AuthService, loginLimiter))
	r.Post("/auth/passkey/finish", handlePasskeyLoginFinish(cfg.Store, cfg.AuthService, loginLimiter))
	r.HandleFunc("/logout", handleLogout(cfg.AuthService)) // Handles both GET and POST

	// 8. Protected Profile / Pilot Settings & Passkey Management Routes
	r.Get("/pilot/user/settings", handlePilotSettings(cfg.Store, cfg.AuthService))
	r.Post("/pilot/passkeys/begin", handlePasskeyRegisterBegin(cfg.Store, cfg.AuthService))
	r.Post("/pilot/passkeys/finish", handlePasskeyRegisterFinish(cfg.Store, cfg.AuthService))
	r.Post("/pilot/passkeys/{id}/delete", handlePasskeyDelete(cfg.Store, cfg.AuthService))

	return r
}

func safeRecoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rvr := recover(); rvr != nil {
				if rvr == http.ErrAbortHandler {
					panic(rvr)
				}
				ref := errorpage.NewReference()
				log.Printf("[PANIC RECOVERED] Ref: %s | URL: %s | Panic: %v\nStack:\n%s",
					ref, r.URL.Path, rvr, string(debug.Stack()))

				errorpage.RenderWithReference(w, http.StatusInternalServerError, "", ref)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func handleLanding(authSvc *auth.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, err := serverfns.RequireAuth(authSvc, r)
		isAuth := (err == nil)
		_ = landing.Page(isAuth).Render(w)
	}
}

func handleBookI() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_ = booki.Page().Render(w)
	}
}

func renderLoginView(w http.ResponseWriter, r *http.Request, store *db.Store, authSvc *auth.Service, next, errMsg string, status int) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if status != http.StatusOK {
		w.WriteHeader(status)
	}
	requirePassword := authSvc != nil && authSvc.HasPassword()
	hasPasskeys := false
	if store != nil {
		if rpID, _, supported := auth.ResolveWebAuthnOrigin(r, isHTTPSRequest(r)); supported {
			if count, err := store.CountPasskeys(r.Context(), rpID); err == nil && count > 0 {
				hasPasskeys = true
			}
		}
	}
	_ = login.TOTPPage(next, errMsg, requirePassword, hasPasskeys).Render(w)
}

func isValidVerifyID(id string) bool {
	if len(id) != 32 {
		return false
	}
	for i := 0; i < len(id); i++ {
		c := id[i]
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			return false
		}
	}
	return true
}

func loginErrorMessage(code string) string {
	switch code {
	case "invalid":
		return "Invalid or expired authentication code."
	case "replayed":
		return "That code was already used. Wait for the next 6-digit code."
	case "locked":
		return "Too many failed attempts. Please wait a few minutes."
	case "expired":
		return "Confirmation expired. Please enter a fresh 6-digit code."
	default:
		return ""
	}
}

func handleLogin(store *db.Store, authSvc *auth.Service, limiter *ipRateLimiter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		next := serverfns.SanitizeRedirect(r.URL.Query().Get("next"))
		if _, err := serverfns.RequireAuth(authSvc, r); err == nil {
			http.Redirect(w, r, next, http.StatusSeeOther)
			return
		}

		if limiter != nil && !limiter.allow(clientIPFromRequest(r)) {
			errorpage.Render(w, http.StatusTooManyRequests, "Too many authentication requests. Please wait briefly.")
			return
		}

		if authSvc == nil {
			errorpage.Render(w, http.StatusServiceUnavailable, "Authentication service is currently unavailable.")
			return
		}

		errMsg := loginErrorMessage(r.URL.Query().Get("err"))
		renderLoginView(w, r, store, authSvc, next, errMsg, http.StatusOK)
	}
}

func handleLoginSubmit(store *db.Store, authSvc *auth.Service, limiter *ipRateLimiter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if authSvc == nil || !authSvc.HasTOTP() {
			errorpage.Render(w, http.StatusServiceUnavailable, "TOTP authentication is not configured.")
			return
		}

		_ = r.ParseForm()
		next := serverfns.SanitizeRedirect(r.FormValue("next"))
		if next == "/" && r.URL.Query().Get("next") != "" {
			next = serverfns.SanitizeRedirect(r.URL.Query().Get("next"))
		}

		if limiter != nil && !limiter.allow(clientIPFromRequest(r)) {
			renderLoginView(w, r, store, authSvc, next, "Too many attempts. Please wait a few minutes.", http.StatusTooManyRequests)
			return
		}

		password := r.FormValue("password")
		code := strings.TrimSpace(r.FormValue("code"))
		verifyID, bindingCookie, _, err := authSvc.StartTOTPVerification(password, code, next)
		if err != nil {
			if errors.Is(err, auth.ErrTOTPPending) {
				renderLoginView(w, r, store, authSvc, next, "A confirmation is already in progress. Please wait.", http.StatusTooManyRequests)
				return
			}
			errorpage.Render(w, http.StatusInternalServerError, "Failed to initiate authentication confirmation.")
			return
		}

		secure := isHTTPSRequest(r)
		http.SetCookie(w, &http.Cookie{
			Name:     auth.BindingCookieName,
			Value:    bindingCookie,
			Path:     "/",
			HttpOnly: true,
			Secure:   secure,
			SameSite: http.SameSiteLaxMode,
			MaxAge:   180,
		})

		http.Redirect(w, r, "/login/confirm?id="+verifyID, http.StatusSeeOther)
	}
}

func handleTOTPConfirm(authSvc *auth.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if authSvc == nil || !authSvc.HasTOTP() {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		verifyID := strings.TrimSpace(r.URL.Query().Get("id"))
		if !isValidVerifyID(verifyID) {
			http.Redirect(w, r, "/login?err=expired", http.StatusSeeOther)
			return
		}
		bindingCookie, err := r.Cookie(auth.BindingCookieName)
		if err != nil || bindingCookie.Value == "" {
			http.Redirect(w, r, "/login?err=expired", http.StatusSeeOther)
			return
		}
		remaining, err := authSvc.PendingTOTPStatus(verifyID, bindingCookie.Value)
		if err != nil {
			http.Redirect(w, r, "/login?err=expired", http.StatusSeeOther)
			return
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_ = login.TOTPConfirmPage(verifyID, remaining.Milliseconds(), authSvc.TOTPConfirmDelay().Milliseconds()).Render(w)
	}
}

func handleTOTPStatus(authSvc *auth.Service, limiter *ipRateLimiter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if authSvc == nil || !authSvc.HasTOTP() {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		if limiter != nil && !limiter.allow(clientIPFromRequest(r)) {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}

		verifyID := strings.TrimSpace(r.URL.Query().Get("id"))
		if !isValidVerifyID(verifyID) {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		bindingCookie, err := r.Cookie(auth.BindingCookieName)
		if err != nil || bindingCookie.Value == "" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		sessionID, redirectURL, err := authSvc.RedeemTOTPVerification(verifyID, bindingCookie.Value)
		if errors.Is(err, auth.ErrTOTPConfirming) {
			// Mandatory 30-second confirmation cooldown still active: keep polling
			w.WriteHeader(http.StatusNoContent)
			return
		}

		secure := isHTTPSRequest(r)
		// Clear temporary binding cookie once confirmation resolves
		http.SetCookie(w, &http.Cookie{
			Name:     auth.BindingCookieName,
			Value:    "",
			Path:     "/",
			HttpOnly: true,
			Secure:   secure,
			SameSite: http.SameSiteLaxMode,
			MaxAge:   -1,
		})

		if err != nil {
			errCode := "invalid"
			switch {
			case errors.Is(err, auth.ErrTOTPLocked):
				errCode = "locked"
			case errors.Is(err, auth.ErrTOTPReplayed):
				errCode = "replayed"
			case errors.Is(err, auth.ErrChallengeExpired), errors.Is(err, auth.ErrChallengeNotFound):
				errCode = "expired"
			}
			vals := url.Values{}
			vals.Set("err", errCode)
			if safeNext := serverfns.SanitizeRedirect(redirectURL); safeNext != "/" {
				vals.Set("next", safeNext)
			}
			failTarget := "/login?" + vals.Encode()
			if r.Header.Get("HX-Request") == "true" {
				w.Header().Set("HX-Redirect", failTarget)
				w.WriteHeader(http.StatusOK)
				return
			}
			http.Redirect(w, r, failTarget, http.StatusSeeOther)
			return
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

		target := serverfns.SanitizeRedirect(redirectURL)
		if r.Header.Get("HX-Request") == "true" {
			w.Header().Set("HX-Redirect", target)
			w.WriteHeader(http.StatusOK)
			return
		}
		http.Redirect(w, r, target, http.StatusSeeOther)
	}
}

func handlePilotSettings(store *db.Store, authSvc *auth.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, err := serverfns.RequireAuth(authSvc, r); err != nil {
			http.Redirect(w, r, "/login?next=/pilot/user/settings", http.StatusSeeOther)
			return
		}

		methodLabel := "TOTP (RFC 6238)"
		if authSvc != nil && authSvc.HasPassword() {
			methodLabel = "Password (Argon2id) + TOTP (RFC 6238)"
		}

		rpID, _, supported := auth.ResolveWebAuthnOrigin(r, isHTTPSRequest(r))
		var items []pilot.PasskeyItem
		if store != nil && supported {
			if recs, err := store.ListPasskeys(r.Context(), rpID); err == nil {
				for _, rec := range recs {
					lastUsed := ""
					if !rec.LastUsedAt.IsZero() {
						lastUsed = rec.LastUsedAt.Local().Format("Jan 2, 2006")
					}
					items = append(items, pilot.PasskeyItem{
						ID:         rec.ID,
						Name:       rec.Name,
						CreatedAt:  rec.CreatedAt.Local().Format("Jan 2, 2006"),
						LastUsedAt: lastUsed,
					})
				}
			}
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_ = pilot.SettingsPage(pilot.SettingsData{
			MethodLabel:      methodLabel,
			RPID:             rpID,
			PasskeySupported: supported && store != nil,
			Passkeys:         items,
		}).Render(w)
	}
}

func handleLogout(authSvc *auth.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if cookie, err := r.Cookie(auth.SessionCookieName); err == nil && cookie.Value != "" {
			if authSvc != nil {
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

		http.Redirect(w, r, "/", http.StatusSeeOther)
	}
}

func serveCSS(css string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
		_, _ = w.Write([]byte(css))
	}
}
