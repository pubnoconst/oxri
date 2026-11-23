# Next

- **Base Commit on `main`**: `7f6cc82` (PR #35 — passkey end-to-end tests; PR #34 quick wins before it)
- **Active Branch**: `v2-touch-ripple-2` (PR #36): press ripple with wall echoes and the adaptive quality governor; add-a-passkey moved inside the WebAuthn card; live session-dot ripple (RATIONALE 037).
- **Retired branch**: `v2-touch-ripple` (local only, superseded by `v2-touch-ripple-2`; safe to delete).
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

1. Watch the press ripple on real devices: a phone, and Safari if available (the governor starts Safari at level 1, unverified).
2. Continue the plan from step 3.
3. **Edge tweets:** confirm the cause in Edge DevTools (look for "Tracking Prevention blocked …twitter.com").
4. Run `mage backup` and author Books II and III via `/book-ii/edit` and `/book-iii/edit`.
