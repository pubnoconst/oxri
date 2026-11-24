// Package routes wires the application HTTP router, middleware, and handlers.
package routes

import (
	"errors"
	"github.com/oxri/oxri-go/assets"
	"log"
	"math"
	"net/http"
	"runtime/debug"
	"strconv"
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
	"github.com/oxri/oxri-go/views/linen"
	"github.com/oxri/oxri-go/views/loading"
	"github.com/oxri/oxri-go/views/login"
	"github.com/oxri/oxri-go/views/offline"
	"github.com/oxri/oxri-go/views/settings"
)

// Config configures the application router dependencies.
// Static assets are embedded in the binary (package assets).
type Config struct {
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

	// 3. Component CSS Routes (each component owns its stylesheet; content-hashed URLs)
	for _, style := range []assets.Asset{
		landing.Style, errorpage.Style, booki.Style, login.Style,
		settings.Style, journals.Style, editor.Style, loading.Style,
		linen.Style,
	} {
		r.Get(style.Path(), style.Handler())
	}

	// 4. Static Assets (/assets/*), embedded; no directory listings
	r.Handle(assets.RoutePattern, assets.FileHandler(func(w http.ResponseWriter, _ *http.Request) {
		errorpage.Render(w, http.StatusNotFound, "")
	}))

	// Web app manifest (installable on phones and desktops).
	r.Get("/manifest.webmanifest", handleManifest)
	// Service worker: an offline page, nothing else (routes/sw.js).
	r.Get(ServiceWorkerPath, handleServiceWorker)
	r.Get(offline.Path, handleOffline)

	// 5. Public Page, Books & Health Routes
	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	r.Get("/", handleLanding(cfg.AuthService))
	r.Get("/loading", handleLoadingPage())

	bookWriteLimiter := newIPRateLimiter(20, 5*time.Minute)
	for _, slug := range []string{"book-i", "book-ii", "book-iii"} {
		r.Get("/"+slug, handleBookView(cfg.Store, cfg.AuthService, slug))
		r.Get("/"+slug+"/edit", handleBookEditForm(cfg.Store, cfg.AuthService, slug))
		r.Post("/"+slug, handleBookSave(cfg.Store, cfg.AuthService, slug, bookWriteLimiter))
	}

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
	r.Get("/login", handleLogin(cfg.Store, cfg.AuthService, loginLimiter))
	r.Post("/login", handleLoginSubmit(cfg.Store, cfg.AuthService, totpSubmitLimiter))
	r.Post("/auth/passkey/begin", handlePasskeyLoginBegin(cfg.Store, cfg.AuthService, loginLimiter))
	r.Post("/auth/passkey/finish", handlePasskeyLoginFinish(cfg.Store, cfg.AuthService, loginLimiter))
	r.Post("/logout", handleLogout(cfg.AuthService))

	// 8. Protected Settings & Passkey Management Routes
	r.Get("/settings", handleSettings(cfg.Store, cfg.AuthService))
	r.Post("/settings/passkeys/begin", handlePasskeyRegisterBegin(cfg.Store, cfg.AuthService))
	r.Post("/settings/passkeys/finish", handlePasskeyRegisterFinish(cfg.Store, cfg.AuthService))
	r.Post("/settings/passkeys/{id}/delete", handlePasskeyDelete(cfg.Store, cfg.AuthService))

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

func renderLoginView(w http.ResponseWriter, r *http.Request, store *db.Store, authSvc *auth.Service, next, errMsg string, status int, reauth bool) {
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
	_ = login.TOTPPage(next, errMsg, requirePassword, hasPasskeys, reauth).Render(w)
}

// loginFailure maps a sign-in failure to a plain, non-technical message and
// HTTP status. Wrong password and wrong code share one message on purpose.
func loginFailure(err error, hasPassword bool) (string, int) {
	wait := humanWait(auth.RetryAfter(err))
	what := "code"
	if hasPassword {
		what = "password or code"
	}
	switch {
	case errors.Is(err, auth.ErrTOTPPaused):
		return "Password sign-in is paused for a few minutes after unusual activity. Use your passkey, or try again in " + wait + ".", http.StatusTooManyRequests
	case errors.Is(err, auth.ErrTOTPLocked):
		return "Too many attempts. Please try again in " + wait + ".", http.StatusTooManyRequests
	case errors.Is(err, auth.ErrTOTPCooling):
		return "Please wait " + wait + " before trying again.", http.StatusTooManyRequests
	case errors.Is(err, auth.ErrTOTPReplayed):
		return "That code was already used. Wait for the next 6-digit code.", http.StatusUnauthorized
	case errors.Is(err, auth.ErrTOTPInvalid):
		if auth.RetryAfter(err) >= time.Minute {
			return "Incorrect " + what + ". Too many attempts, so please try again in " + wait + ".", http.StatusUnauthorized
		}
		return "Incorrect " + what + ". You can try again in " + wait + ".", http.StatusUnauthorized
	default:
		return "", http.StatusInternalServerError
	}
}

// humanWait renders a wait as whole seconds under a minute, else whole minutes.
func humanWait(d time.Duration) string {
	if d <= time.Second {
		return "a moment"
	}
	if d < time.Minute {
		n := int(math.Ceil(d.Seconds()))
		return strconv.Itoa(n) + " seconds"
	}
	n := int(math.Ceil(d.Minutes()))
	if n == 1 {
		return "1 minute"
	}
	return strconv.Itoa(n) + " minutes"
}

func handleLogin(store *db.Store, authSvc *auth.Service, limiter *ipRateLimiter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		next := serverfns.SanitizeRedirect(r.URL.Query().Get("next"))
		reauth := r.URL.Query().Get("reauth") == "1"
		if !reauth {
			if _, err := serverfns.RequireAuth(authSvc, r); err == nil {
				http.Redirect(w, r, next, http.StatusSeeOther)
				return
			}
		}

		if limiter != nil && !limiter.allow(clientIPFromRequest(r)) {
			errorpage.Render(w, http.StatusTooManyRequests, "Too many authentication requests. Please wait briefly.")
			return
		}

		if authSvc == nil {
			errorpage.Render(w, http.StatusServiceUnavailable, "Authentication service is currently unavailable.")
			return
		}

		renderLoginView(w, r, store, authSvc, next, "", http.StatusOK, reauth)
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

		clientIP := clientIPFromRequest(r)
		if limiter != nil && !limiter.allow(clientIP) {
			renderLoginView(w, r, store, authSvc, next, "Too many attempts. Please wait a few minutes.", http.StatusTooManyRequests, false)
			return
		}

		// Instant on success; failures carry an escalating per-IP hold, the
		// per-IP lockout and the global budget (internal/auth/login.go).
		password := r.FormValue("password")
		code := strings.TrimSpace(r.FormValue("code"))
		sessionID, err := authSvc.LoginTOTP(clientIP, password, code)
		if err != nil {
			msg, status := loginFailure(err, authSvc.HasPassword())
			if status == http.StatusInternalServerError {
				errorpage.Render(w, status, "Sign-in is temporarily unavailable.")
				return
			}
			if wait := auth.RetryAfter(err); wait > 0 {
				w.Header().Set("Retry-After", strconv.Itoa(int(math.Ceil(wait.Seconds()))))
			}
			renderLoginView(w, r, store, authSvc, next, msg, status, false)
			return
		}

		secure := isHTTPSRequest(r)
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

		target := next
		if r.Header.Get("HX-Request") == "true" {
			w.Header().Set("HX-Redirect", target)
			w.WriteHeader(http.StatusOK)
			return
		}
		http.Redirect(w, r, target, http.StatusSeeOther)
	}
}

func handleSettings(store *db.Store, authSvc *auth.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, err := serverfns.RequireAuth(authSvc, r); err != nil {
			http.Redirect(w, r, "/login?next=/settings", http.StatusSeeOther)
			return
		}

		_, freshErr := serverfns.RequireFreshAuth(authSvc, r, auth.FreshAuthWindow)

		methodLabel := "TOTP (RFC 6238)"
		if authSvc != nil && authSvc.HasPassword() {
			methodLabel = "Password (Argon2id) + TOTP (RFC 6238)"
		}

		rpID, _, supported := auth.ResolveWebAuthnOrigin(r, isHTTPSRequest(r))
		var items []settings.PasskeyItem
		if store != nil && supported {
			if recs, err := store.ListPasskeys(r.Context(), rpID); err == nil {
				for _, rec := range recs {
					lastUsed := ""
					if !rec.LastUsedAt.IsZero() {
						lastUsed = rec.LastUsedAt.Local().Format("Jan 2, 2006")
					}
					items = append(items, settings.PasskeyItem{
						ID:         rec.ID,
						Name:       rec.Name,
						CreatedAt:  rec.CreatedAt.Local().Format("Jan 2, 2006"),
						LastUsedAt: lastUsed,
					})
				}
			}
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_ = settings.SettingsPage(settings.SettingsData{
			MethodLabel:      methodLabel,
			RPID:             rpID,
			PasskeySupported: supported && store != nil,
			FreshAuth:        freshErr == nil,
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
			Secure:   isHTTPSRequest(r),
			SameSite: http.SameSiteLaxMode,
			MaxAge:   -1,
		})

		http.Redirect(w, r, "/", http.StatusSeeOther)
	}
}
