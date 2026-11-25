package routes

import (
	"encoding/json"
	"io"
	"math"
	"net"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/oxri/oxri-go/internal/auth"
	"github.com/oxri/oxri-go/internal/db"
	"github.com/oxri/oxri-go/serverfns"
)

// Field performance (PROFILING.md, HIG §1.6): readers' browsers report Core
// Web Vitals for the page they leave (assets/js/vitals.js), and the server
// keeps them for the p75 summary in Settings.
//
// What is recorded is only what the summary needs: the route template, the
// metric and its value and rating, mobile or desktop, the effective
// connection type, the device memory class, and the CSS selector of the
// element the browser blamed. No address, agent string or identifier.
//
// Not recorded: the admin's own views (signed in), and anything served to a
// loopback host (the local test server, lab runs), so the numbers are readers'.

// maxVitalsBody bounds one report (a page view sends at most five samples).
const maxVitalsBody = 4 << 10

type vitalReport struct {
	Page    string  `json:"page"`
	Device  string  `json:"device"`
	Network string  `json:"network"`
	Memory  float64 `json:"memory"`
	Samples []struct {
		Name   string  `json:"name"`
		Value  float64 `json:"value"`
		Rating string  `json:"rating"`
		Target string  `json:"target"`
	} `json:"samples"`
}

var (
	journalEntryPath = regexp.MustCompile(`^/journals/\d+$`)
	journalEditPath  = regexp.MustCompile(`^/journals/\d+/edit$`)
	networkTypes     = map[string]bool{"slow-2g": true, "2g": true, "3g": true, "4g": true, "": true}
	// A selector as web-vitals writes one: tags, classes, ids, :nth-child.
	selectorChars = regexp.MustCompile(`^[A-Za-z0-9 _>#.:()\[\]="'-]*$`)
)

// vitalsPage maps a path to the route template it is reported under; any
// path outside the site's pages is "other".
func vitalsPage(path string) string {
	switch {
	case path == "/", path == "/journals", path == "/journals/new", path == "/login", path == "/settings",
		path == "/book-i", path == "/book-ii", path == "/book-iii", path == "/offline":
		return path
	case journalEntryPath.MatchString(path):
		return "/journals/:id"
	case journalEditPath.MatchString(path):
		return "/journals/:id/edit"
	case strings.HasPrefix(path, "/journals/vacate-space"):
		return "/journals/vacate-space"
	default:
		return "other"
	}
}

func isLoopbackHost(hostport string) bool {
	host := hostport
	if h, _, err := net.SplitHostPort(hostport); err == nil {
		host = h
	}
	host = strings.Trim(host, "[]")
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func handleVitals(store *db.Store, authSvc *auth.Service, limiter *ipRateLimiter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// The browser does not wait on or read the answer (sendBeacon).
		defer w.WriteHeader(http.StatusNoContent)
		if store == nil || isLoopbackHost(r.Host) {
			return
		}
		if _, err := serverfns.RequireAuth(authSvc, r); err == nil {
			return // the admin's own views are not readers'
		}
		if limiter != nil && !limiter.allow(clientIPFromRequest(r)) {
			return
		}
		body, err := io.ReadAll(io.LimitReader(r.Body, maxVitalsBody+1))
		if err != nil || len(body) > maxVitalsBody {
			return
		}
		var rep vitalReport
		if json.Unmarshal(body, &rep) != nil {
			return
		}
		if vs := validVitals(rep, time.Now()); len(vs) > 0 {
			_ = store.PutVitals(r.Context(), vs)
		}
	}
}

// validVitals keeps the samples that are well-formed; the rating is the
// server's own, by Google's thresholds, not the browser's word for it.
func validVitals(rep vitalReport, now time.Time) []db.Vital {
	if rep.Device != "mobile" && rep.Device != "desktop" {
		return nil
	}
	if !networkTypes[rep.Network] {
		rep.Network = ""
	}
	if math.IsNaN(rep.Memory) || rep.Memory < 0 || rep.Memory > 64 {
		rep.Memory = 0
	}
	page := vitalsPage(rep.Page)
	seen := map[string]bool{}
	var out []db.Vital
	for i, s := range rep.Samples {
		if i >= 5 {
			break
		}
		if _, ok := db.Thresholds[s.Name]; !ok || seen[s.Name] {
			continue
		}
		limit := 120000.0
		if s.Name == "CLS" {
			limit = 100
		}
		if math.IsNaN(s.Value) || math.IsInf(s.Value, 0) || s.Value < 0 || s.Value > limit {
			continue
		}
		target := s.Target
		if len(target) > 100 || !selectorChars.MatchString(target) {
			target = ""
		}
		seen[s.Name] = true
		out = append(out, db.Vital{
			At: now, Page: page, Metric: s.Name, Value: s.Value, Rating: db.RateVital(s.Name, s.Value),
			Device: rep.Device, Network: rep.Network, Memory: rep.Memory, Target: target,
		})
	}
	return out
}
