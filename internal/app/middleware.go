package app

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/oxri/oxri-go/internal/pages/errorpage"
)

// DefaultMaxBodyBytes caps incoming HTTP request bodies at 1 MiB.
const DefaultMaxBodyBytes int64 = 1 << 20

// RenderError sends the safe, branded error page with an opaque reference and QR code.
func RenderError(w http.ResponseWriter, r *http.Request, status int) {
	RenderErrorWithCause(w, r, status, nil)
}

// RenderErrorWithCause logs the opaque reference code alongside any internal failure
// to os.Stderr while returning a generic, trace-free response to the client.
func RenderErrorWithCause(w http.ResponseWriter, r *http.Request, status int, cause any) {
	ref := errorpage.NewReference()
	if status >= 500 || cause != nil {
		method, path := "", ""
		if r != nil {
			method = r.Method
			if r.URL != nil {
				path = r.URL.Path
			}
		}
		fmt.Fprintf(os.Stderr, "[ERROR] ref=%s status=%d method=%s path=%s cause=%v\n", ref, status, method, path, cause)
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)

	qr := errorpage.QRDataURI(ref)
	msg := errorpage.MessageForStatus(status)

	var buf bytes.Buffer
	component := errorpage.Page(status, msg, ref, qr)
	if err := component.Render(r.Context(), &buf); err != nil {
		_, _ = w.Write([]byte(errorpage.FallbackPage(status)))
		return
	}
	_, _ = w.Write(buf.Bytes())
}

// MaxBodyBytesMiddleware limits the size of incoming request bodies to prevent memory exhaustion.
func MaxBodyBytesMiddleware(maxBytes int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Body != nil && r.Body != http.NoBody {
				r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
			}
			next.ServeHTTP(w, r)
		})
	}
}

// CrossOriginMiddleware protects state-mutating requests (POST, PUT, DELETE, PATCH)
// against Cross-Site Request Forgery using Sec-Fetch-Site and Origin header verification.
func CrossOriginMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch:
			secFetch := strings.ToLower(strings.TrimSpace(r.Header.Get("Sec-Fetch-Site")))
			if secFetch != "" && secFetch != "same-origin" && secFetch != "none" {
				RenderErrorWithCause(w, r, http.StatusForbidden, "cross-origin Sec-Fetch-Site: "+secFetch)
				return
			}
			origin := strings.TrimSpace(r.Header.Get("Origin"))
			if origin != "" && r.Host != "" {
				u, err := url.Parse(origin)
				if err != nil || !strings.EqualFold(u.Host, r.Host) {
					RenderErrorWithCause(w, r, http.StatusForbidden, "mismatched Origin: "+origin)
					return
				}
			}
		}
		next.ServeHTTP(w, r)
	})
}

// TimeoutMiddleware bounds request processing to the given duration, returning a safe 504 error page.
// All handler writes are buffered under a mutex so the underlying http.ResponseWriter is never
// accessed concurrently across goroutines.
func TimeoutMiddleware(timeout time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, cancel := context.WithTimeout(r.Context(), timeout)
			defer cancel()

			r = r.WithContext(ctx)
			done := make(chan struct{})
			tw := &timeoutWriter{
				header: make(http.Header),
				status: http.StatusOK,
			}

			go func() {
				defer func() {
					if rec := recover(); rec != nil {
						tw.mu.Lock()
						tw.panicVal = rec
						tw.mu.Unlock()
					}
					close(done)
				}()
				next.ServeHTTP(tw, r)
			}()

			select {
			case <-done:
				tw.mu.Lock()
				if tw.timedOut {
					tw.mu.Unlock()
					return
				}
				panicVal := tw.panicVal
				if panicVal != nil {
					tw.mu.Unlock()
					RenderErrorWithCause(w, r, http.StatusInternalServerError, panicVal)
					return
				}
				for k, vals := range tw.header {
					for _, v := range vals {
						w.Header().Add(k, v)
					}
				}
				if tw.wroteHeader {
					w.WriteHeader(tw.status)
				}
				bodyBytes := tw.buf.Bytes()
				tw.mu.Unlock()
				if len(bodyBytes) > 0 {
					_, _ = w.Write(bodyBytes)
				}
			case <-ctx.Done():
				tw.mu.Lock()
				tw.timedOut = true
				tw.mu.Unlock()
				RenderErrorWithCause(w, r, http.StatusGatewayTimeout, ctx.Err())
			}
		})
	}
}

// RecoverMiddleware safely catches uncaught panics and renders the branded 500 error page.
func RecoverMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				RenderErrorWithCause(w, r, http.StatusInternalServerError, rec)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

type timeoutWriter struct {
	mu          sync.Mutex
	header      http.Header
	buf         bytes.Buffer
	status      int
	wroteHeader bool
	timedOut    bool
	panicVal    any
}

func (tw *timeoutWriter) Header() http.Header {
	tw.mu.Lock()
	defer tw.mu.Unlock()
	if tw.timedOut {
		return make(http.Header)
	}
	return tw.header
}

func (tw *timeoutWriter) WriteHeader(code int) {
	tw.mu.Lock()
	defer tw.mu.Unlock()
	if tw.timedOut || tw.wroteHeader {
		return
	}
	tw.status = code
	tw.wroteHeader = true
}

func (tw *timeoutWriter) Write(b []byte) (int, error) {
	tw.mu.Lock()
	defer tw.mu.Unlock()
	if tw.timedOut {
		return 0, http.ErrHandlerTimeout
	}
	if !tw.wroteHeader {
		tw.status = http.StatusOK
		tw.wroteHeader = true
	}
	return tw.buf.Write(b)
}
