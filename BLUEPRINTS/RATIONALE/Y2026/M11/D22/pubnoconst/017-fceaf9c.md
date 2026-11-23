# 017 — Pure-Go TOTP login with ripple confirmation UI (`fceaf9c`)

- **Commit**: `fceaf9c` (head of PR)
- **Committed**: 2026-11-22 13:44:26 +1000
- **Committer**: `pubnoconst`
- **Branch**: `v2-totp-auth`
- **Merged**: PR #15 as `06d360c` at 2026-11-22 13:49:15 +1000

> Reconstructed on 2026-11-23 from git and GitHub history: no RATIONALE file was committed when this PR merged. *Changes* is derived from the commits and touched files; *PR description* is the reasoning as written on GitHub at merge time.

## Commits

- `947036b` 2026-11-22 13:07:25 +1000 — feat(auth): add pure-Go RFC 6238 TOTP login, replay/lockout protection, HTTPS cookie hardening, and mage totp QR generator
- `07eb252` 2026-11-22 13:19:24 +1000 — feat(auth): enforce 30s server-side TOTP confirmation cooldown with water-drop ripple and progress UI
- `495127d` 2026-11-22 13:24:27 +1000 — ux(auth): make confirmation water-drop ripple pagewide and center Verify button
- `f23d9fd` 2026-11-22 13:27:53 +1000 — ux(auth): add submerged SVG water-surface refraction filter and elevated pagewide sunlit ripple
- `24ad0fe` 2026-11-22 13:35:48 +1000 — ux(auth): replace noise wobble with realistic radial wavefront refraction driven by the sunlit ripple rings
- `fceaf9c` 2026-11-22 13:44:26 +1000 — ux(water-ripple): soften wave refraction and share across /login/confirm, /journals Turso fetch, and Twitter card embeds

## Changes

- Added RFC 6238 TOTP login implemented in pure Go (`internal/auth/totp.go`), with replay/lockout protection, HTTPS cookie hardening, and a `mage totp` QR generator (`cmd/totpgen`) (`947036b`).
- Enforced a 30 s server-side confirmation cooldown, visualised by a pagewide water-drop ripple and progress UI (`07eb252`, `495127d`).
- Introduced `assets/js/water-ripple.js`: SVG water-surface refraction driven by the ripple rings, later softened and shared across `/login/confirm`, the `/journals` Turso fetch, and Twitter card embeds (`f23d9fd`, `24ad0fe`, `fceaf9c`).

## PR description (#15, as written at merge time)

Adds pure-Go RFC 6238 TOTP login with single-use replay protection, failed-attempt lockout, per-IP rate limiting, HTTPS Secure cookies, trusted proxy IP parsing, and a mage totp generator that writes ADMIN_TOTP_SECRET and totp-qr.png to the parent secrets folder.
