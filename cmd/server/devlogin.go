package main

import (
	"errors"
	"net"
	"net/http"
	"os"
	"sync"

	"github.com/oxri/oxri-go/internal/auth"
)

// devAutoLoginEnv turns on the local test server's automatic sign-in:
// DEV_AUTO_LOGIN=1 signs every request in as the admin, so the owner can try
// a branch against the real database without the password and code.
const devAutoLoginEnv = "DEV_AUTO_LOGIN"

// checkDevAutoLogin refuses automatic sign-in anywhere but a server that only
// this machine can reach: it must listen on a loopback address, and not run
// on Render or as prod.
func checkDevAutoLogin(addr string, svc *auth.Service) error {
	if os.Getenv("RENDER") != "" || os.Getenv("APP_ENV") == "prod" {
		return errors.New("refused: this is Render or prod")
	}
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return errors.New("refused: cannot read the listen address")
	}
	if host != "localhost" {
		ip := net.ParseIP(host)
		if ip == nil || !ip.IsLoopback() {
			return errors.New("refused: the server must listen on 127.0.0.1, ::1 or localhost")
		}
	}
	if svc == nil {
		return errors.New("refused: no auth service (ADMIN_TOTP_SECRET unset)")
	}
	return nil
}

// devAutoLogin gives a request without a valid session the server's one dev
// session, both on the request and as a cookie for the browser. Every other
// check (CSRF, freshness for credential pages) still runs as usual.
func devAutoLogin(svc *auth.Service, next http.Handler) http.Handler {
	var (
		mu  sync.Mutex
		sid string
	)
	session := func() (string, error) {
		mu.Lock()
		defer mu.Unlock()
		if _, ok := svc.Authenticate(sid); sid != "" && ok {
			return sid, nil
		}
		s, err := svc.CreateSession()
		if err != nil {
			return "", err
		}
		sid = s
		return sid, nil
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if c, err := r.Cookie(auth.SessionCookieName); err == nil {
			if _, ok := svc.Authenticate(c.Value); ok {
				next.ServeHTTP(w, r)
				return
			}
		}
		s, err := session()
		if err != nil {
			next.ServeHTTP(w, r)
			return
		}
		r = r.Clone(r.Context())
		cookies := r.Cookies()
		r.Header.Del("Cookie")
		for _, c := range cookies {
			if c.Name != auth.SessionCookieName {
				r.AddCookie(c)
			}
		}
		r.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: s})
		http.SetCookie(w, &http.Cookie{
			Name:     auth.SessionCookieName,
			Value:    s,
			Path:     "/",
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
			MaxAge:   int(auth.SessionDuration.Seconds()),
		})
		next.ServeHTTP(w, r)
	})
}
