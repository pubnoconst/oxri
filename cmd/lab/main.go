// Command lab is the repeatable mobile lab run behind PROFILING.md: it loads
// pages in headless Chrome emulating a budget phone on a slow mobile network,
// from a cold cache, several times each, taps and types as a reader would,
// and reports the Core Web Vitals per page as median, spread and worst run,
// rated by Google's thresholds (HIG §1.6: the spread matters as much as the
// median).
//
// The vitals are measured by the same vendored web-vitals library the site
// sends field data with, injected before the page's own scripts, so the lab
// and the field measure alike.
//
//	go run ./cmd/lab                                   # local server, 5 runs
//	go run ./cmd/lab -base https://example.org -runs 7
//	go run ./cmd/lab -beacon                           # the field report is complete
//	mage lab
//
// Emulation (Lighthouse's mobile defaults): 412×823 CSS px at DPR 1.75, touch,
// an Android Chrome user agent, CPU slowed 4×, 150 ms latency, 1.6 Mb/s down
// and 750 kb/s up. The site's own /vitals beacon is blocked, so lab runs never
// count as field data.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/chromedp/cdproto/emulation"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"

	"github.com/oxri/oxri-go/internal/db"
)

const userAgent = "Mozilla/5.0 (Linux; Android 11; SM-A025F) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/158.0.0.0 Mobile Safari/537.36"

// step is one reader action after load: tap a selector, then type text into
// it if any. Taps are on elements that do not navigate.
type step struct{ tap, typeText string }

// pages are the paths measured by default, each with what a reader does.
var pages = map[string][]step{
	"/":         {{tap: ".landing-title"}},
	"/journals": {{tap: ".journals-search-form input", typeText: "god"}},
	"/book-i":   {{tap: ".book-content p"}},
	"/login":    {{tap: "input[name=code]", typeText: "123"}},
}

// collector records the latest value of each metric (reportAllChanges), so
// INP includes the scripted interactions without hiding the page. It also
// keeps the field report vitals.js builds as the page is left (the beacon
// itself is blocked), for -beacon to read from the next page.
const collector = `
(() => {
  window.__lab = {};
  const keep = (m) => { window.__lab[m.name] = m.value; };
  const opts = { reportAllChanges: true };
  webVitals.onLCP(keep, opts); webVitals.onINP(keep, opts);
  webVitals.onCLS(keep, opts); webVitals.onTTFB(keep, opts); webVitals.onFCP(keep, opts);
  const B = window.Blob;
  window.Blob = function (parts, o) {
    if (parts && typeof parts[0] === "string" && parts[0].includes('"samples"')) localStorage.setItem("lab-beacon", parts[0]);
    return new B(parts, o);
  };
})();`

type sample map[string]float64

// timelineJS lists paints, resources and font faces in use (-debug).
const timelineJS = `[
  ...performance.getEntriesByType("paint").map((p) => [Math.round(p.startTime), "paint", p.name]),
  ...performance.getEntriesByType("resource").map((r) => [Math.round(r.startTime), Math.round(r.responseEnd), r.initiatorType + " " + new URL(r.name).pathname]),
].sort((a, b) => a[0] - b[0]).map((e) => e.join("  ")).join(String.fromCharCode(10)) + String.fromCharCode(10) + "fonts: " +
  [...document.fonts].filter((f) => f.status !== "unloaded").map((f) => f.family + " " + f.unicodeRange.slice(0, 8) + " " + f.status).join(", ")`

type config struct {
	base     string
	vitalsJS string
	cpu      float64
	debug    bool
}

func main() {
	base := flag.String("base", "http://127.0.0.1:8080", "site to measure")
	runs := flag.Int("runs", 5, "cold-cache runs per page")
	only := flag.String("pages", "", "comma-separated paths (default: all of "+strings.Join(sortedKeys(pages), ", ")+")")
	cpu := flag.Float64("cpu", 4, "CPU slowdown factor")
	lib := flag.String("lib", "assets/js/web-vitals.min.js", "the vendored web-vitals build")
	out := flag.String("out", "", "also write the report (Markdown) to this file")
	debug := flag.Bool("debug", false, "print each run's paint, resource and font timeline")
	warm := flag.Bool("warm", true, "request each page once first, so the server's own cold start is not counted against the page (field data covers the server)")
	local := flag.Bool("local", false, "serve the site in this process from an in-memory database with seeded content, and measure that (no credentials needed; CI uses it)")
	budget := flag.Bool("budget", false, "exit 1 if any page misses Google's good line: median LCP > 2.5 s, any run's CLS > 0.1, or median INP > 200 ms")
	beacon := flag.Bool("beacon", false, "check instead that leaving each page builds a field report with LCP, INP, CLS and TTFB (vitals.js); exits 1 if not")
	flag.Parse()

	vitalsJS, err := os.ReadFile(*lib)
	if err != nil {
		fail("read web-vitals (run from the repository root): %v", err)
	}
	if *local {
		url, stop, err := startLocal(context.Background())
		if err != nil {
			fail("local server: %v", err)
		}
		cleanups = append(cleanups, stop)
		defer stop()
		*base = url
		fmt.Fprintln(os.Stderr, "lab: serving the seeded site at", url)
	}
	cfg := config{base: *base, vitalsJS: string(vitalsJS), cpu: *cpu, debug: *debug}
	paths := sortedKeys(pages)
	if *only != "" {
		paths = strings.Split(*only, ",")
	}

	if *beacon {
		failed := false
		for _, p := range paths {
			got, err := checkBeacon(cfg, p)
			if err != nil {
				failed = true
				fmt.Printf("%s: %v\n", p, err)
				continue
			}
			fmt.Printf("%s: %s\n", p, got)
		}
		if failed {
			exit(1)
		}
		return
	}

	if *warm {
		for _, p := range append(paths, "/journals/content") {
			if res, err := http.Get(*base + p); err == nil {
				_, _ = io.Copy(io.Discard, res.Body)
				_ = res.Body.Close()
			}
		}
	}

	var report strings.Builder
	fmt.Fprintf(&report, "# Lab run %s\n\n", time.Now().Format("2006-01-02 15:04"))
	fmt.Fprintf(&report, "`%s`, %d cold-cache runs per page; budget phone: 412×823 @1.75, CPU %g× slower, 150 ms RTT, 1.6 Mb/s down. Median / spread (max − min) / worst. Good: LCP ≤ 2.5 s, INP ≤ 200 ms, CLS ≤ 0.1.\n\n", *base, *runs, *cpu)
	fmt.Fprintf(&report, "| Page | LCP | INP | CLS | TTFB | FCP |\n|---|---|---|---|---|---|\n")
	var misses []string
	for _, p := range paths {
		var results []sample
		for i := 0; i < *runs; i++ {
			s, err := measure(cfg, p)
			if err != nil {
				fmt.Fprintf(os.Stderr, "%s run %d: %v\n", p, i+1, err)
				continue
			}
			results = append(results, s)
			fmt.Fprintf(os.Stderr, "%s run %d: LCP %.0f  INP %.0f  CLS %.3f  TTFB %.0f\n", p, i+1, s["LCP"], s["INP"], s["CLS"], s["TTFB"])
		}
		fmt.Fprintf(&report, "| `%s` | %s | %s | %s | %s | %s |\n", p,
			cell(results, "LCP"), cell(results, "INP"), cell(results, "CLS"), cell(results, "TTFB"), cell(results, "FCP"))
		misses = append(misses, checkBudget(p, results, *runs)...)
	}
	fmt.Fprintf(&report, "\nA cell reads *median ± spread (worst)* and the median's rating. `–` is a metric the page never produced (no interaction, or no shift).\n")

	if *budget {
		if len(misses) == 0 {
			report.WriteString("\n**Budget: met** on every page (median LCP ≤ 2.5 s, every run's CLS ≤ 0.1, median INP ≤ 200 ms).\n")
		} else {
			report.WriteString("\n**Budget: missed**\n\n")
			for _, m := range misses {
				report.WriteString("- " + m + "\n")
			}
		}
	}

	fmt.Print(report.String())
	if *out != "" {
		if err := os.WriteFile(*out, []byte(report.String()), 0o644); err != nil {
			fail("write %s: %v", *out, err)
		}
	}
	if *budget && len(misses) > 0 {
		exit(1)
	}
}

// checkBudget holds a page to Google's good lines (-budget). CLS is held on
// every run, since layout is deterministic and a shift on any run is real;
// LCP and INP on the median, since a shared CI runner's speed varies. Every
// run must also have produced its vitals.
func checkBudget(page string, results []sample, runs int) []string {
	var misses []string
	if len(results) < runs {
		misses = append(misses, fmt.Sprintf("`%s`: %d of %d runs failed", page, runs-len(results), runs))
	}
	values := func(m string) []float64 {
		var vs []float64
		for _, r := range results {
			if v, ok := r[m]; ok {
				vs = append(vs, v)
			}
		}
		sort.Float64s(vs)
		return vs
	}
	if vs := values("LCP"); len(vs) == 0 {
		misses = append(misses, fmt.Sprintf("`%s`: no LCP recorded", page))
	} else if m := median(vs); m > db.Thresholds["LCP"][0] {
		misses = append(misses, fmt.Sprintf("`%s`: median LCP %.0f ms > 2500 ms", page, m))
	}
	if vs := values("CLS"); len(vs) > 0 && vs[len(vs)-1] > db.Thresholds["CLS"][0] {
		misses = append(misses, fmt.Sprintf("`%s`: a run's CLS %.3f > 0.1", page, vs[len(vs)-1]))
	}
	if vs := values("INP"); len(vs) > 0 && median(vs) > db.Thresholds["INP"][0] {
		misses = append(misses, fmt.Sprintf("`%s`: median INP %.0f ms > 200 ms", page, median(vs)))
	}
	return misses
}

// browser starts a fresh headless Chrome with its own empty profile, so an
// empty cache as on a first visit. DevTools' "disable cache" is not used: it
// also stops a preloaded resource being reused, which is not a first visit.
func browser() (context.Context, func()) {
	opts := append(chromedp.DefaultExecAllocatorOptions[:], chromedp.Flag("disable-extensions", true))
	if os.Getenv("CI") != "" {
		// GitHub's Ubuntu runners restrict the user namespaces Chrome's
		// sandbox needs; the pages measured are our own.
		opts = append(opts, chromedp.NoSandbox)
	}
	alloc, cancelAlloc := chromedp.NewExecAllocator(context.Background(), opts...)
	ctx, cancel := chromedp.NewContext(alloc)
	ctx, cancelTimeout := context.WithTimeout(ctx, 90*time.Second)
	return ctx, func() { cancelTimeout(); cancel(); cancelAlloc() }
}

// emulate sets up the budget phone and the collector in a target.
func emulate(cfg config) chromedp.Action[chromedp.Void] {
	return func(ctx context.Context, _ *chromedp.Target) (chromedp.Void, error) {
		var err error
		try := func(f func() error) {
			if err == nil {
				err = f()
			}
		}
		try(func() error { _, e := chromedp.Call(ctx, network.Enable, network.EnableParams{}); return e })
		try(func() error {
			_, e := chromedp.Call(ctx, network.SetBlockedURLs, network.SetBlockedURLsParams{URLPatterns: []*network.BlockPattern{{URLPattern: "*://*:*/vitals", Block: true}}})
			return e
		})
		try(func() error {
			_, e := chromedp.Call(ctx, network.EmulateNetworkConditionsByRule, network.EmulateNetworkConditionsByRuleParams{
				MatchedNetworkConditions: []*network.Conditions{{Latency: 150, DownloadThroughput: 1.6e6 / 8, UploadThroughput: 750e3 / 8}},
			})
			return e
		})
		try(func() error {
			_, e := chromedp.Call(ctx, emulation.SetDeviceMetricsOverride, emulation.SetDeviceMetricsOverrideParams{Width: 412, Height: 823, DeviceScaleFactor: 1.75, Mobile: true})
			return e
		})
		try(func() error {
			_, e := chromedp.Call(ctx, emulation.SetTouchEmulationEnabled, emulation.SetTouchEmulationEnabledParams{Enabled: true, MaxTouchPoints: 5})
			return e
		})
		try(func() error {
			_, e := chromedp.Call(ctx, emulation.SetUserAgentOverride, emulation.SetUserAgentOverrideParams{UserAgent: userAgent})
			return e
		})
		try(func() error {
			_, e := chromedp.Call(ctx, emulation.SetCPUThrottlingRate, emulation.SetCPUThrottlingRateParams{Rate: cfg.cpu})
			return e
		})
		try(func() error {
			_, e := chromedp.Call(ctx, page.AddScriptToEvaluateOnNewDocument, page.AddScriptToEvaluateOnNewDocumentParams{Source: cfg.vitalsJS + "\n" + collector})
			return e
		})
		return chromedp.Void{}, err
	}
}

// visit loads path in an emulated target, settles, and does what a reader
// does there.
func visit(ctx context.Context, cfg config, path string) error {
	steps, ok := pages[path]
	if !ok {
		steps = []step{{tap: "h1"}}
	}
	if err := chromedp.Do(ctx, emulate(cfg), chromedp.Navigate(cfg.base+path), chromedp.Sleep(3*time.Second)); err != nil {
		return err
	}
	for _, s := range steps {
		actions := []chromedp.Action[chromedp.Void]{chromedp.WaitVisible(chromedp.CSS(s.tap)), chromedp.Click(chromedp.CSS(s.tap)), chromedp.Sleep(500 * time.Millisecond)}
		if s.typeText != "" {
			actions = append(actions, chromedp.SendKeys(chromedp.CSS(s.tap), s.typeText), chromedp.Sleep(500*time.Millisecond))
		}
		stepCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
		err := chromedp.Do(stepCtx, actions...)
		cancel()
		if err != nil {
			return fmt.Errorf("tap %q: %w (is the element on the page? an empty database has no catalog)", s.tap, err)
		}
	}
	return chromedp.Do(ctx, chromedp.Sleep(time.Second))
}

// measure is one cold run: the page visited, then its vitals.
func measure(cfg config, path string) (sample, error) {
	ctx, done := browser()
	defer done()
	if err := visit(ctx, cfg, path); err != nil {
		return nil, err
	}
	if cfg.debug {
		timeline, err := chromedp.Run(ctx, chromedp.Evaluate[string](timelineJS))
		if err != nil {
			fmt.Fprintln(os.Stderr, "timeline:", err)
		}
		fmt.Fprintln(os.Stderr, timeline)
	}
	raw, err := chromedp.Run(ctx, chromedp.Evaluate[string](`JSON.stringify(window.__lab || {})`))
	if err != nil {
		return nil, err
	}
	var s sample
	if err := json.Unmarshal([]byte(raw), &s); err != nil {
		return nil, err
	}
	if len(s) == 0 {
		return nil, errors.New("no vitals recorded")
	}
	return s, nil
}

// checkBeacon visits path, leaves it for another page, and checks the field
// report vitals.js built as the page was left holds all four metrics. (A
// report sent on pagehide once carried only TTFB: Chrome fires pagehide
// before web-vitals finalises the rest on visibilitychange.)
func checkBeacon(cfg config, path string) (string, error) {
	ctx, done := browser()
	defer done()
	if err := visit(ctx, cfg, path); err != nil {
		return "", err
	}
	leave := "/journals"
	if path == leave {
		leave = "/"
	}
	if err := chromedp.Do(ctx, chromedp.Navigate(cfg.base+leave), chromedp.Sleep(time.Second)); err != nil {
		return "", err
	}
	raw, err := chromedp.Run(ctx, chromedp.Evaluate[string](`localStorage.getItem("lab-beacon") || ""`))
	if err != nil {
		return "", err
	}
	var rep struct {
		Samples []struct {
			Name  string  `json:"name"`
			Value float64 `json:"value"`
		} `json:"samples"`
	}
	if raw == "" || json.Unmarshal([]byte(raw), &rep) != nil {
		return "", errors.New("no field report was built on leaving the page")
	}
	have := map[string]bool{}
	var parts []string
	for _, s := range rep.Samples {
		have[s.Name] = true
		parts = append(parts, fmt.Sprintf("%s %g", s.Name, s.Value))
	}
	for _, m := range []string{"LCP", "INP", "CLS", "TTFB"} {
		if !have[m] {
			return "", fmt.Errorf("the report lacks %s (it has %s)", m, strings.Join(parts, ", "))
		}
	}
	return "report has " + strings.Join(parts, ", "), nil
}

func cell(results []sample, metric string) string {
	var vs []float64
	for _, r := range results {
		if v, ok := r[metric]; ok {
			vs = append(vs, v)
		}
	}
	if len(vs) == 0 {
		return "–"
	}
	sort.Float64s(vs)
	med := median(vs)
	spread := vs[len(vs)-1] - vs[0]
	rating := db.RateVital(metric, med)
	if rating == "" {
		rating = "-"
	}
	f := func(v float64) string {
		if metric == "CLS" {
			return fmt.Sprintf("%.3f", v)
		}
		return fmt.Sprintf("%.0f", v)
	}
	unit := " ms"
	if metric == "CLS" {
		unit = ""
	}
	return fmt.Sprintf("%s%s ± %s (%s) %s", f(med), unit, f(spread), f(vs[len(vs)-1]), rating)
}

func median(sorted []float64) float64 {
	n := len(sorted)
	if n%2 == 1 {
		return sorted[n/2]
	}
	return math.Round((sorted[n/2-1]+sorted[n/2])/2*1000) / 1000
}

func sortedKeys(m map[string][]step) []string {
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	return ks
}

// cleanups run before the process exits early (the -local server).
var cleanups []func()

func exit(code int) {
	for _, c := range cleanups {
		c()
	}
	os.Exit(code)
}

func fail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "lab: "+format+"\n", args...)
	exit(1)
}
