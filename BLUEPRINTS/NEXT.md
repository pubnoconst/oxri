# Next

- **Base Commit on `main`**: `233c9a3` (PR #92: the lab as a Node script; the seeded site as a server binary)
- **Active Branch**: `windows-exodus` (PR #94, **open, not to be merged yet**: the owner holds the merge to `main`, and `prod` is not touched): the Go lab is retired and the script is the lab, run by `mage` on the host (RATIONALE 092, `c12ad06`).
- **Focus (owner, 2026-11-24)**: real-world robustness on budget phones. Reference reader phone: Samsung Galaxy A02s. Test phone: the owner's Pixel 10 (owner, 2026-11-25: the S9+ holds other bootstrapping and is not to be disturbed).
- **Branches**: `main` (development) and `prod` (deployed by Render, fast-forwarded to `main` on release); `v1`, `fsharp-legacy`, `rust-experiment`, `water-ripple`, `card-preview-ideas` (archived/prototype lineage, kept on purpose; `water-ripple` is the SVG water effect as it last shipped, `ac1ec45`; `card-preview-ideas` holds the standalone universal card and newspaper entry prototypes tagged at `preview-cards-v1` / `d5e8e63` and `newspaper-entry-v1` / `a6bd1c5`).
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

- **Windows exodus (owner, 2026-11-28; Principia #9, RATIONALE 090)**: Windows Antimalware quarantined freshly built Go binaries thirteen times in five days, and defeating it is neither standardized nor durable. Development moves to WSL, where the same build ran with no detection. Still `CGO_ENABLED=0`.
  1. ~~The lab as a Node script on the host, and the seeded site as a server binary~~: shipped together in PR #92 (`cmd/lab/lab.mjs`, `cmd/labserver`, `internal/labsite`; RATIONALE 091).
  2. ~~The owner's decision on the Go lab~~: retired (owner, 2026-11-28) after the comparison in RATIONALE 091.
  3. ~~`mage` targets and docs for WSL~~: in PR #94, open and not to be merged yet (RATIONALE 092): `mage lab`, `mage benchmark` and the new `mage labseeded` call the host's `node.exe` from WSL; `README` and `PROFILING.md` describe one lab and record that CI's timing baseline moved (the Go lab measured 20–30% higher there).
  - **Before PR #94 is merged:** the owner reads RATIONALE 091 and decides whether the unexplained part of the timing gap matters (a same-day comparison on the owner's machine needs a Chrome inside WSL, which only the owner can install: `sudo snap install chromium`).
  - **On the owner's machine:** Ubuntu 26.04.1 LTS under WSL 3.0.5 (NAT networking). Go 1.26.8 from apt (`go` fetches 1.27.4 on demand), the project's dependencies and that toolchain in the module cache (1.2 GB), a GitHub SSH key that already works, no Node, no `gh`, no `mage` on PATH (`go run github.com/magefile/mage@v1.15.0 <target>` works). The host's `node.exe` is reachable from inside WSL, and the script works from there against the Linux side. The secrets are still on the Windows side at `C:\Users\relay\proj\oxri-parent\`: a repository kept on the Windows drive (`/mnt/c/…`) finds them where they are, through `../oxri-parent/`; one moved into the Linux filesystem needs a copy. There is no `.wslconfig`: the VM may take half the RAM, and a cold build inside it left Windows 47 MB free.
  - **Open:** the admin steps in Ubuntu (`nodejs` for `mage typecheck`, `gh`, `mage`, a memory cap in `.wslconfig`); whether the repository itself moves into the Linux filesystem (faster builds: 13–15 s cold there against 18 s from `/mnt/c`); a host Chrome reaching a server in WSL costs 1.8 ms at the median and about 25 ms at p99 per request; CI's timing figures need a new baseline with the script (RATIONALE 091).
- ~~**Universal preview cards and newspaper entry layout (HIG §4, RATIONALE 086–088)**~~: shipped in PR #87 (`230d26f`)—universal `1C` caption, newspaper snippet, and statement clippings (`views/tweet`), aligned passage spine (`views/passage`), decluttered three-layer catalog cards, and newspaper `/journals/{id}` reading layout (`wrap-left`, `stack-rich-source`, stretched/centred `wrap-compact-code`, and `stack-early-code`) verified at `CLS = 0.000` in `cmd/lab`.
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
