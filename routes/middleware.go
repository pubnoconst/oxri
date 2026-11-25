package routes

import (
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/oxri/oxri-go/views/errorpage"
)

const (
	// MaxRequestBodyBytes caps all incoming HTTP request bodies at 1 MiB.
	MaxRequestBodyBytes = 1 << 20

	defaultMaxRateLimiterEntries = 4096

	// ContentSecurityPolicy enforces strict origin restrictions with zero 'unsafe-eval'
	// and zero 'unsafe-inline' scripts.
	ContentSecurityPolicy = "default-src 'self'; " +
		"script-src 'self' https://platform.twitter.com; " +
		"style-src 'self' 'unsafe-inline'; " +
		"font-src 'self'; " +
		"img-src 'self' data: https: http:; " +
		"media-src 'self' https: http:; " +
		"frame-src https://platform.twitter.com https://syndication.twitter.com https://www.youtube.com https://www.youtube-nocookie.com https://player.vimeo.com; " +
		"connect-src 'self' https://bible.helloao.org; " +
		"object-src 'none'; " +
		"base-uri 'self'; " +
		"form-action 'self'; " +
		"frame-ancestors 'none'"
)

var cgnatPrefix = netip.MustParsePrefix("100.64.0.0/10")

// securityHeadersMiddleware attaches strict defensive HTTP headers to every response.
func securityHeadersMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("Content-Security-Policy", ContentSecurityPolicy)
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		h.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=(), payment=(), usb=()")
		if r.TLS != nil || (isTrustedProxyRequest(r) && strings.EqualFold(strings.TrimSpace(r.Header.Get("X-Forwarded-Proto")), "https")) {
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

// ipRateLimiter enforces bounded-memory per-IP request limits over a sliding window.
type ipRateLimiter struct {
	mu         sync.Mutex
	window     time.Duration
	maxReqs    int
	maxEntries int
	lastSweep  time.Time
	hits       map[string][]time.Time
}

func newIPRateLimiter(maxReqs int, window time.Duration) *ipRateLimiter {
	return &ipRateLimiter{
		window:     window,
		maxReqs:    maxReqs,
		maxEntries: defaultMaxRateLimiterEntries,
		hits:       make(map[string][]time.Time),
	}
}

func (l *ipRateLimiter) sweepLocked(now time.Time) {
	cutoff := now.Add(-l.window)
	for ip, times := range l.hits {
		kept := times[:0]
		for _, t := range times {
			if t.After(cutoff) {
				kept = append(kept, t)
			}
		}
		if len(kept) == 0 {
			delete(l.hits, ip)
		} else {
			l.hits[ip] = kept
		}
	}
	l.lastSweep = now
}

func (l *ipRateLimiter) allow(ip string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	maxEntries := l.maxEntries
	if maxEntries <= 0 {
		maxEntries = defaultMaxRateLimiterEntries
	}

	if now.Sub(l.lastSweep) >= l.window || len(l.hits) >= maxEntries {
		l.sweepLocked(now)
	}

	if _, exists := l.hits[ip]; !exists && len(l.hits) >= maxEntries {
		var oldestIP string
		var oldestTime time.Time
		for existingIP, times := range l.hits {
			var latest time.Time
			if len(times) > 0 {
				latest = times[len(times)-1]
			}
			if oldestIP == "" || latest.Before(oldestTime) {
				oldestIP = existingIP
				oldestTime = latest
			}
		}
		if oldestIP != "" {
			delete(l.hits, oldestIP)
		}
	}

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

func parseHeaderIP(raw string) (string, bool) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", false
	}
	addr, err := netip.ParseAddr(trimmed)
	if err != nil {
		return "", false
	}
	return addr.Unmap().String(), true
}

func isTrustedProxyAddr(remoteAddr string) bool {
	raw := strings.TrimSpace(remoteAddr)
	if raw == "" {
		return false
	}
	host, _, err := net.SplitHostPort(raw)
	if err != nil {
		host = raw
	}
	addr, err := netip.ParseAddr(strings.TrimSpace(host))
	if err != nil {
		return false
	}
	addr = addr.Unmap()

	if addr.IsLoopback() || addr.IsPrivate() || addr.IsLinkLocalUnicast() || cgnatPrefix.Contains(addr) {
		return true
	}

	if extra := strings.TrimSpace(os.Getenv("TRUSTED_PROXY_CIDRS")); extra != "" {
		for _, item := range strings.Split(extra, ",") {
			token := strings.TrimSpace(item)
			if token == "" {
				continue
			}
			if prefix, pErr := netip.ParsePrefix(token); pErr == nil && prefix.Contains(addr) {
				return true
			}
			if single, aErr := netip.ParseAddr(token); aErr == nil && single.Unmap() == addr {
				return true
			}
		}
	}

	return false
}

func isTrustedProxyRequest(r *http.Request) bool {
	if r == nil {
		return false
	}
	if !strings.EqualFold(strings.TrimSpace(os.Getenv("TRUST_PROXY")), "true") {
		return false
	}
	return isTrustedProxyAddr(r.RemoteAddr)
}

func clientIPFromRequest(r *http.Request) string {
	if isTrustedProxyRequest(r) {
		if ip, ok := parseHeaderIP(r.Header.Get("True-Client-IP")); ok {
			return ip
		}
		if ip, ok := parseHeaderIP(r.Header.Get("CF-Connecting-IP")); ok {
			return ip
		}
		if xff := strings.TrimSpace(r.Header.Get("X-Forwarded-For")); xff != "" {
			var ips []string
			for _, part := range strings.Split(xff, ",") {
				if ip, ok := parseHeaderIP(part); ok {
					ips = append(ips, ip)
				}
			}
			if len(ips) > 0 {
				hops := 1
				if rawHops := strings.TrimSpace(os.Getenv("TRUSTED_PROXY_HOPS")); rawHops != "" {
					if parsed, err := strconv.Atoi(rawHops); err == nil && parsed > 0 {
						hops = parsed
					}
				}
				idx := len(ips) - hops
				if idx < 0 {
					idx = 0
				}
				return ips[idx]
			}
		}
	}

	raw := strings.TrimSpace(r.RemoteAddr)
	if host, _, err := net.SplitHostPort(raw); err == nil && host != "" {
		if ip, ok := parseHeaderIP(host); ok {
			return ip
		}
		return host
	}
	if ip, ok := parseHeaderIP(raw); ok {
		return ip
	}
	if raw != "" {
		return raw
	}
	return "127.0.0.1"
}

func isHTTPSRequest(r *http.Request) bool {
	if r.TLS != nil {
		return true
	}
	if isTrustedProxyRequest(r) && strings.EqualFold(strings.TrimSpace(r.Header.Get("X-Forwarded-Proto")), "https") {
		return true
	}
	return strings.EqualFold(strings.TrimSpace(os.Getenv("APP_ENV")), "prod")
}
