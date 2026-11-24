# Next

- **Base Commit on `main`**: `407898d` (PR #40 — no placeholder while a tweet loads; the slot unrolls with the push)
- **Active Branch**: `v2-entry-cap` (PR #41): journal entry body capped at 2,000 characters, with a live editor counter (RATIONALE 042).
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

1. If the press ripple feels choppy on the Pixel 10: limit the filter to the wave's bounding box, and add a ring-only (transform/opacity) tier that phones start on. (No Safari device is available; the governor's cautious Safari start, level 1, stays unverified.)
2. Continue the plan from step 3.
3. **Edge tweets (deferred by the owner, "we will address Edge later"):** tweets take a long time to load in Edge. Start in Edge DevTools on `/journals`: look for "Tracking Prevention blocked …twitter.com" and check the network timing of `platform.twitter.com` / `syndication.twitter.com`. The 12 s link-card fallback still bounds the worst case.
4. Run `mage backup` and author Books II and III via `/book-ii/edit` and `/book-iii/edit`.
