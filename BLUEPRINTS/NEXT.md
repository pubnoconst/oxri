# Next

- **Latest Commit**: `df215c7` (`Implement Phase 1 skeleton, CGO_ENABLED=0 magefile, and landing page`)
- **Active Branch**: `v2-phase1-skeleton-landing` (merging to `v2`)
- **Active Experiment / Undo Instructions**: None.

## Next on Mind

1. **Phase 2: Domain Modeling ("Parse, Don't Validate") (`internal/domain`)**:
   - Implement opaque domain types with unexported fields:
     - `MediaURL`: Whitelisted URLs with strict scheme (`https://`), host whitelisting, and no internal IP addresses.
     - `Tag` & `TagSet`: Canonical alphanumeric lower-case kebab tags.
     - `Prose`: Sanitized markdown text with length caps.
     - `SafePath`: Bounded alphanumeric path tokens preventing path traversal.
     - `ValidatedEntry`: Composite domain entity that can only be created via constructors returning `(ValidatedEntry, error)`.
   - Implement compile-time capability tokens:
     - `AdminCap`: Unexported field token minted only upon verified cryptographic SSH challenge response.
   - Comprehensive unit tests covering fuzzing, character whitelisting, and state unrepresentability.

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
