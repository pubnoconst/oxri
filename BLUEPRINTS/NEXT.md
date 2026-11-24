# Next

- **Base Commit on `main`**: `887674a` (PR #47 — HIG.md; instructional Tags placeholder)
- **Active Branch**: `v2-catalog-loader` (PR #48): the catalog's "Loading entries…" waits bottom-centre, where the slow-navigation loader stands (RATIONALE 049).
- **Focus (owner, 2026-11-24)**: real-world robustness on budget phones. Reference reader phone: Samsung Galaxy A02s. Test phone: the owner's S9+ (set Display → Screen resolution → HD+ to match the A02s pixel count; Developer options → Profile GPU rendering shows jank).
- **Branches**: `main` (development) and `prod` (deployed by Render, fast-forwarded to `main` on release); `v1`, `fsharp-legacy`, `rust-experiment` , `water-ripple` (archived lineage, kept on purpose; `water-ripple` is the SVG water effect as it last shipped, `ac1ec45`).
- **Active Experiment / Undo Instructions**: None. Devices that ran the water may still hold `res-imperii.water-quality` / `res-imperii.water-off-at` in localStorage; nothing reads them any more and they are harmless.

## Principles

The full UI language (type, colour, controls, loaders, motion, copy) is in [`HIG.md`](HIG.md).

- **Design:** Subtlety is the key. Linen is felt, not seen. Cards are their own cloth, told apart by weave and shadow, never by tint. Waiting is one quiet 3×3 dot matrix: CSS-drawn, opacity-only, the same on every device.
- **Code:** Many small modules, each with one reason to change. Diligence can betray you; the type system won't. Use packages where the compiler should enforce a boundary, and files for cohesion within one.

## Modularity / type-safety plan (one PR each, no visible change, tests green)

1. ~~`AdminCap` required by every administrative write~~ (PR #31)
2. ~~Content-hashed asset URLs~~ (PR #32)
3. Typed route paths (`paths.Login(next)`, `paths.JournalEdit(id)`) and form structs shared by view and handler.
4. Split `routes` into feature packages (`web/journals`, `web/auth`, `web/books`) plus `httpx` single-purpose middleware.
5. Split `db.Store` (42 methods) into narrow repositories (`Journals`, `Books`, `Passkeys`, `AuthState`).
6. JS: `// @ts-check` + JSDoc + `tsc --noEmit` in CI (no build step), then native ES modules, one job per file. (Highest value for the frontend: most bugs this project has had lived in untyped JS.)
7. CSS split by component.

## Next on Mind

1. On the S9+ (and desktop), after PR #46 ships: seeded twinkles on tweet slots, the puzzle on Confirming and sign-in, the comet on passkey prompts, and cards unrolling smoothly (no snap) as tweets arrive.
   - Next budget-phone candidates: build tweets only when their card nears the screen, a couple at a time (12 iframes at once is the heaviest thing left on a page); check what the catalog does if `/journals/content` fails or stalls (Render cold start on slow mobile data).
2. Continue the plan from step 3.
3. **Faint dark text after theme toggles (set aside by the owner, 2026-11-24):** toggling light/dark a few times leaves tweet-containing journal cards with faint dark text in light mode. Not reproduced in Chrome (single, 5× and 6× rapid toggles, real double-clicks during tweet load: tweets rebuilt light, colours and opacity correct, card pixels identical before/after). Needed to resume: the owner's screenshot, the browser, and whether the faint text is inside the tweet or the entry's own text. Suspects: another browser (Edge), or the light linen weave reading as text in tall stretched cards.
4. **Edge tweets (deferred by the owner, "we will address Edge later"):** tweets take a long time to load in Edge. Start in Edge DevTools on `/journals`: look for "Tracking Prevention blocked …twitter.com" and check the network timing of `platform.twitter.com` / `syndication.twitter.com`. The 12 s link-card fallback still bounds the worst case.
5. Run `mage backup` and author Books II and III via `/book-ii/edit` and `/book-iii/edit`.
