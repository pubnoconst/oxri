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
# from WSL, where the site is built (Principia #9). The lab is a script that
# runs on the Windows host and drives the host's Chrome; mage calls it there.
mage lab                                        # server running signed out; writes bin/lab.md
mage labseeded                                  # the seeded site, started by the lab; the CI budget
node cmd/lab/lab.mjs -runs 10 -pages /journals  # one page, more runs (from the host)
node cmd/lab/lab.mjs -pages /login -debug       # plus each run's paint, resource and font timeline
LAB_BASE=https://example.org mage lab           # any deployment
node cmd/lab/lab.mjs -beacon                    # leaving each page builds a complete field report
```

(In Git Bash on the host, prefix a direct call with `MSYS_NO_PATHCONV=1` so `/journals` is not rewritten into a Windows path.)

**`mage benchmark`: the site with its real data, against Google's lines.**
- It builds the server and starts it signed out on `127.0.0.1:8089` against the database in `db.env`. It only reads, and local views are never recorded as field data.
- It runs 10 cold budget-phone loads per page with `-breakdown`, writes `bin/benchmark.md`, then stops the server.
- `-breakdown` splits each page's LCP the way `web-vitals` attributes it: server (TTFB), the wait before the LCP element's resource is requested, its download, and the render delay. It also shows the LCP element, when the catalog's request ran, when the last font arrived, and the page's kB and requests.
- For a text LCP element there is no resource, so the whole wait to draw it is render delay.

- **Device.** The lab script (`cmd/lab/lab.mjs`) drives Chrome emulating a budget phone, using Lighthouse's mobile defaults:
  - 412×823 CSS px at 1.75 DPR, touch input, an Android Chrome user agent;
  - CPU 4× slower;
  - 150 ms latency, 1.6 Mb/s down, 750 kb/s up.
- **Cold cache.** Each run is a fresh browser with an empty profile. DevTools' "disable cache" is not used, because it also stops preloaded resources being reused.
- **Warm server.** Before measuring, each page is requested once, so the server's own cold start is left to field data.
- **What a run does.** It loads the page, settles for 3 s, then taps (and types where a page has a field) as a reader would, so INP measures real interactions. It reads the vitals with the same `web-vitals` library the field data uses, injected before the page's scripts. The site's `/vitals` beacon is blocked.
- **Report.** Each cell is *median ± spread (worst run)* with the median's rating.
- **Profile signed out.** With `DEV_AUTO_LOGIN` on, every request is the admin's, and the lab would measure the admin view (`/login` redirects away).

**Where the lab runs.** The site is generated in WSL and tested on the host (Principia #9). The lab is a script, `cmd/lab/lab.mjs`: no package and no build step, so nothing is built on the host for the antimalware to judge. It drives Chrome over the DevTools protocol through a pipe, and needs Node 18 or later and a Chrome (`-chrome` to say which). Inside WSL, `mage` runs it through the host's `node.exe`; on any other system, with `node`. `mage typecheck` type-checks it. Until PR #94 it was a Go program (`cmd/lab`, on chromedp); RATIONALE 091 holds how it was ported and how the two compared.

### In CI, on every pull request

The `performance` job in `.github/workflows/ci.yml` runs the lab on GitHub's Ubuntu runner, which has Chrome:

- `node cmd/lab/lab.mjs -local -beacon -parallel 4`: leaving each page must build a field report holding LCP, INP, CLS and TTFB.
- `node cmd/lab/lab.mjs -local -runs 3 -parallel 4 -budget`: no page may shift (CLS ≤ 0.1 on every run; layout is deterministic, so a shift on any run is real), and every run must produce its vitals. Median LCP and INP against Google's lines are reported in the summary but do not fail the build: on GitHub's shared runners they swing by hundreds of milliseconds between runs, and by about a tenth between jobs, so a timing gate would fail pull requests at random. `-timing-gate` adds them on a quiet machine.
- The table goes into the job summary and is kept as the `lab-report` artifact.

**Cost** (owner, 2026-11-25: "these CIs are getting time-expensive"):
- **Parallel pages:** the four pages are measured at once (`-parallel 4`), 3 runs each. Layout shift, the only gate, is the same on every run, and timings are only reported. Measured locally, the job's lab steps dropped from 264 s to 74 s, with the same result. Parallel timings share the CPU, so compare them only between runs with the same setting.
- **Pull requests only:** the job runs on pull requests, not again on `main` after the merge.
- **Superseded runs are cancelled:** a newer push to the same PR cancels the older run (`concurrency`).

`-local` starts `cmd/labserver`, which serves the site from an in-memory database seeded with content like the real thing (`internal/labsite`), and measures that:
- 14 entries quoting stored tweet snapshots and Bible passages, with tags, Markdown and Greek;
- a long Book I.

It needs no credentials, never touches the real database, and calls neither X nor the Bible provider. Readers are signed out. The server prints its address as its first line; the script waits until it answers and ends it with a request when it is done. On Windows the script runs the server in WSL, where the Go toolchain is.

Run the same locally with `mage labseeded`.

**Watch `/journals`.** It is the slowest page on the seeded site (median LCP about 1.2 s, on GitHub's runners with the script and on the owner's machine), with room before the 2.5 s line. Its LCP element is the first entry's excerpt, so its wait is render delay (RATIONALE 076).

**The baseline moved with the script (RATIONALE 091).** On CI's four-core runner the Go lab measured 20–30% higher than the script (within one job, for example 1486 against 1084 ms), in part because its own processing competed with the browsers for the runner's cores. Figures in the results below that were taken on GitHub's runners before PR #94 are on that old baseline; figures from the owner's machine are within run-to-run spread of the script's. CLS did not move: it was the same in every one of 168 runs.

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

### 2026-11-28: the lab is a script (PR #92 and PR #94, RATIONALE 091)

The Go lab (`cmd/lab`, on chromedp) became a Node script (`cmd/lab/lab.mjs`) that drives Chrome over the DevTools pipe, and the seeded site became `cmd/labserver`. Both ran on one CI runner, the seeded site, 3 cold runs × 4 pages at once per pass.

| On the runner, in this order | CLS | Median LCP, all pages | CPU of the lab process |
|---|---|---|---|
| Go lab, then the script (PR #92) | identical | 1290 ms, then 1012 ms | not measured |
| Go, script, script, Go | identical | Go 1486 ms both times; script 1084 ms both times | not measured |
| Go over a port, script, Go over the pipe, script | identical | Go 1540 and 1490 ms; script 1246 and 1076 ms | Go 25.4 and 18.7 s; script 11.7 and 10.8 s |
| Script, script with a CPU-second burned at each page load, the same again, script | identical | script 1214 ms; with the burner 1322 ms | not comparable |

- **CLS was the same in every run of every pass:** 168 runs, `/` 0.000, `/book-i` 0.001, `/journals` 0.008, `/login` 0.000.
- **Run order and the transport were ruled out.** The lab's own CPU use was shown to be part of the cause (a CPU-second taken at each page load costs the page 9%); the rest was not accounted for.
- **On the owner's machine**, 3 runs × 4 pages at once: `/` 1180 ms, `/book-i` 1052 ms, `/journals` 1324 ms, `/login` 956 ms; budget met. The Go lab's figures there (RATIONALE 088) were `/journals` 1464 ms (1100–1512).
- **Against production**, signed out, 2 runs: `/` 1138 ms, `/book-i` 1282 ms, `/journals` 1590 ms (CLS 0.005), `/login` 1284 ms. On a site with passkeys `/login` folds its code field away; the script opens it first.

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

### 2026-11-25: pages arrive whole and still (HIG §1.7)

The lab now also scrolls each page to the end, a screen at a time, and reports the layout shift and the long frames (over 50 ms) during the scroll. `-debug` lists every shift's elements (position and height before and after) and every long frame's split (render, style and layout, blocking, scripts).

`mage benchmark`, 10 cold budget-phone runs, real data:

| `/journals` | Before | After |
|---|---|---|
| CLS | 0.015 on every run | **0.000** on every run |
| Shift while scrolling | 0 | 0 |
| Long frames while scrolling | 1 in 5 of 10 runs (51–68 ms) | 0 in 10 runs |
| LCP (median) | 1608 ms | 1642 ms (within run-to-run spread) |

The other pages were unchanged: CLS 0 (0.001 on `/book-i`), with no long frames while scrolling.

**The 0.015 was not media.** `-debug` named a tweet clipping's signature line (`span.tweet-author`). In the size-matched fallback a name and handle wrapped to two lines (33 px), and they fitted on one once Short Stack arrived at about 1.9 s (17 px), moving the card below up 10 px. The line is now one line by design: no wrap, with the handle and then the name cut by an ellipsis.

**Changed with it:**
- **Media slots get their space from their kind** (`views/journals/mediakind.go`, which mirrors `journal-media.js`): video 16:9, audio the player's height, images a fixed frame with the image fitted inside. A plain link's card is drawn by the server.
- **Page dimming is removed.** The tapped control turns pending at once instead, with the dots after 180 ms.
- **The catalog's search, tags and pages swap the results in place.**

**What remains:**
- **One heavy frame at load (65–148 ms, nearly all style and layout)** as the web fonts replace the fallbacks. Nothing moves, but the page is laid out again. Only `font-display: optional` avoids it, at the cost of first visits keeping the fallback fonts.
- **On the seeded site**, a quote's body text can still wrap one line differently once Short Stack arrives (CLS 0.008), for the same reason.

### 2026-11-25: compression, Server-Timing, render-blocking, the cache spike

Seeded lab, 5 cold budget-phone runs, after each step:

| LCP (median) | `/` | `/book-i` | `/journals` | `/login` |
|---|---|---|---|---|
| Before | 1224 | 1320 | 1564 | 1160 |
| + gzip (as production already sends) | 900 | 996 | 1100 | 856 |
| + `dot-matrix.js` deferred | 884 | 1012 | 1068 | 816 |

What each step changed:
- **Compression** halved the bytes (e.g. `/journals` 322 → 193 kB). Production already had it through Cloudflare, so mainly the lab now matches what readers receive, and a laptop host without Cloudflare will need it.
- **Deferring `dot-matrix.js`** gained 16 to 40 ms, near the noise.
- **Scripts at `fetchpriority="low"`** were tried, measured no better, and were reverted.
- **Inlining `fonts.css`** was not done: it already arrives before `site.css`, so it is not on the critical path.

**`Server-Timing` named the server spike.** Pages now carry `app`, `db` and `tweets` durations, which the lab reads (`-breakdown`, and per run). Warm, the server takes 0.3 to 2 ms. The occasional ~0.8–1.1 s first byte was the server itself: one run showed `server 1147.3`. The entry and book caches expired every 2 minutes, and the next reader waited for Turso. They are now **stale-while-revalidate**: a stale copy is served at once and one background refresh updates it. Across a cache expiry (15 runs), the worst server time was 2.2 ms.

`mage benchmark`, 10 runs, real data:

| | Before (this morning) | After |
|---|---|---|
| `/` LCP | 1208 ms | 898 ms |
| `/book-i` LCP | 1310 ms | 954 ms |
| `/journals` LCP | 1642 ms | 1180 ms |
| `/login` LCP | 1188 ms | 840 ms |
| CLS, every page | 0 | 0 |

The real-data run still had one TTFB spike per page, from the cache expiry, before stale-while-revalidate was added.

**What is left of the render delay** (`/`): `site.css` arrives last at about 530 ms; first paint follows at about 600 ms. Then come the page fade-in (220 ms then; 100 ms since PR #79) and about 100 ms in which `font-display: optional` holds text invisible while it decides whether the web font is ready (the price of never swapping).

### 2026-11-26: catalog page size, image sizes, tap-to-play

The seeded lab gained `-seed N` (entries; every tweet clipping carries a 1200×900 photo) and `-page-size`. X's image requests are redirected to the lab's own server, still over the emulated network, one URL per image: a shared URL let the cache serve 11 of 12 images, and would have hidden the very cost being measured. The lab counts the image bytes it serves, since browsers report cross-origin sizes as 0.

`/journals`, 160 seeded entries, 3 cold runs per size (today's cards, full-size images):

| Cards per page | 12 | 24 | 48 | 96 | 150 |
|---|---|---|---|---|---|
| LCP (median) | 1104 | 1104 | 1136 | 1172 | 1256 ms |

No layout shift and no long frames while scrolling at any size. Images load lazily near the screen, and the cards are server-drawn HTML. The page holds **24** (it was 12 while every tweet was X's iframe; 24 fills 1, 2, 3 or 4 columns evenly).

**Clipping images** on the 24-card page: full size 206 kB each, about 2.5 MB per page view; X's small size **68 kB** each, **about 0.8 MB**. LCP and scrolling are unchanged, since the main content is text.

**Tap-to-play:** a YouTube or Vimeo player, a GIF or a video file loads nothing heavy until tapped. YouTube shows only its thumbnail, a small image.

**Server stills** (internal/thumb, table `media_thumbs`, schema v10): the server fetches a linked image, GIF, or YouTube or Vimeo poster once in the background, keeps it as a JPEG at most 680 px wide, and serves it from `/thumb/{key}.jpg` (cached for good). The fetch reaches public addresses only, checked on the address actually dialled, and downloads and decoded size are capped. On the real catalog (24 cards):

| | LCP | CLS | Note |
|---|---|---|---|
| Still made by the script | 1.65 s | 0.039 | the still waited 780 ms for the script before it was even requested; a lazily loaded still grew in place while scrolling |
| Still sized to its slot | 1.65 s | **0** | container query units: the largest box of its shape the slot holds, before it loads |
| Tile drawn by the server | **1.26 s** | **0** | the still is in the HTML and requested at 51 ms; the script only starts the player on tap |

The lab's shift report now names an image's source and its parent, which found this.

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
