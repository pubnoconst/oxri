# Next

- **Base Commit on `main`**: `bf707af` (PR #31 — AdminCap on every administrative write)
- **Active Branch**: `v2-asset-hashing` (PR #32): embedded assets with typed, content-hashed URLs; README refresh (RATIONALE 033).
- **Parked Branch (local only)**: `v2-touch-ripple`, the press ripple with mirror-source bounces, UI-approved. Before shipping it needs the **adaptive performance governor**: measure frame time on the first ripple per device and step down (fewer echoes, then drop only, then no refraction); start one level down on coarse-pointer / low-memory / save-data devices; check Safari support for SVG displacement on HTML and skip silently if absent. It also needs a rebase onto `main`: PR #30 changed `views/login/login.go`, and PR #32 replaced every `?v=N` and per-page `water-ripple.js` include with typed `assets` handles (its global `water-ripple.js` / `touch-ripple.js` includes must become `assets.WaterRippleJS` / a new `assets.TouchRippleJS`).
- **Active Experiment / Undo Instructions**: None.

## Principles

- **Design:** Subtlety is the key. Linen is felt, not seen. Cards are their own cloth, told apart by weave and shadow, never by tint. Water shows as refraction, not paint.
- **Code:** Many small modules, each with one reason to change. Diligence can betray you; the type system won't. Use packages where the compiler should enforce a boundary, and files for cohesion within one.

## Modularity / type-safety plan (one PR each, no visible change, tests green)

1. ~~`AdminCap` required by every administrative write~~ (PR #31)
2. ~~Content-hashed asset URLs~~ (PR #32)
3. Typed route paths (`paths.Login(next)`, `paths.JournalEdit(id)`) and form structs shared by view and handler.
4. Split `routes` into feature packages (`web/journals`, `web/auth`, `web/books`) plus `httpx` single-purpose middleware.
5. Split `db.Store` (42 methods) into narrow repositories (`Journals`, `Books`, `Passkeys`, `AuthState`).
6. JS: `// @ts-check` + JSDoc + `tsc --noEmit` in CI (no build step), then native ES modules, one job per file.
7. CSS split by component.

## Next on Mind

1. Ship `v2-touch-ripple` with the adaptive governor (rebased onto the `assets` handles), then step 3 of the plan.
2. Shrink `favicon.png` (968 KB today) to a few KB.
3. **Edge tweets:** confirm the cause in Edge DevTools (look for "Tracking Prevention blocked …twitter.com").
4. Run `mage backup` and author Books II and III via `/book-ii/edit` and `/book-iii/edit`.
