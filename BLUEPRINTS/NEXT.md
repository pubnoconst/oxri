# Next

- **Base Commit on `main`**: `cb94539` (PR #84: a paragraph break is one newline in the card preview)
- **Active Branch**: `docs/journal-cards-and-newspaper-hig` (PR #85): universal preview cards and newspaper entry layout recorded in HIG (RATIONALE 086, `270fcf5`).
- **Focus (owner, 2026-11-24)**: real-world robustness on budget phones. Reference reader phone: Samsung Galaxy A02s. Test phone: the owner's Pixel 10 (owner, 2026-11-25: the S9+ holds other bootstrapping and is not to be disturbed).
- **Branches**: `main` (development) and `prod` (deployed by Render, fast-forwarded to `main` on release); `v1`, `fsharp-legacy`, `rust-experiment`, `water-ripple`, `card-preview-ideas` (archived/prototype lineage, kept on purpose; `water-ripple` is the SVG water effect as it last shipped, `ac1ec45`; `card-preview-ideas` holds the standalone universal card and newspaper entry prototypes tagged at `preview-cards-v1` / `d5e8e63` and `newspaper-entry-v1` / `19aeb9c`).
- **Active Experiment / Undo Instructions**: None. Devices that ran the water may still hold `res-imperii.water-quality` / `res-imperii.water-off-at` in localStorage; nothing reads them any more and they are harmless.

## Principles

The full UI language (type, colour, controls, loaders, motion, copy) is in [`HIG.md`](HIG.md).

- **Design:** Subtlety is the key. Depth, not decoration: every surface sits at one depth (HIG §4: `--depth-{sunk,0,1,2}-surface` and `-shadow`), never told apart by texture or tint. Waiting is one quiet 3×3 dot matrix: CSS-drawn, opacity-only, the same on every device.
- **Code:** Many small modules, each with one reason to change. Diligence can betray you; the type system won't. Use packages where the compiler should enforce a boundary, and files for cohesion within one.

## Modularity / type-safety plan (one PR each, no visible change, tests green)

1. ~~`AdminCap` required by every administrative write~~ (PR #31)
2. ~~Content-hashed asset URLs~~ (PR #32)
3. Typed route paths (`paths.Login(next)`, `paths.JournalEdit(id)`) and form structs shared by view and handler.
4. Split `routes` into feature packages (`web/journals`, `web/auth`, `web/books`) plus `httpx` single-purpose middleware.
5. Split `db.Store` (42 methods) into narrow repositories (`Journals`, `Books`, `Passkeys`, `AuthState`).
6. ~~JS type-checked in CI: JSDoc types, `tsc --noEmit` with `strict` (`tsconfig.json`, `assets/types/globals.d.ts`, `mage typecheck`)~~ (PR #63). Next for the JS: native ES modules, one job per file. Also worth adding to CI: `staticcheck ./...` and `deadcode -test ./...` (both clean as of PR #62).
7. CSS split by component.

## Next on Mind

- **Universal preview cards and newspaper entry layout (HIG §4, RATIONALE 086; prototyped on `card-preview-ideas`)**:
  1. **Catalog preview cards (`preview-cards-v1`, `d5e8e63`)**: wire the three universal post clipping archetypes into `views/tweet` and `views/journals`—`1C` full-bleed caption (`media` + `≤ 80` chars, author pill and caption hidden at rest, `240 ms` ease-in/out on hover, `:focus-within`, or touch viewport focal band), newspaper snippet (`media` + `> 80` chars, top-left author + left square thumb + right snippet), and pure statement (no media, top-left author + full-width text)—inside the decluttered three-layer left-aligned outer card (`clipping/media` → `excerpt` → single quiet `Date · Tags` + `Draft`/`★` footer, keeping the frozen signed-in action bar).
  2. **Full journal entry newspaper view (`newspaper-entry-v1`, `19aeb9c`)**: wire `/journals/{id}` (`views/journals`) with `display: flow-root` top-left source float (`wrap-left`, commentary right and wrapping `100%` below), bottom-stacking when the source combines media with a `> 80`-char description (`stack-rich-source`), and compressed side-by-side coexistence (`wrap-compact-code`, `~29%` source left + `~71%` `contain: inline-size` `<pre><code>` right) when an early code block meets a concise (`≤ 140`-char, media-free) source (falling back to `stack-early-code` only for longer/media sources).
- **Hosting** (owner, 2026-11-25): leave Render for a $2/month domain, served from the laptop if need be. Before the laptop goes live, harden `DEV_AUTO_LOGIN` (a per-request loopback Host check) and set `APP_ENV=prod`. With the domain on Cloudflare, cache `/assets/*` at the edge (immutable hashed URLs).
- **Stable beats peak (HIG §1.6, owner 2026-11-25)**: performance work in this order, each judged by the variance it removes:
  1. Cold starts on Render's free plan (owner's billing decision).
  2. ~~Self-hosted fonts with size-matched fallbacks~~ (PR #74; no preload: it held text back).
  3. ~~`/journals` bottleneck~~ (PR #75: catalog in the page, LCP 2274 → 1498 ms).
  4. **Next** (RATIONALE 076):
     - Cold starts: 12.6 s measured on the live site.
     - Compress locally so the lab matches production (Cloudflare serves Brotli).
     - Load htmx only for the signed-in author.
     - Edge caching of hashed assets (`cf-cache-status: DYNAMIC`).
  5. **Scale** (RATIONALE 084 measured 150 cards at LCP 1.26 s): when entries number in the thousands, drop the per-request snapshot copy, pre-lowercase the search text, refresh incrementally, and consider FTS5. A resizing proxy for linked images on the entry page.
  Dropped (owner, 2026-11-25): APM, tracing, synthetics, a boot log. Field data plus the lab is enough for a site this size.
  Ruled out: Speculation Rules prerendering, `content-visibility: auto`. Kept on purpose: the 220 ms page fade-in.
- **Local test server**: `DEV_AUTO_LOGIN=1 bin/server.exe` (real Turso from db.env, signed in). Saves change real data; a branch with a schema migration migrates the real database on start.
- **Typefaces (PR #69)**: look at entries, quotes and a long passage on the S9+. Possible next: self-host the fonts and route all Greek to Didact Gothic with `unicode-range`, so a Greek phrase in an entry no longer switches face midway (and no third-party font request).
0. **Scripture media (PR #65)**: on the S9+, tap and scroll the Book list, then save a passage and check the card. Set aside by the owner: highlighting phrases (must not put markup in the database). Possible later: ranges across chapters, other scriptures in the Scripture dropdown.
1. On the S9+ (and desktop), after PR #46 ships: seeded twinkles on tweet slots, the puzzle on Confirming and sign-in, the comet on passkey prompts, and cards unrolling smoothly (no snap) as tweets arrive.
   - Done in PR #56: tweets are server-drawn clippings (no X script); the remaining embeds (no snapshot yet, or a tapped video) are built only near the screen. Still open: check what the catalog does if `/journals/content` stalls (it now says "offline"/"couldn't reach the server" on a send error, PR #51).
   - After PR #56 ships: open `/journals` on prod twice. The first view fills snapshots in the background (Render log: `[TWEET]` lines only on failure); the second should show clippings for every tweet. Possible next: clippings for other sites (Reddit, Threads, Pinterest), each needing its own snapshot fetcher.
2. **PWA:** installable (PR #50) and an offline page (PR #51). The owner chose not to go further ("I will not boil the ocean"): no cached pages, no offline reading, no offline writes. Optional: a crisper 512 px icon redrawn from the font (the current one is a bicubic upscale of the 96 px favicon).
3. Continue the plan from step 3.
4. **Faint dark text after theme toggles (set aside by the owner, 2026-11-24; likely moot once tweets are clippings, PR #56):** toggling light/dark a few times leaves tweet-containing journal cards with faint dark text in light mode. Not reproduced in Chrome (single, 5× and 6× rapid toggles, real double-clicks during tweet load: tweets rebuilt light, colours and opacity correct, card pixels identical before/after). Needed to resume: the owner's screenshot, the browser, and whether the faint text is inside the tweet or the entry's own text. Suspects: another browser (Edge), or the light linen weave reading as text in tall stretched cards.
5. **Edge tweets (deferred by the owner, "we will address Edge later"; clippings need no X script, PR #56, so only embeds without a snapshot are affected):** tweets take a long time to load in Edge. Start in Edge DevTools on `/journals`: look for "Tracking Prevention blocked …twitter.com" and check the network timing of `platform.twitter.com` / `syndication.twitter.com`. The 12 s link-card fallback still bounds the worst case.
6. Run `mage backup` and author Books II and III via `/book-ii/edit` and `/book-iii/edit`.
