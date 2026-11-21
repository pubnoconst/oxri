# Next

- **Latest Commit**: `85b28d7` (`Implement Phase 2 domain modeling and capability tokens`)
- **Active Branch**: `v2-phase2-domain` (PR open for review and UI test)
- **Active Experiment / Undo Instructions**: None.

## Next on Mind

1. **Phase 2 Review & UI Check**:
   - Keep Pull Request open for user green light.
   - Run server for UI test.

2. **Phase 3: Database & Quota Subsystem (`internal/db`)**:
   - Remote Turso connection (`libsql://`) via external `../oxri-parent/db.env`.
   - Transactional versioned schema migrations (`schema_migrations`).
   - Self-imposed 4.5 GB storage limit & prioritized auto-eviction (`AutoVacate`).

3. **Phase 4: Authentication & Security Hardening (`internal/auth`)**:
   - Browser-bound SSH login challenges (HttpOnly cookie hash, eliminating approve-latest vulnerability).
   - Strict HTTP security headers (CSP without `unsafe-eval`, HSTS, nosniff, frame-ancestors).
   - Rate limiting and request timeout / body cap middleware.

4. **Phase 5: Views with Gomponents & Handlers**:
   - Book I reading view, Journal catalog & editor, Make Room overlay (`Confirming, please wait.`), Error pages.
