# Next

- **Base Commit on `main`**: `ac1ec45` (PR #45 — dot-matrix loaders on phones)
- **Active Branch**: `v2-dot-matrix` (PR #46): full dot matrix on every device; the water ripple is retired and archived on the `water-ripple` branch. Turn-signal sweep; the tweet loading line is centred (RATIONALE 047).
- **Focus (owner, 2026-11-24)**: real-world robustness on budget phones. Reference reader phone: Samsung Galaxy A02s. Test phone: the owner's S9+ (set Display → Screen resolution → HD+ to match the A02s pixel count; Developer options → Profile GPU rendering shows jank).
- **Branches**: `main`, `prod`, `v2` (live); `v1`, `fsharp-legacy`, `rust-experiment` , `water-ripple` (archived lineage, kept on purpose; `water-ripple` is the SVG water effect as it last shipped, `ac1ec45`).
- **Active Experiment / Undo Instructions**: None. Devices that ran the water may still hold `res-imperii.water-quality` / `res-imperii.water-off-at` in localStorage; nothing reads them any more and they are harmless.

## Principles

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

1. On the S9+ (and desktop), after PR #46 ships: the turn-signal sweep, the centred "Loading preview" line, no water anywhere. The owner also mentioned a Grok-style "solving Rubik's cube" matrix as an alternative motion; the column keyframes in `site.css` are the place to try it.
   - Next budget-phone candidates: build tweets only when their card nears the screen, a couple at a time (12 iframes at once is the heaviest thing left on a page); check what the catalog does if `/journals/content` fails or stalls (Render cold start on slow mobile data).
2. Continue the plan from step 3.
3. **Edge tweets (deferred by the owner, "we will address Edge later"):** tweets take a long time to load in Edge. Start in Edge DevTools on `/journals`: look for "Tracking Prevention blocked …twitter.com" and check the network timing of `platform.twitter.com` / `syndication.twitter.com`. The 12 s link-card fallback still bounds the worst case.
4. Run `mage backup` and author Books II and III via `/book-ii/edit` and `/book-iii/edit`.
