package routes

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Server-Timing (PROFILING.md): each response names where the server's
// time went before it began to answer, as a standard header that DevTools,
// the lab and readers' browsers (PerformanceNavigationTiming.serverTiming)
// all read. It explains a slow first byte: "app" is the whole time before
// the headers went out; handlers add parts with timed ("db" reading the
// entries or a book, "tweets" attaching clippings). Only durations are
// sent, under generic names.

type serverTimings struct {
	mu    sync.Mutex
	start time.Time
	parts []string
}

type serverTimingKey struct{}

// timed starts a named part of the server's time for this request; call
// the returned func when it ends. Outside the middleware it does nothing.
func timed(ctx context.Context, name string) func() {
	st, _ := ctx.Value(serverTimingKey{}).(*serverTimings)
	if st == nil {
		return func() {}
	}
	begin := time.Now()
	return func() {
		ms := float64(time.Since(begin).Microseconds()) / 1000
		st.mu.Lock()
		st.parts = append(st.parts, fmt.Sprintf("%s;dur=%.1f", name, ms))
		st.mu.Unlock()
	}
}

func serverTimingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		st := &serverTimings{start: time.Now()}
		tw := &timingWriter{ResponseWriter: w, st: st}
		next.ServeHTTP(tw, r.WithContext(context.WithValue(r.Context(), serverTimingKey{}, st)))
	})
}

// timingWriter adds the Server-Timing header as the response begins.
type timingWriter struct {
	http.ResponseWriter
	st   *serverTimings
	sent bool
}

func (tw *timingWriter) stamp() {
	if tw.sent {
		return
	}
	tw.sent = true
	tw.st.mu.Lock()
	parts := append([]string{fmt.Sprintf("app;dur=%.1f", float64(time.Since(tw.st.start).Microseconds())/1000)}, tw.st.parts...)
	tw.st.mu.Unlock()
	tw.Header().Set("Server-Timing", strings.Join(parts, ", "))
}

func (tw *timingWriter) WriteHeader(code int) {
	tw.stamp()
	tw.ResponseWriter.WriteHeader(code)
}

func (tw *timingWriter) Write(b []byte) (int, error) {
	tw.stamp()
	return tw.ResponseWriter.Write(b)
}

// Flush passes through, for handlers that stream.
func (tw *timingWriter) Flush() {
	if f, ok := tw.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// Unwrap lets http.ResponseController reach the underlying writer.
func (tw *timingWriter) Unwrap() http.ResponseWriter { return tw.ResponseWriter }
