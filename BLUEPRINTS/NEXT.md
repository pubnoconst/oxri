# Next

- **Latest Commit**: `f4b4ace` (`fix(db): enforce atomic transactional quota & eviction, unify domain validation boundary, and decompose internal/db`)
- **Active Branch**: `v2-db-integrity-and-modularity` (PR open against `main` for review and UI test)
- **Active Experiment / Undo Instructions**: None.

## Next on Mind

1. **Database Data-Integrity & Modularity Audit Remediation Review**:
   - Keep Pull Request open for user review and UI testing.
   - All 6 database audit findings (empty-DB quota bypass, atomic transactional quota check, UTF-8 `CAST(... AS BLOB)` byte parity between Go and SQLite, atomic transactional `AutoVacateBytes`, unified `domain.ValidatedJournalEntry` persistence boundary, and modular decomposition of `internal/db`) are implemented and covered by concurrency/adversarial unit tests.

2. **Pre-Deployment Authentication & Operational Hardening**:
   - Challenge collision loop / unique challenge IDs, `Secure: true` cookie enforcement under HTTPS, `POST`-only `/logout`, strict host-key persistence check, and bounded SSH listener shutdown (`sync.WaitGroup`).
   - Phishing-resistant authentication (TOTP + channel/origin binding & WebAuthn passkeys).
