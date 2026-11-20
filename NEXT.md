# Next

> **STATUS: Development stalled until 2026-11-27 per user instruction.**

When development resumes on **11/27/2026**:

1. **Non-technical UX copy rule (D-20261121-002)**:
   - Audit and sanitize all user-facing copy to eliminate internal engineering terms (e.g., "Turso", raw byte debug figures).
   - Talk to the user as if they know very little about technology. It is a UX failure if the user ever has to look something up.
2. **Pure-CSS magnifying-wave loading state (`/loading`) (D-20261121-002)**:
   - Produce an independent loading view at `/loading` for user UI testing before integrating into form transitions.
   - Copy: `"Ensuring confirmation, this should be quick."`
   - Centered text that gently pulsates overall, with a pure-CSS linear magnifying-glass wave animation where each character smoothly scales up and back down in sequence.
   - Smoothly ease into this state during validation/budget checks to prevent page flickering.
3. **Restore default storage limit**:
   - Revert the temporary single-entry storage limit override (`store.SetMaxStorageBytes(usedBytes)`) in `cmd/server/main.go` back to standard 4.5 GB (`DefaultMaxStorageBytes`).
4. **Hosting, reverse-proxy error alignment & production deployment**:
   - Align upstream reverse-proxy timeouts and error pages.
   - Production passkey enrollment and bootstrap (`/pilot/user/settings`).
