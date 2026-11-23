# Next

- **Base Commit on `main`**: `0aadaff` (PR #30 — instant sign-in, failure budget)
- **Active Branch**: `v2-admin-cap` (PR #31): every administrative write demands `domain.AdminCap`; admin handlers are served only via `adminOnly` / `freshAdminOnly` (RATIONALE 032).
- **Parked Branch (local only)**: `v2-touch-ripple`, the press ripple with mirror-source bounces, UI-approved. Before shipping it needs the **adaptive performance governor**: measure frame time on the first ripple per device and step down (fewer echoes, then drop only, then no refraction); start one level down on coarse-pointer / low-memory / save-data devices; check Safari support for SVG displacement on HTML and skip silently if absent. It also needs a rebase onto `main` (PR #30 changed `views/login/login.go`). Land it after step 2 below, to avoid more manual `?v=` bumps.
- **Active Experiment / Undo Instructions**: None.

## Principles

- **Design:** Subtlety is the key. Linen is felt, not seen. Cards are their own cloth, told apart by weave and shadow, never by tint. Water shows as refraction, not paint.
- **Code:** Many small modules, each with one reason to change. Diligence can betray you; the type system won't. Use packages where the compiler should enforce a boundary, and files for cohesion within one.

## Modularity / type-safety plan (one PR each, no visible change, tests green)

1. ~~`AdminCap` required by every administrative write~~ (PR #31)
2. Content-hashed asset URLs: an `assets` package hashing files at startup, typed asset names, and no more hand-bumped `?v=N` (38 today).
3. Typed route paths (`paths.Login(next)`, `paths.JournalEdit(id)`) and form structs shared by view and handler.
4. Split `routes` into feature packages (`web/journals`, `web/auth`, `web/books`) plus `httpx` single-purpose middleware.
5. Split `db.Store` (42 methods) into narrow repositories (`Journals`, `Books`, `Passkeys`, `AuthState`).
6. JS: `// @ts-check` + JSDoc + `tsc --noEmit` in CI (no build step), then native ES modules, one job per file.
7. CSS split by component.

## Next on Mind

1. Step 2 of the plan, then ship `v2-touch-ripple` with the adaptive governor.
2. **Edge tweets:** confirm the cause in Edge DevTools (look for "Tracking Prevention blocked …twitter.com").
3. Run `mage backup` and author Books II and III via `/book-ii/edit` and `/book-iii/edit`.
