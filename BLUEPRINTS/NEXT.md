# Next

- **Base Commit on `main`**: `a57696e` (PR #43 — water recovers from "off"; painted loading crest while off)
- **Active Branch**: `v2-journal-pages` (PR #44): the journal catalog is paginated, 12 entries per page, so budget phones never render the whole archive (RATIONALE 045).
- **Branches**: `main`, `prod`, `v2` (live); `v1`, `fsharp-legacy`, `rust-experiment` (archived lineage, kept on purpose).
- **Active Experiment / Undo Instructions**: None. To reset a device's water quality level: in the browser console, `localStorage.removeItem("res-imperii.water-quality")` (and `"res-imperii.water-off-at"`). On a phone: Chrome → Site settings → the site → Clear data.

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

1. Check the Pixel 10 (PR #43 shipped): taps should ripple again (level 1). If they are choppy there, the phone will step down to off again after two slow taps, and loading ripples will show the painted crest. The lasting fix for a phone that can't refract the viewport smoothly: limit the filter to the wave's bounding box, or a ring-only (transform/opacity) tier. (No Safari device is available; the governor's cautious Safari start, level 1, stays unverified.)
   - If the loading ripple still feels too brief on phone-sized cards, scale its duration and reach to the card's size.
2. Continue the plan from step 3.
3. **Edge tweets (deferred by the owner, "we will address Edge later"):** tweets take a long time to load in Edge. Start in Edge DevTools on `/journals`: look for "Tracking Prevention blocked …twitter.com" and check the network timing of `platform.twitter.com` / `syndication.twitter.com`. The 12 s link-card fallback still bounds the worst case.
4. Run `mage backup` and author Books II and III via `/book-ii/edit` and `/book-iii/edit`.
