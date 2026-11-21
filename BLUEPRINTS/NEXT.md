# Next

- **Latest Commit**: `b749c58` (`Record technology stack rationale for Gomponents, HTMX, Vanilla JS, and zero-framework CSS`)
- **Active Branch**: `v2`
- **Active Experiment / Undo Instructions**: None.

## Next on Mind

1. **Phase 1: Project Skeleton & Build System**:
   - Initialize `go.mod` on Go 1.27.4+ (pure Go, `CGO_ENABLED=0`).
   - Implement minimal `magefile.go` (`build`, `test`, `run`, `check`).
   - Set up core assets: modern CSS reset + design tokens (`assets/css/design.css`), vendored `assets/js/htmx.min.js`, and favicon.
2. **Phase 2: Domain Modeling ("Parse, Don't Validate") (`internal/domain`)**:
   - Opaque types: `MediaURL`, `Tag`, `TagSet`, `Prose`, `SafePath`, `ValidatedEntry`.
   - Compile-time capability: `AdminCap`.
3. **Phase 3: Database & Quota (`internal/db`)**:
   - Turso remote connection (`libsql://`) via external `../oxri-parent/db.env`.
   - Transactional versioned schema migrations (`schema_migrations`).
   - Self-imposed 4.5 GB storage limit & prioritized auto-eviction (`AutoVacate`).
4. **Phase 4: Authentication & Security Hardening (`internal/auth`)**:
   - Browser-bound SSH login challenges (HttpOnly cookie hash, no approve-latest vulnerability).
   - Strict HTTP security headers (CSP without `unsafe-eval`, HSTS, nosniff, frame-ancestors).
   - Rate limiting and request timeout / body cap middleware.
5. **Phase 5: Views with Gomponents & Handlers**:
   - Landing page, Book I reading view, Journal catalog & editor, Make Room overlay (`Confirming, please wait.`), Error pages.
