# Next

- **Base Commit on `main`**: `482d16c` (PR #38 — press ripple regression fix)
- **Active Branch**: `v2-tweet-flip` (PR #39): tweet reveal via FLIP instead of an animated height (choppy on Pixel 10); box-shadow replaces a drop-shadow filter over the tweet iframe (RATIONALE 040). Awaiting the owner's go-ahead to ship, then a check on the Pixel.
- **Branches**: `main`, `prod`, `v2` (live); `v1`, `fsharp-legacy`, `rust-experiment` (archived lineage, kept on purpose).
- **Active Experiment / Undo Instructions**: None. To reset a device's water quality level: in the browser console, `localStorage.removeItem("res-imperii.water-quality")`.

## Principles

- **Design:** Subtlety is the key. Linen is felt, not seen. Cards are their own cloth, told apart by weave and shadow, never by tint. Water shows as refraction, not paint.
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

1. After PR #39 ships, check the tweet reveal and the press ripple on the Pixel 10. If the press ripple is still choppy: limit the filter to the wave's bounding box, and add a ring-only (transform/opacity) tier that phones start on.
2. Watch the press ripple on a real phone. (No Safari device is available; the governor's cautious Safari start, level 1, stays unverified.)
3. Continue the plan from step 3.
4. **Edge tweets:** confirm the cause in Edge DevTools (look for "Tracking Prevention blocked …twitter.com").
5. Run `mage backup` and author Books II and III via `/book-ii/edit` and `/book-iii/edit`.
