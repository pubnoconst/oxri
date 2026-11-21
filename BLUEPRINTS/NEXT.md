# Next

- **Latest Commit**: `675b541` (`Implement Book I reader view, Goldmark markdown parsing, and BackLink navigation`)
- **Active Branch**: `v2-book-i` (PR open for review and UI test)
- **Active Experiment / Undo Instructions**: None.

## Next on Mind

1. **Book I UI Test & Review**:
   - Keep Pull Request open for user UI testing and green light.
   - Run server for UI test at `/book-i`.

2. **Phase 3: Database & Quota Subsystem (`internal/db`)**:
   - Remote Turso connection (`libsql://`) via external `../oxri-parent/db.env`.
   - Transactional versioned schema migrations (`schema_migrations`).
   - Self-imposed 4.5 GB storage limit & prioritized auto-eviction (`AutoVacate`).

3. **Phase 4: Authentication & Security Hardening (`internal/auth`)**:
   - Browser-bound SSH login challenges (HttpOnly cookie hash, eliminating approve-latest vulnerability).
   - Strict HTTP security headers (CSP without `unsafe-eval`, HSTS, nosniff, frame-ancestors).
   - Rate limiting and request timeout / body cap middleware.

4. **Phase 5: Journal System & Editor**:
   - Journal catalog view, Journal editor, Make Room overlay (`Confirming, please wait.`).
