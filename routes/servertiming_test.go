package routes

import (
	"context"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"

	"github.com/oxri/oxri-go/internal/db"
)

// A page says where the server's time went (Server-Timing), compressed or
// not: the whole time before the headers, and the named parts.
func TestPagesCarryServerTiming(t *testing.T) {
	store, err := db.Open(context.Background(), ":memory:", "")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()
	router := NewRouter(Config{Store: store})
	for _, path := range []string{"/journals", "/book-i"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.Header.Set("Accept-Encoding", "gzip")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		st := rec.Header().Get("Server-Timing")
		if !regexp.MustCompile(`^app;dur=[0-9.]+, db;dur=[0-9.]+`).MatchString(st) {
			t.Errorf("%s: Server-Timing %q", path, st)
		}
	}
}
