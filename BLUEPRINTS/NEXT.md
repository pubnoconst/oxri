# Next

- **Base Commit on `main`**: `7aa58c5` (PR #64 — catalog media centred)
- **Active Branch**: `scripture-media` (PR #65): an entry's media can be a Bible passage, picked by book/from/to/translation, fetched from the Free Use Bible API and verified by the server on save (RATIONALE 066, `430af3e`).
- **Focus (owner, 2026-11-24)**: real-world robustness on budget phones. Reference reader phone: Samsung Galaxy A02s. Test phone: the owner's S9+ (set Display → Screen resolution → HD+ to match the A02s pixel count; Developer options → Profile GPU rendering shows jank).
- **Branches**: `main` (development) and `prod` (deployed by Render, fast-forwarded to `main` on release); `v1`, `fsharp-legacy`, `rust-experiment` , `water-ripple` (archived lineage, kept on purpose; `water-ripple` is the SVG water effect as it last shipped, `ac1ec45`).
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

0. **Scripture media (PR #65)**: on the S9+, tap and scroll the Book list, then save a passage and check the card. Set aside by the owner: highlighting phrases (must not put markup in the database). Possible later: ranges across chapters, other scriptures in the Scripture dropdown.
1. On the S9+ (and desktop), after PR #46 ships: seeded twinkles on tweet slots, the puzzle on Confirming and sign-in, the comet on passkey prompts, and cards unrolling smoothly (no snap) as tweets arrive.
   - Done in PR #56: tweets are server-drawn clippings (no X script); the remaining embeds (no snapshot yet, or a tapped video) are built only near the screen. Still open: check what the catalog does if `/journals/content` stalls (it now says "offline"/"couldn't reach the server" on a send error, PR #51).
   - After PR #56 ships: open `/journals` on prod twice. The first view fills snapshots in the background (Render log: `[TWEET]` lines only on failure); the second should show clippings for every tweet. Possible next: clippings for other sites (Reddit, Threads, Pinterest), each needing its own snapshot fetcher.
2. **PWA:** installable (PR #50) and an offline page (PR #51). The owner chose not to go further ("I will not boil the ocean"): no cached pages, no offline reading, no offline writes. Optional: a crisper 512 px icon redrawn from the font (the current one is a bicubic upscale of the 96 px favicon).
3. Continue the plan from step 3.
4. **Faint dark text after theme toggles (set aside by the owner, 2026-11-24; likely moot once tweets are clippings, PR #56):** toggling light/dark a few times leaves tweet-containing journal cards with faint dark text in light mode. Not reproduced in Chrome (single, 5× and 6× rapid toggles, real double-clicks during tweet load: tweets rebuilt light, colours and opacity correct, card pixels identical before/after). Needed to resume: the owner's screenshot, the browser, and whether the faint text is inside the tweet or the entry's own text. Suspects: another browser (Edge), or the light linen weave reading as text in tall stretched cards.
5. **Edge tweets (deferred by the owner, "we will address Edge later"; clippings need no X script, PR #56, so only embeds without a snapshot are affected):** tweets take a long time to load in Edge. Start in Edge DevTools on `/journals`: look for "Tracking Prevention blocked …twitter.com" and check the network timing of `platform.twitter.com` / `syndication.twitter.com`. The 12 s link-card fallback still bounds the worst case.
6. Run `mage backup` and author Books II and III via `/book-ii/edit` and `/book-iii/edit`.
