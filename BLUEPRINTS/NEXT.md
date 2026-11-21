# Next

- **Latest Commit**: `e2b96d1` (`feat: add smooth page transitions (ease in/out) and connect remote Turso database`)
- **Active Branch**: `v2-transitions-and-db` (PR open for review and UI test)
- **Active Experiment / Undo Instructions**: None.

## Next on Mind

1. **Page Transitions & Remote Turso DB Review**:
   - Keep Pull Request open for user UI testing and green light.
   - Verify remote Turso connection at startup and smooth page ease-in/out transitions across routes.

2. **Journal System, Editor & Make Room Flow (`views/journals`, `views/editor`, `views/vacate`, `serverfns/journals.go`)**:
   - Journal catalog view (`/journals`) with tag filter, search query, and single-entry permalink (`/journals/{id}`).
   - Journal editor (`/journals/new`, `/journals/{id}/edit`) with live tactile media preview (`assets/js/journal-media.js`) and Markdown preview.
   - Make Room / Vacate Space view (`/journals/vacate-space`) with prioritized Auto-Vacate and `Confirming, please wait.` loading overlay.
