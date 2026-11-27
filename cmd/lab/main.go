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
	"net/url"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/cdproto/emulation"
	"github.com/chromedp/cdproto/fetch"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"

	"github.com/oxri/oxri-go/internal/db"
	"github.com/oxri/oxri-go/internal/labsite"
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
  // Every layout shift and long frame (over 50 ms), for the scroll phase.
  window.__labShifts = []; window.__labFrames = [];
  try {
    window.__labShiftSources = [];
    const describe = (n) => n && n.nodeType === 1 ? n.tagName.toLowerCase() + (n.className && typeof n.className === "string" ? "." + n.className.trim().split(/\s+/).join(".") : "") + (n.tagName === "IMG" ? " [" + (n.getAttribute("src") || "").slice(0, 60) + " in " + ((n.parentElement && n.parentElement.className) || "") + "]" : "") : String(n && n.nodeName);
    new PerformanceObserver((l) => { for (const e of l.getEntries()) if (!e.hadRecentInput) {
      window.__labShifts.push([e.startTime, e.value]);
      for (const s of e.sources || []) window.__labShiftSources.push(Math.round(e.startTime) + "ms " + e.value.toFixed(4) + " " + describe(s.node) + " y " + Math.round(s.previousRect.y) + "→" + Math.round(s.currentRect.y) + " h " + Math.round(s.previousRect.height) + "→" + Math.round(s.currentRect.height));
    } }).observe({ type: "layout-shift", buffered: true });
    window.__labFrameSources = [];
    new PerformanceObserver((l) => { for (const e of l.getEntries()) {
      window.__labFrames.push([e.startTime, e.duration]);
      const style = e.styleAndLayoutStart ? Math.round(e.startTime + e.duration - e.styleAndLayoutStart) : 0;
      const scripts = (e.scripts || []).map((x) => (x.invoker || x.sourceFunctionName || "?") + " " + Math.round(x.duration) + "ms").join("; ");
      window.__labFrameSources.push(Math.round(e.startTime) + "ms " + Math.round(e.duration) + "ms (render " + (e.renderStart ? Math.round(e.startTime + e.duration - e.renderStart) : 0) + ", style+layout " + style + ", blocking " + Math.round(e.blockingDuration) + ") " + scripts);
    } }).observe({ type: "long-animation-frame", buffered: true });
  } catch (e) {}
  const keep = (m) => {
    window.__lab[m.name] = m.value;
    // LCP in its four parts (web-vitals attribution), and the element.
    if (m.name === "LCP" && m.attribution) {
      const a = m.attribution;
      window.__lab.lcpTTFB = a.timeToFirstByte;
      window.__lab.lcpLoadDelay = a.resourceLoadDelay;
      window.__lab.lcpLoadTime = a.resourceLoadDuration;
      window.__lab.lcpRenderDelay = a.elementRenderDelay;
      window.__labTarget = a.target || "";
    }
  };
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
  [...document.fonts].filter((f) => f.status !== "unloaded").map((f) => f.family + " " + f.unicodeRange.slice(0, 8) + " " + f.status).join(", ") +
  String.fromCharCode(10) + "shifts: " + (window.__labShiftSources || []).join(String.fromCharCode(10) + "  ") +
  String.fromCharCode(10) + "long frames: " + (window.__labFrameSources || []).join(String.fromCharCode(10) + "  ")`

type config struct {
	local    bool // the in-process site: its X images are served by the lab
	base     string
	vitalsJS string
	cpu      float64
	debug    bool
}

func main() {
	base := flag.String("base", "http://127.0.0.1:8080", "site to measure")
	runs := flag.Int("runs", 5, "cold-cache runs per page")
	parallel := flag.Int("parallel", 1, "pages measured at once, each in its own browser (faster; timings then share the CPU, so compare them only between runs with the same setting: CI uses it, as it gates only on layout shift)")
	only := flag.String("pages", "", "comma-separated paths (default: all of "+strings.Join(sortedKeys(pages), ", ")+")")
	cpu := flag.Float64("cpu", 4, "CPU slowdown factor")
	lib := flag.String("lib", "assets/js/web-vitals.min.js", "the vendored web-vitals build")
	out := flag.String("out", "", "also write the report (Markdown) to this file")
	debug := flag.Bool("debug", false, "print each run's paint, resource and font timeline")
	warm := flag.Bool("warm", true, "request each page once first, so the server's own cold start is not counted against the page (field data covers the server)")
	seedN := flag.Int("seed", 14, "with -local: how many entries to seed (every other one a tweet clipping with a photo)")
	pageSize := flag.Int("page-size", 0, "with -local: entries per catalog page, to compare sizes (0 = the site's own)")
	local := flag.Bool("local", false, "serve the site in this process from an in-memory database with seeded content, and measure that (no credentials needed; CI uses it)")
	budget := flag.Bool("budget", false, "exit 1 if a page shifts (any run's CLS > 0.1) or a run fails; median LCP > 2.5 s and INP > 200 ms are reported, not failed (a shared runner's timing varies)")
	breakdown := flag.Bool("breakdown", false, "also report each page's LCP split into its parts (server, resource wait, resource load, render) and its network shape")
	timingGate := flag.Bool("timing-gate", false, "with -budget, also exit 1 on median LCP > 2.5 s or INP > 200 ms (for a quiet machine, not CI)")
	beacon := flag.Bool("beacon", false, "check instead that leaving each page builds a field report with LCP, INP, CLS and TTFB (vitals.js); exits 1 if not")
	flag.Parse()

	vitalsJS, err := os.ReadFile(*lib)
	if err != nil {
		fail("read web-vitals (run from the repository root): %v", err)
	}
	if *local {
		url, stop, err := startLocal(context.Background(), *seedN, *pageSize)
		if err != nil {
			fail("local server: %v", err)
		}
		cleanups = append(cleanups, stop)
		defer stop()
		*base = url
		fmt.Fprintln(os.Stderr, "lab: serving the seeded site at", url)
	}
	cfg := config{base: *base, vitalsJS: string(vitalsJS), cpu: *cpu, debug: *debug, local: *local}
	paths := sortedKeys(pages)
	if *only != "" {
		paths = strings.Split(*only, ",")
	}

	if *beacon {
		results := make([]string, len(paths))
		errs := make([]error, len(paths))
		eachPage(paths, *parallel, func(i int, p string) { results[i], errs[i] = checkBeacon(cfg, p) })
		failed := false
		for i, p := range paths {
			if errs[i] != nil {
				failed = true
				fmt.Printf("%s: %v\n", p, errs[i])
				continue
			}
			fmt.Printf("%s: %s\n", p, results[i])
		}
		if failed {
			exit(1)
		}
		return
	}

	if *warm {
		for _, p := range paths {
			if res, err := http.Get(*base + p); err == nil {
				_, _ = io.Copy(io.Discard, res.Body)
				_ = res.Body.Close()
			}
		}
	}

	var report strings.Builder
	fmt.Fprintf(&report, "# Lab run %s\n\n", time.Now().Format("2006-01-02 15:04"))
	fmt.Fprintf(&report, "`%s`, %d cold-cache runs per page; budget phone: 412×823 @1.75, CPU %g× slower, 150 ms RTT, 1.6 Mb/s down. Median / spread (max − min) / worst. Good: LCP ≤ 2.5 s, INP ≤ 200 ms, CLS ≤ 0.1.\n\n", *base, *runs, *cpu)
	fmt.Fprintf(&report, "| Page | LCP | INP | CLS | TTFB | FCP | Shift while scrolling | Long frames while scrolling | Worst frame |\n|---|---|---|---|---|---|---|---|---|\n")
	var misses, advisories []string
	all := map[string][]sample{}
	perPage := make([][]sample, len(paths))
	eachPage(paths, *parallel, func(n int, p string) {
		for i := 0; i < *runs; i++ {
			s, err := measure(cfg, p)
			if err != nil {
				fmt.Fprintf(os.Stderr, "%s run %d: %v\n", p, i+1, err)
				continue
			}
			perPage[n] = append(perPage[n], s)
			fmt.Fprintf(os.Stderr, "%s run %d: LCP %.0f  INP %.0f  CLS %.3f  TTFB %.0f (server %.1f)  scroll: shift %.3f, %0.f long frames, worst %.0f ms\n", p, i+1, s["LCP"], s["INP"], s["CLS"], s["TTFB"], s["st_app"], s["scrollShift"], s["scrollFrames"], s["scrollWorstFrame"])
		}
	})
	for n, p := range paths {
		results := perPage[n]
		fmt.Fprintf(&report, "| `%s` | %s | %s | %s | %s | %s | %s | %s | %s |\n", p,
			cell(results, "LCP"), cell(results, "INP"), cell(results, "CLS"), cell(results, "TTFB"), cell(results, "FCP"),
			plain(results, "scrollShift", "%.3f"), plain(results, "scrollFrames", "%.0f"), plain(results, "scrollWorstFrame", "%.0f ms"))
		all[p] = results
		strict, timing := checkBudget(p, results, *runs)
		misses = append(misses, strict...)
		if *timingGate {
			misses = append(misses, timing...)
		} else {
			advisories = append(advisories, timing...)
		}
	}
	fmt.Fprintf(&report, "\nA cell reads *median ± spread (worst)* and the median's rating. `–` is a metric the page never produced (no interaction, or no shift).\n")

	if *breakdown {
		report.WriteString("\n### Where the LCP time goes (medians, ms)\n\n")
		report.WriteString("LCP = server (TTFB) + wait before the LCP element's resource is requested + its download + render delay (web-vitals attribution). For a text element there is no resource: the wait for the text to be drawn (its data, fonts, scripts) is all render delay.\n\n")
		report.WriteString("| Page | LCP | LCP element | TTFB | Server (app / db / tweets) | Resource wait | Resource load | Render delay | Catalog request (start → end) | Last font | kB / requests |\n|---|---|---|---|---|---|---|---|---|---|---|\n")
		for _, p := range paths {
			report.WriteString(breakdownRow(p, all[p]))
		}
	}

	if *budget {
		if len(misses) == 0 {
			report.WriteString("\n**Budget: met**: no page shifts (every run's CLS ≤ 0.1) and every run produced its vitals.\n")
		} else {
			report.WriteString("\n**Budget: missed**\n\n")
			for _, m := range misses {
				report.WriteString("- " + m + "\n")
			}
		}
		if len(advisories) > 0 {
			report.WriteString("\n**Over Google's good line on this machine** (reported, not failed: timing varies with the runner):\n\n")
			for _, m := range advisories {
				report.WriteString("- " + m + "\n")
			}
		}
	}

	if *local {
		if count, bytes := localSite.Images(); count > 0 {
			fmt.Fprintf(&report, "\nImages served by the lab (as from X): %d, %.0f kB in all, %.0f kB each on average (over all runs).\n",
				count, float64(bytes)/1024, float64(bytes)/1024/float64(count))
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

// checkBudget holds a page to Google's good lines. strict: CLS on every run
// (layout is deterministic, so a shift on any run is real) and every run
// produced its vitals. timing: median LCP and INP, which on a shared CI
// runner swing by hundreds of milliseconds between runs, so they fail only
// with -timing-gate.
func checkBudget(page string, results []sample, runs int) (strict, timing []string) {
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
		timing = append(timing, fmt.Sprintf("`%s`: median LCP %.0f ms > 2500 ms", page, m))
	}
	if vs := values("CLS"); len(vs) > 0 && vs[len(vs)-1] > db.Thresholds["CLS"][0] {
		misses = append(misses, fmt.Sprintf("`%s`: a run's CLS %.3f > 0.1", page, vs[len(vs)-1]))
	}
	if vs := values("INP"); len(vs) > 0 && median(vs) > db.Thresholds["INP"][0] {
		timing = append(timing, fmt.Sprintf("`%s`: median INP %.0f ms > 200 ms", page, median(vs)))
	}
	return misses, timing
}

// browser starts a fresh headless Chrome with its own empty profile, so an
// empty cache as on a first visit. DevTools' "disable cache" is not used: it
// also stops a preloaded resource being reused, which is not a first visit.
func browser() (context.Context, func()) {
	opts := append(chromedp.DefaultExecAllocatorOptions[:], chromedp.Flag("disable-extensions", true))
	if os.Getenv("LAB_PIPE") != "" {
		// Throwaway comparison with the Node lab: the DevTools pipe, not a port.
		opts = append(opts, chromedp.Flag("remote-debugging-pipe", true))
	}
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
		if cfg.local {
			// X's images go to the lab's server instead, still over the
			// emulated network (the request is redirected, not answered here).
			_ = chromedp.Listen(ctx, func(ev any) {
				if e, ok := ev.(*fetch.EventRequestPaused); ok {
					// One URL per image (its X path), so each is fetched as on
					// X: a shared URL would let the cache serve them all after
					// the first.
					name := "large"
					if strings.Contains(e.Request.URL, "name=small") {
						name = "small"
					}
					dest := cfg.base + labsite.ImagePath + "?name=" + name + "&src=" + url.QueryEscape(e.Request.URL)
					go func() {
						_, _ = chromedp.Call(ctx, fetch.ContinueRequest, fetch.ContinueRequestParams{RequestID: e.RequestID, URL: dest})
					}()
				}
			})
			try(func() error {
				_, e := chromedp.Call(ctx, fetch.Enable, fetch.EnableParams{Patterns: []*fetch.RequestPattern{{URLPattern: "https://pbs.twimg.com/*"}}})
				return e
			})
		}
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
	if err := chromedp.Do(ctx, chromedp.Sleep(time.Second)); err != nil {
		return err
	}
	// Then read to the end as a reader would: a screen at a time, with a
	// pause to look, so lazy media loads and lays out as it comes into view.
	// What moves and which frames run long during this is the stability a
	// load-time metric misses.
	_, err := chromedp.Run(ctx, chromedp.Evaluate[bool](scrollJS, chromedp.EvalAwaitPromise))
	return err
}

// scrollJS scrolls to the bottom in steps of 80 % of the screen every
// 300 ms (at most 40 steps), waits for the page to settle, and records in
// window.__lab: scrollShift (layout shift during the scroll), scrollFrames
// (frames over 50 ms) and scrollWorstFrame (ms).
const scrollJS = `(async () => {
  const wait = (ms) => new Promise((r) => setTimeout(r, ms));
  const start = performance.now();
  for (let i = 0; i < 40; i++) {
    const before = scrollY;
    scrollBy(0, Math.round(innerHeight * 0.8));
    await wait(300);
    if (scrollY === before) break;
  }
  await wait(1200);
  const shifts = (window.__labShifts || []).filter((e) => e[0] >= start);
  const frames = (window.__labFrames || []).filter((e) => e[0] >= start);
  window.__lab.scrollShift = shifts.reduce((a, e) => a + e[1], 0);
  window.__lab.scrollFrames = frames.length;
  window.__lab.scrollWorstFrame = frames.reduce((a, e) => Math.max(a, e[1]), 0);
  return true;
})()`

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
	raw, err := chromedp.Run(ctx, chromedp.Evaluate[string](`JSON.stringify(Object.assign({}, window.__lab || {}, `+resourcesJS+`))`))
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
	if target, err := chromedp.Run(ctx, chromedp.Evaluate[string](`String(window.__labTarget || "")`)); err == nil {
		lcpTargetsMu.Lock()
		lcpTargets[path] = append(lcpTargets[path], target)
		lcpTargetsMu.Unlock()
	}
	return s, nil
}

// resourcesJS summarises the page's requests: when the catalog fragment
// (/journals/content) was requested and arrived, when the last font
// arrived, and the bytes the page transferred (kB).
const resourcesJS = `(() => {
  const rs = performance.getEntriesByType("resource");
  const out = {};
  const cat = rs.find((r) => r.name.includes("/journals/content"));
  if (cat) { out.catalogStart = cat.startTime; out.catalogEnd = cat.responseEnd; }
  const fonts = rs.filter((r) => r.name.includes(".woff2"));
  if (fonts.length) out.fontsEnd = Math.max(...fonts.map((r) => r.responseEnd));
  const nav = performance.getEntriesByType("navigation")[0];
  out.kB = (rs.reduce((a, r) => a + (r.transferSize || 0), 0) + (nav ? nav.transferSize : 0)) / 1024;
  out.requests = rs.length + 1;
  // Where the server's time went (its Server-Timing header).
  if (nav && nav.serverTiming) for (const t of nav.serverTiming) out["st_" + t.name] = t.duration;
  return out;
})()`

// lcpTargets are the LCP elements seen per page, for the breakdown
// (written by runs that may be in parallel).
var (
	lcpTargets   = map[string][]string{}
	lcpTargetsMu sync.Mutex
)

// breakdownRow is a page's LCP split into its parts and the page's network
// shape, as medians (-breakdown).
func breakdownRow(page string, results []sample) string {
	med := func(k string) string {
		var vs []float64
		for _, r := range results {
			if v, ok := r[k]; ok {
				vs = append(vs, v)
			}
		}
		if len(vs) == 0 {
			return "–"
		}
		sort.Float64s(vs)
		return fmt.Sprintf("%.0f", median(vs))
	}
	counts := map[string]int{}
	top := ""
	for _, t := range lcpTargets[page] {
		counts[t]++
		if counts[t] > counts[top] {
			top = t
		}
	}
	if top == "" {
		top = "–"
	} else if i := strings.LastIndex(top, ">"); i >= 0 {
		top = "…" + top[i:]
	}
	catalog := "–"
	if s, e := med("catalogStart"), med("catalogEnd"); s != "–" {
		catalog = s + " → " + e
	}
	// The server's own parts are fractions of a millisecond when warm.
	fine := func(k string) string {
		var vs []float64
		for _, r := range results {
			if v, ok := r[k]; ok {
				vs = append(vs, v)
			}
		}
		if len(vs) == 0 {
			return "–"
		}
		sort.Float64s(vs)
		return fmt.Sprintf("%.1f", median(vs))
	}
	server := fine("st_app") + " / " + fine("st_db") + " / " + fine("st_tweets")
	return fmt.Sprintf("| `%s` | %s | `%s` | %s | %s | %s | %s | %s | %s | %s | %s / %s |\n", page, med("LCP"), top,
		med("lcpTTFB"), server, med("lcpLoadDelay"), med("lcpLoadTime"), med("lcpRenderDelay"), catalog, med("fontsEnd"), med("kB"), med("requests"))
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
			Name   string  `json:"name"`
			Value  float64 `json:"value"`
			Target string  `json:"target"`
		} `json:"samples"`
	}
	if raw == "" || json.Unmarshal([]byte(raw), &rep) != nil {
		return "", errors.New("no field report was built on leaving the page")
	}
	have := map[string]bool{}
	var parts []string
	for _, s := range rep.Samples {
		have[s.Name] = true
		part := fmt.Sprintf("%s %g", s.Name, s.Value)
		if s.Target != "" && s.Value > 0 {
			part += " (" + s.Target + ")"
		}
		parts = append(parts, part)
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

// plain is a metric's median ± spread (worst), with no rating.
func plain(results []sample, metric, format string) string {
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
	f := func(v float64) string { return fmt.Sprintf(format, v) }
	return f(median(vs)) + " ± " + strings.TrimSuffix(f(vs[len(vs)-1]-vs[0]), " ms") + " (" + f(vs[len(vs)-1]) + ")"
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

// eachPage runs do for every page, at most parallel at a time, and returns
// when all are done.
func eachPage(paths []string, parallel int, do func(i int, path string)) {
	if parallel < 1 {
		parallel = 1
	}
	slots := make(chan struct{}, parallel)
	var wg sync.WaitGroup
	for i, p := range paths {
		wg.Add(1)
		slots <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-slots }()
			do(i, p)
		}()
	}
	wg.Wait()
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
