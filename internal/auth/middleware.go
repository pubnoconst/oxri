package auth

import (
	"context"
	"net/http"
	"net/url"
)

type authContextKey struct{}

var isAdminKey = authContextKey{}

// SessionMiddleware inspects the session cookie and populates admin status in context.
func SessionMiddleware(authSvc *Service) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			isAdmin := false
			cookie, err := r.Cookie(SessionCookieName)
			if err == nil && cookie.Value != "" {
				if authSvc.ValidateSession(cookie.Value) {
					isAdmin = true
				}
			}

			ctx := context.WithValue(r.Context(), isAdminKey, isAdmin)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireAuth enforces that only authenticated requests proceed.
func RequireAuth(authSvc *Service) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !IsAuthenticated(r.Context()) {
				nextTarget := url.QueryEscape(r.URL.RequestURI())
				http.Redirect(w, r, "/login?next="+nextTarget, http.StatusSeeOther)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// IsAuthenticated reports whether the current request is authenticated as admin.
func IsAuthenticated(ctx context.Context) bool {
	v, ok := ctx.Value(isAdminKey).(bool)
	return ok && v
}
