# Next

- **Base Commit on `main`**: `851ec93` (PR #23 — Production Hardening Sprint)
- **Active Branch**: `v2-remove-pilot-routes` (moved settings to `/settings` and removed `/pilot` routes)
- **Active Experiment / Undo Instructions**: None (isolated on `v2-remove-pilot-routes`).

## Completed Production Milestones

- **Authentication & Credentials (PRs #15, #16, #18)**:
  - Removed loopback SSH login cleanly; replaced with Argon2id passphrase (`pw.env`) + RFC 6238 TOTP (30-second per-IP confirmation cooldown with synced radial SVG water ripple) and FIDO2/WebAuthn Passkeys stored in Turso (`passkeys`, schema v3).
- **Content & UX Polish (PRs #19, #20, #21, #22)**:
  - Migrated Book I and seeded Books II & III (`*Forthcoming.*`) into Turso `books` table (schema v4) with authenticated 2-column Markdown editor and deleted `rawcontent/`.
  - Standardized all button labels to sentence case, expanded tags to 8 ASCII-only characters (schema v5), and styled checkboxes plain white (`#ffffff`) in light mode.
- **Deployment & CI (PR #17)**:
  - Distroless `CGO_ENABLED=0` `Dockerfile`, `render.yaml`, `.github/workflows/ci.yml`, live on Render (`https://res-imperii.onrender.com`, `prod` branch).
- **Production Hardening Sprint (PR #23)**:
  - Per-IP TOTP lockout and cooldown isolation, persistent TOTP replay watermark in Turso `auth_state` (schema v6), trusted-proxy IP validation, `POST`-only `/logout`, 5-minute step-up reauthentication for passkeys, strict `script-src` CSP, and `mage backup`.

## Current Work (`v2-remove-pilot-routes`)

- Moved administrative security and passkey settings from `/pilot/user/settings` (`views/pilot/`) to `/settings` (`views/settings/`) and removed all `/pilot/*` routes (`404 Not Found`) while preserving enrolled WebAuthn credentials (`res-imperii-pilot` user handle).

## Next on Mind

1. Merge the `v2-remove-pilot-routes` PR into `main` and sync `v2` / `prod` to Render.
2. Run `mage backup` to snapshot Turso into `../oxri-parent/backups/` and author Books II and III via `/book-ii/edit` and `/book-iii/edit`.
