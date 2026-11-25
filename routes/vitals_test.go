package routes

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/oxri/oxri-go/internal/db"
)

// A reader's report is stored under its route template, rated by the
// server; malformed samples, the admin's own views and local servers are not.
func TestVitalsAreRecordedFromReadersOnly(t *testing.T) {
	ctx := context.Background()
	store, err := db.Open(ctx, ":memory:", "")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()
	authSvc, session := createTestTOTPAuth(t)
	defer func() { _ = authSvc.Close() }()
	router := NewRouter(Config{Store: store, AuthService: authSvc})

	post := func(host, body string, signedIn bool) int {
		req := httptest.NewRequest(http.MethodPost, "/vitals", strings.NewReader(body))
		req.Host = host
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Sec-Fetch-Site", "same-origin")
		if signedIn {
			req.AddCookie(session)
		}
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		return rec.Code
	}
	count := func() int {
		stats, err := store.VitalsSummary(ctx, time.Now(), "")
		if err != nil {
			t.Fatal(err)
		}
		n := 0
		for _, s := range stats {
			n += s.Count
		}
		return n
	}

	reader := `{"page":"/journals/42","device":"mobile","network":"4g","memory":2,"samples":[
		{"name":"LCP","value":3100,"rating":"good","target":"p.journal-entry-excerpt"},
		{"name":"INP","value":120,"rating":"good","target":"input#search"},
		{"name":"CLS","value":0.02,"rating":"good","target":""},
		{"name":"FCP","value":900,"rating":"good"},
		{"name":"LCP","value":1,"rating":"good"}]}`
	if code := post("res-imperii.example", reader, false); code != http.StatusNoContent {
		t.Fatalf("reader report: got %d, want 204", code)
	}
	if n := count(); n != 3 {
		t.Fatalf("stored %d samples, want 3 (FCP and a repeated LCP dropped)", n)
	}
	stats, _ := store.VitalsSummary(ctx, time.Now(), "/journals/:id")
	for _, s := range stats {
		if s.Metric == "LCP" && (s.Rating != "needs-improvement" || s.P75 != 3100) {
			t.Errorf("LCP 3100 ms must be rated by the server as needs-improvement, got %q", s.Rating)
		}
	}

	for name, body := range map[string]string{
		"bad device":   `{"page":"/","device":"fridge","samples":[{"name":"LCP","value":100}]}`,
		"negative":     `{"page":"/","device":"mobile","samples":[{"name":"INP","value":-5}]}`,
		"huge CLS":     `{"page":"/","device":"mobile","samples":[{"name":"CLS","value":1e9}]}`,
		"not JSON":     `LCP=100`,
		"unknown kind": `{"page":"/","device":"mobile","samples":[{"name":"XYZ","value":1}]}`,
	} {
		before := count()
		_ = post("res-imperii.example", body, false)
		if count() != before {
			t.Errorf("%s: stored", name)
		}
	}

	before := count()
	_ = post("res-imperii.example", reader, true)
	if count() != before {
		t.Error("the admin's own view was recorded")
	}
	_ = post("127.0.0.1:8080", reader, false)
	_ = post("localhost:8080", reader, false)
	if count() != before {
		t.Error("a view on a local server was recorded")
	}
}

func TestVitalsPageTemplates(t *testing.T) {
	for path, want := range map[string]string{
		"/":                       "/",
		"/journals":               "/journals",
		"/journals/12":            "/journals/:id",
		"/journals/12/edit":       "/journals/:id/edit",
		"/book-ii":                "/book-ii",
		"/journals/vacate-space/": "/journals/vacate-space",
		"/wp-admin":               "other",
		"/journals/12/../x":       "other",
	} {
		if got := vitalsPage(path); got != want {
			t.Errorf("vitalsPage(%q) = %q, want %q", path, got, want)
		}
	}
}

// Rejected selectors are dropped, not the sample.
func TestVitalsTargetsArePlainSelectors(t *testing.T) {
	rep := vitalReport{Page: "/", Device: "desktop"}
	rep.Samples = append(rep.Samples, struct {
		Name   string  `json:"name"`
		Value  float64 `json:"value"`
		Rating string  `json:"rating"`
		Target string  `json:"target"`
	}{Name: "INP", Value: 80, Target: "<img src=x onerror=alert(1)>"})
	vs := validVitals(rep, time.Now())
	if len(vs) != 1 || vs[0].Target != "" {
		t.Fatalf("got %+v", vs)
	}
}
