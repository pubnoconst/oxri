# Next

- **Base Commit on `main`**: `6e9ec8a` (PR #22 — light-mode plain white checkboxes & synced `color-scheme`)
- **Active Branch**: `v2-prod-hardening` (Production Hardening Sprint ready for PR review)
- **Active Experiment / Undo Instructions**: None (all changes isolated on `v2-prod-hardening`).

## Completed Production Milestones

- **Authentication & Credentials (PRs #15, #16, #18)**:
  - Removed loopback SSH login cleanly; replaced with Argon2id passphrase (`pw.env`) + RFC 6238 TOTP (30-second per-IP confirmation cooldown with synced radial SVG water ripple) and FIDO2/WebAuthn Passkeys stored in Turso (`passkeys`, schema v3).
- **Content & UX Polish (PRs #19, #20, #21, #22)**:
  - Migrated Book I and seeded Books II & III (`*Forthcoming.*`) into Turso `books` table (schema v4) with authenticated 2-column Markdown editor and deleted `rawcontent/`.
  - Standardized all button labels to sentence case, expanded tags to 8 ASCII-only characters (schema v5), and styled checkboxes plain white (`#ffffff`) in light mode.
- **Deployment & CI (PR #17)**:
  - Distroless `CGO_ENABLED=0` `Dockerfile`, `render.yaml`, `.github/workflows/ci.yml`, live on Render (`https://res-imperii.onrender.com`, `prod` branch).

## Current Work (`v2-prod-hardening` — Production Hardening Sprint)

1. **Per-IP Lockout & Bounded Auth Maps**: Isolated TOTP failed-attempt lockouts and 30s single-flight cooldowns per client IP (avoiding global DoS lockout) and bounded pending/session/limiter maps.
2. **Restart-Safe TOTP Replay Persistence (Schema v6)**: Persisted `last_totp_step` monotonically in Turso `auth_state` via `auth.TOTPReplayStore`.
3. **Hardened Proxy IP Trust**: Validated trusted peer CIDRs (loopback, private, CGNAT `100.64.0.0/10`) before honoring `True-Client-IP`, `CF-Connecting-IP`, or rightmost untrusted `X-Forwarded-For` hop.
4. **POST-Only `/logout` & Step-Up Passkey Reauth**: Made `/logout` POST-only (`GET` returns `405`) and gated `/pilot/passkeys/*` add/delete actions behind fresh 5-minute reauthentication (`RequireFreshAuth`, `?reauth=1`).
5. **Strict Script CSP & Offline Backup CLI**: Removed `'unsafe-inline'` from `script-src` by externalizing form confirm prompts into `assets/js/navigation-feedback.js`, and added `cmd/backup` (`mage backup`) for offline Turso JSON snapshots and restoration.

## Next on Mind

1. Merge the Production Hardening PR into `main`, sync `v2` and `prod` to deploy to Render, and run an initial `mage backup` snapshot to `../oxri-parent/backups/`.
2. Author Books II and III via `/book-ii/edit` and `/book-iii/edit` directly in production.
