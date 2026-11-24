# Next

- **Base Commit on `main`**: `b3ba4a9` (PR #44 — catalog paginated, 12 per page)
- **Active Branch**: `v2-phone-loaders` (PR #45): touch phones get a 3x3 dot-matrix loader instead of water everywhere (tweet slot line, catalog, Confirming, slow navigation); desktop unchanged (RATIONALE 046).
- **Focus (owner, 2026-11-24)**: real-world robustness on budget phones. Reference reader phone: Samsung Galaxy A02s. Test phone: the owner's S9+ (set Display → Screen resolution → HD+ to match the A02s pixel count; Developer options → Profile GPU rendering shows jank).
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

1. On the S9+ (and Pixel 10), after PR #45 ships: dots on the catalog, a "Loading preview" line per tweet that unrolls into the tweet, no tap ripple, the Confirming overlay with dots, and the "Loading" dots on a slow navigation.
   - Next budget-phone candidates: build tweets only when their card nears the screen, a couple at a time (12 iframes at once is the heaviest thing left on a page); check what the catalog does if `/journals/content` fails or stalls (Render cold start on slow mobile data).
2. Continue the plan from step 3.
3. **Edge tweets (deferred by the owner, "we will address Edge later"):** tweets take a long time to load in Edge. Start in Edge DevTools on `/journals`: look for "Tracking Prevention blocked …twitter.com" and check the network timing of `platform.twitter.com` / `syndication.twitter.com`. The 12 s link-card fallback still bounds the worst case.
4. Run `mage backup` and author Books II and III via `/book-ii/edit` and `/book-iii/edit`.
