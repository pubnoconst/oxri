# Next

- **Latest Commit**: `5a2211c` (`Implement loopback SSH challenge authentication, browser binding, and pilot settings view`)
- **Active Branch**: `v2-auth-ssh-profile` (PR open for review and UI test)
- **Active Experiment / Undo Instructions**: None.

## Next on Mind

1. **Authentication & Profile UI Test & Review**:
   - Keep Pull Request open for user UI testing and green light.
   - Run server for UI test at `/login` and `/pilot/user/settings`.

2. **Phase 3: Database & Quota Subsystem (`internal/db`)**:
   - Remote Turso connection (`libsql://`) via external `../oxri-parent/db.env`.
   - Transactional versioned schema migrations (`schema_migrations`).
   - Self-imposed 4.5 GB storage limit & prioritized auto-eviction (`AutoVacate`).

3. **Phase 5: Journal System & Editor**:
   - Journal catalog view, Journal editor, Make Room overlay (`Confirming, please wait.`).
