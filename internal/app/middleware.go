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
		w.Write([]byte(errorpage.FallbackPage(status)))
		return
	}
	w.Write(buf.Bytes())
}

// TimeoutMiddleware bounds request processing to the given duration, returning a safe 504 error page.
func TimeoutMiddleware(timeout time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, cancel := context.WithTimeout(r.Context(), timeout)
			defer cancel()

			r = r.WithContext(ctx)
			done := make(chan struct{})
			tw := &timeoutWriter{ResponseWriter: w}

			go func() {
				defer func() {
					if rec := recover(); rec != nil {
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
			case <-ctx.Done():
				if !tw.wroteHeader {
					RenderError(w, r, http.StatusGatewayTimeout)
				}
			}
		})
	}
}

// RecoverMiddleware safely catches uncaught panics and renders the branded 500 error page.
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
