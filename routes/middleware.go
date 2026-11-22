package routes

import (
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/oxri/oxri-go/views/errorpage"
)

const (
	// MaxRequestBodyBytes caps all incoming HTTP request bodies at 1 MiB.
	MaxRequestBodyBytes = 1 << 20

	// ContentSecurityPolicy enforces strict origin restrictions with zero 'unsafe-eval'.
	ContentSecurityPolicy = "default-src 'self'; " +
		"script-src 'self' 'unsafe-inline' https://platform.twitter.com; " +
		"style-src 'self' 'unsafe-inline' https://fonts.googleapis.com; " +
		"font-src 'self' https://fonts.gstatic.com; " +
		"img-src 'self' data: https: http:; " +
		"media-src 'self' https: http:; " +
		"frame-src https://platform.twitter.com https://syndication.twitter.com https://www.youtube.com https://www.youtube-nocookie.com https://player.vimeo.com; " +
		"connect-src 'self'; " +
		"object-src 'none'; " +
		"base-uri 'self'; " +
		"form-action 'self'; " +
		"frame-ancestors 'none'"
)

// securityHeadersMiddleware attaches strict defensive HTTP headers to every response.
func securityHeadersMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("Content-Security-Policy", ContentSecurityPolicy)
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		h.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=(), payment=(), usb=()")
		if r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https") {
			h.Set("Strict-Transport-Security", "max-age=63072000; includeSubDomains")
		}
		next.ServeHTTP(w, r)
	})
}

// bodySizeCapMiddleware caps all incoming HTTP request bodies at MaxRequestBodyBytes (1 MiB).
func bodySizeCapMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Body != nil {
			r.Body = http.MaxBytesReader(w, r.Body, MaxRequestBodyBytes)
		}
		next.ServeHTTP(w, r)
	})
}

// csrfOriginMiddleware protects state-changing requests (POST/PUT/DELETE/PATCH)
// against Cross-Site Request Forgery by inspecting Sec-Fetch-Site and Origin headers.
func csrfOriginMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch:
			if sfs := strings.ToLower(strings.TrimSpace(r.Header.Get("Sec-Fetch-Site"))); sfs != "" {
				if sfs != "same-origin" && sfs != "none" {
					errorpage.Render(w, http.StatusForbidden, "")
					return
				}
			}
			if origin := strings.TrimSpace(r.Header.Get("Origin")); origin != "" {
				u, err := url.Parse(origin)
				if err != nil || u.Host == "" || !strings.EqualFold(u.Host, r.Host) {
					errorpage.Render(w, http.StatusForbidden, "")
					return
				}
			}
		}
		next.ServeHTTP(w, r)
	})
}

// noDirListingAssetHandler serves static files from assetsDir while returning a 404
// error page for any directory path or traversal attempt (disabling directory listings).
func noDirListingAssetHandler(assetsDir string) http.Handler {
	fs := http.FileServer(http.Dir(assetsDir))
	stripped := http.StripPrefix("/assets/", fs)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		relPath := strings.TrimPrefix(r.URL.Path, "/assets/")
		if relPath == "" || strings.HasSuffix(r.URL.Path, "/") || strings.Contains(relPath, "..") {
			errorpage.Render(w, http.StatusNotFound, "")
			return
		}
		cleanRel := filepath.FromSlash(filepath.Clean("/" + relPath))
		fullPath := filepath.Join(assetsDir, cleanRel)
		info, err := os.Stat(fullPath)
		if err != nil || info.IsDir() {
			errorpage.Render(w, http.StatusNotFound, "")
			return
		}
		stripped.ServeHTTP(w, r)
	})
}

// ipRateLimiter enforces per-IP request limits over a sliding window.
type ipRateLimiter struct {
	mu      sync.Mutex
	window  time.Duration
	maxReqs int
	hits    map[string][]time.Time
}

func newIPRateLimiter(maxReqs int, window time.Duration) *ipRateLimiter {
	return &ipRateLimiter{
		window:  window,
		maxReqs: maxReqs,
		hits:    make(map[string][]time.Time),
	}
}

func (l *ipRateLimiter) allow(ip string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-l.window)

	recent := l.hits[ip][:0]
	for _, t := range l.hits[ip] {
		if t.After(cutoff) {
			recent = append(recent, t)
		}
	}
	if len(recent) >= l.maxReqs {
		l.hits[ip] = recent
		return false
	}
	l.hits[ip] = append(recent, now)
	return true
}

func clientIPFromRequest(r *http.Request) string {
	host, _, err := net.SplitHostPort(strings.TrimSpace(r.RemoteAddr))
	if err == nil && host != "" {
		return host
	}
	if r.RemoteAddr != "" {
		return r.RemoteAddr
	}
	return "127.0.0.1"
}
