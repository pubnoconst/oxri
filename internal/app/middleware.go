package app

import (
	"bytes"
	"context"
	"net/http"
	"time"

	"github.com/oxri/oxri-go/internal/pages/errorpage"
)

// RenderError sends the safe, branded error page with an opaque reference and QR code.
func RenderError(w http.ResponseWriter, r *http.Request, status int) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)

	ref := errorpage.NewReference()
	qr := errorpage.QRDataURI(ref)
	msg := errorpage.MessageForStatus(status)

	var buf bytes.Buffer
	component := errorpage.Page(status, msg, ref, qr)
	if err := component.Render(r.Context(), &buf); err != nil {
		// Plain safe fallback if rendering fails
		w.Write([]byte(errorpage.FallbackPage(status)))
		return
	}
	w.Write(buf.Bytes())
}

// TimeoutMiddleware bounds request processing to 30 seconds.
func TimeoutMiddleware(timeout time.Duration, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), timeout)
		defer cancel()

		r = r.WithContext(ctx)
		done := make(chan struct{})
		tw := &timeoutWriter{ResponseWriter: w}

		go func() {
			defer func() {
				if rec := recover(); rec != nil {
					// Safe 500 on panic inside handler
					if !tw.wroteHeader {
						RenderError(w, r, http.StatusInternalServerError)
					}
				}
				close(done)
			}()
			next.ServeHTTP(tw, r)
		}()

		select {
		case <-done:
			// Completed normally
		case <-ctx.Done():
			if !tw.wroteHeader {
				RenderError(w, r, http.StatusGatewayTimeout)
			}
		}
	})
}

// RecoverMiddleware prevents uncaught panics from crashing the server and hides stack traces.
func RecoverMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				RenderError(w, r, http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

type timeoutWriter struct {
	http.ResponseWriter
	wroteHeader bool
}

func (tw *timeoutWriter) WriteHeader(code int) {
	tw.wroteHeader = true
	tw.ResponseWriter.WriteHeader(code)
}

func (tw *timeoutWriter) Write(b []byte) (int, error) {
	tw.wroteHeader = true
	return tw.ResponseWriter.Write(b)
}
