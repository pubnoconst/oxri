# Profiling

How the site's performance is measured. The yardstick is Google's **Core Web Vitals**, judged at the **75th percentile of page views**, mobile and desktop separately:

| Metric | Measures | Good | Needs improvement | Poor |
|---|---|---|---|---|
| **LCP** Largest Contentful Paint | loading: when the main content is drawn | ≤ 2.5 s | ≤ 4.0 s | > 4.0 s |
| **INP** Interaction to Next Paint | responsiveness: the slowest tap or key to its next frame | ≤ 200 ms | ≤ 500 ms | > 500 ms |
| **CLS** Cumulative Layout Shift | visual stability: how much content moves after it is drawn | ≤ 0.1 | ≤ 0.25 | > 0.25 |

TTFB (≤ 800 ms) is recorded too, because the server's cold starts are the largest variance the site has (NEXT.md).

HIG §1.6 adds one rule on top: **stable beats peak.** Read the spread and the worst run before the median.

There are two kinds of evidence, and only the first is the real thing:

1. **Field data**: what readers' own browsers measured. This is what Google's assessment is based on.
2. **Lab runs**: a repeatable, scripted budget-phone profile, for comparing two builds before shipping.

## 1. Field data (readers' browsers)

- **What collects it.** Every page loads Google's `web-vitals` library (`assets/js/web-vitals.min.js`, the 6.2.3 attribution build, vendored with its licence) and `assets/js/vitals.js`. As the reader leaves the page, `vitals.js` sends one beacon to `POST /vitals` with LCP, INP, CLS and TTFB.
- **What is kept** (`routes/vitals.go`, table `vitals`):
  - the route template (`/journals/:id`, not the id);
  - the metric and its value;
  - the server's own rating by the thresholds above (the browser's rating is not trusted);
  - mobile or desktop (coarse pointer);
  - the effective connection type and the device memory class;
  - the CSS selector of the element the browser blamed.
- **What is not kept:** no address, user agent, cookie or identifier.
- **What is not counted:**
  - views while signed in (the admin is not a reader);
  - anything served on a loopback host (the local test server and lab runs);
  - malformed or out-of-range samples.
- **Limits:** the endpoint is rate-limited per address, a report is capped at 4 KB, and rows older than 28 days or past 20,000 are pruned. Field rows do not count toward the writing quota.
- **Where to read it:** **Settings → Field Performance**.
  - The p75 of each metric over the last 28 days, mobile first, coloured good / needs improvement / poor.
  - The number of views behind each figure and the share that were good.
  - The elements behind the poor views.
- **Caveats:**
  - Low traffic gives noisy percentiles: read the view count beside each figure.
  - Browsers without the newer APIs (Safari has no INP) report fewer metrics.

## 2. Lab runs (repeatable budget-phone profile)

```sh
# with the server running, signed OUT (no DEV_AUTO_LOGIN), against real data:
mage lab                                        # writes bin/lab.md
go run ./cmd/lab -runs 10 -pages /journals      # one page, more runs
go run ./cmd/lab -pages /login -debug           # plus each run's paint, resource and font timeline
LAB_BASE=https://example.org mage lab           # any deployment
go run ./cmd/lab -beacon                        # leaving each page builds a complete field report
```

(In Git Bash, prefix with `MSYS_NO_PATHCONV=1` so `/journals` is not rewritten into a Windows path.)

**`mage benchmark`: the site with its real data, against Google's lines.**
- It builds the server and starts it signed out on `127.0.0.1:8089` against the database in `db.env`. It only reads, and local views are never recorded as field data.
- It runs 10 cold budget-phone loads per page with `-breakdown`, writes `bin/benchmark.md`, then stops the server.
- `-breakdown` splits each page's LCP the way `web-vitals` attributes it: server (TTFB), the wait before the LCP element's resource is requested, its download, and the render delay. It also shows the LCP element, when the catalog's request ran, when the last font arrived, and the page's kB and requests.
- For a text LCP element there is no resource, so the whole wait to draw it is render delay.

- **Device.** `cmd/lab` drives headless Chrome emulating a budget phone, using Lighthouse's mobile defaults:
  - 412×823 CSS px at 1.75 DPR, touch input, an Android Chrome user agent;
  - CPU 4× slower;
  - 150 ms latency, 1.6 Mb/s down, 750 kb/s up.
- **Cold cache.** Each run is a fresh browser with an empty profile. DevTools' "disable cache" is not used, because it also stops preloaded resources being reused.
- **Warm server.** Before measuring, each page is requested once, so the server's own cold start is left to field data.
- **What a run does.** It loads the page, settles for 3 s, then taps (and types where a page has a field) as a reader would, so INP measures real interactions. It reads the vitals with the same `web-vitals` library the field data uses, injected before the page's scripts. The site's `/vitals` beacon is blocked.
- **Report.** Each cell is *median ± spread (worst run)* with the median's rating.
- **Profile signed out.** With `DEV_AUTO_LOGIN` on, every request is the admin's, and the lab would measure the admin view (`/login` redirects away).

### In CI, on every pull request

The `performance` job in `.github/workflows/ci.yml` runs the lab on GitHub's Ubuntu runner, which has Chrome:

- `go run ./cmd/lab -local -beacon`: leaving each page must build a field report holding LCP, INP, CLS and TTFB.
- `go run ./cmd/lab -local -runs 5 -budget`: no page may shift (CLS ≤ 0.1 on every run; layout is deterministic, so a shift on any run is real), and every run must produce its vitals. Median LCP and INP against Google's lines are reported in the summary but do not fail the build: on GitHub's shared runners they swing by hundreds of milliseconds between runs (one run saw 1.1 s TTFB spikes), so a timing gate would fail pull requests at random. `-timing-gate` adds them on a quiet machine.
- The table goes into the job summary and is kept as the `lab-report` artifact.

`-local` serves the site inside the lab process from an in-memory database seeded with content like the real thing (`cmd/lab/local.go`):
- 14 entries quoting stored tweet snapshots and Bible passages, with tags, Markdown and Greek;
- a long Book I.

It needs no credentials, never touches the real database, and calls neither X nor the Bible provider. Readers are signed out.

Run the same locally with `go run ./cmd/lab -local -budget`.

**Watch `/journals`.** On the seeded site its median LCP is about 2.36 s, the closest page to the 2.5 s line, because the catalog arrives in a second request after the page shell. Rendering the first page of entries into `/journals` itself (NEXT.md) is the fix that gives it room.

### Lab is not the phone

Emulation slows the CPU and network, but not the GPU, and a desktop window that is not in front has its frames throttled. For frame stability, use the phone:

1. On the phone, turn on Developer options (Settings → About phone → Software information → tap *Build number* seven times), then *USB debugging*.
2. Connect it by USB, and open `chrome://inspect/#devices` on the PC. Allow the prompt on the phone, then click *inspect* under the site's tab.
3. Open the **Performance** panel, record, use the page, and stop. Read:
   - **Frames**: even and green; red or yellow frames are the variance §1.6 is about.
   - **Layout shifts**: purple markers.
   - **Main**: long tasks and what caused them.
4. To see the A02s on the S9+: Display → Screen resolution → HD+, and Developer options → *Profile GPU rendering* → bars on screen.

## Results

### 2026-11-25: self-hosted fonts (PR #74) against Google Fonts (`main`, `c4397e8`)

Lab, 5 cold runs per page (10 for `/journals`), real data, signed out, desktop Chrome 158 emulating the budget phone:

| Page | Google Fonts LCP | Self-hosted LCP | Google Fonts CLS, worst run | Self-hosted CLS, worst run |
|---|---|---|---|---|
| `/` | 1168 ms ± 160 | 1180 ms ± 44 | 0.000 | 0.000 |
| `/book-i` | 1540 ms ± 764 | **1284 ms ± 80** | **0.166** (needs improvement) | **0.000** |
| `/journals` (10 runs) | 2034 ms ± 196 | 2032 ms ± 264 | 0.006 | 0.000 |
| `/login` | 1376 ms ± 360 | **1120 ms ± 76** | 0.000 | 0.000 |

INP stays under 50 ms on every page in both builds; TTFB is 152 ms (the emulated latency).

All medians are "good" in both builds. Self-hosting removed the layout shift (a cold `/book-i` load reflowed by 0.166 as EB Garamond replaced Georgia), and it narrowed the spread on three of the four pages.

**Four regressions the lab and CI caught before shipping:**

1. **A preloaded font held text back.** With Figtree's Latin file preloaded, `/login` drew no text until about 4.6 s in every run, although the font had arrived by about 0.7 s. LCP was never recorded, because the scripted tap at 3 s came first. Without the preload, first text came at 1.12 s. The preload was removed.
2. **Fallback faces without `font-display`.** The size-matched local fallbacks defaulted to `font-display: auto`, which in Chrome hides text while the face resolves. They now use `swap`, and `TestFontFacesAreRegularAndHashed` requires `swap` on every face.

3. **Field reports would have carried TTFB only.** `vitals.js` first sent its beacon on `pagehide` as well as on hidden. When a link is followed, Chrome fires `pagehide` before `visibilitychange`, and `web-vitals` finalises LCP, CLS and INP only on the latter, so every report left with TTFB alone. It now sends on `visibilitychange` to hidden only, as the library recommends. `go run ./cmd/lab -beacon` checks that leaving each page builds a report with all four metrics (`/`, `/book-i` and `/login` pass; `/journals` needs a database with entries for its scripted search).

4. **Fallbacks tuned to one platform's fonts.** The first CI run (Ubuntu, no Georgia) measured CLS 0.194 on every `/book-i` run: the book's fallback was tuned to Georgia only, so Linux fell through to its wider default serif and reflowed when EB Garamond arrived. Android, the reference reader's platform, has neither Georgia nor Arial either. Each role now has one measured fallback per system font family (Georgia, Times New Roman / Liberation Serif, Noto Serif; Arial / Helvetica / Liberation Sans, Roboto).

**Server variance** (field data will show it): the first `/journals/content` after the server starts took 788 ms; warm, 17 ms.

### 2026-11-25: `/journals` renders its catalog in the page (PR #75)

`mage benchmark`, 10 cold budget-phone runs, real data, signed out.

**Before:** the page sent an empty frame, then htmx requested `/journals/content` once the frame and its scripts had run. The LCP element is the first entry's excerpt (text), so all of its wait was render delay:

| | Before | After |
|---|---|---|
| `/journals` LCP, median ± spread (worst) | 2274 ± 800 ms (2972) | **1498 ± 872 ms** (2320) |
| Render delay (median) | 2122 ms | 1326 ms |
| Catalog request | 1382 → 1765 ms | none |
| Last font | 2961 ms | 2335 ms |
| Weight | 321 kB / 22 requests | 320 kB / 21 |

**Where the 0.9 s went before:**
- The catalog was requested only at about 1.38 s, once the frame was drawn and htmx had run.
- It arrived about 0.38 s later.
- Then a deliberate 120 ms swap pause and the layout of the cards followed.

**Now** the catalog comes with the page, the dot loader is gone, and the catalog also works without JavaScript.

The remaining spread is one run whose TTFB was 993 ms, a server-side spike, which is field data's to watch. The other pages are about 1.2 to 1.3 s, with about 1 s on every page between first byte and first text (render-blocking CSS and the head script under a 4× slower CPU, plus the kept 220 ms fade-in).

**Is the font the bottleneck? No.** The excerpt is drawn in the size-matched fallback; Playpen Sans arrives later and swaps in without a shift. Trimming Playpen (80 kB) would save bandwidth and bring the real face in sooner, but would not move LCP.

## In any Chrome tab, without tooling

Paste into the console after the page has loaded:

```js
const take = (type) => new Promise((res) => { const l = []; const o = new PerformanceObserver((x) => l.push(...x.getEntries())); o.observe({ type, buffered: true }); setTimeout(() => { o.disconnect(); res(l); }, 50); });
const lcp = await take("largest-contentful-paint");
const shifts = (await take("layout-shift")).filter((s) => !s.hadRecentInput);
const frames = await take("long-animation-frame");
console.table({
  lcp: lcp.length ? Math.round(lcp.at(-1).startTime) : null,
  cls: shifts.reduce((a, s) => a + s.value, 0).toFixed(4),
  longFrames: frames.length,
  worstFrameMs: frames.length ? Math.round(Math.max(...frames.map((f) => f.duration))) : 0,
});
```
