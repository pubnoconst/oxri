# Next

- **Base Commit on `main`**: `169d14d` (PR #29 — linen surfaces)
- **Active Branch**: `v2-auth-asymmetric` (PR #30): instant password+TOTP sign-in; escalating per-IP holds, per-IP lockout and a global failure budget; 30 s confirm page removed (RATIONALE 031).
- **Parked Branch (local only)**: `v2-touch-ripple`, the press ripple with mirror-source bounces, UI-approved. Before shipping it needs the **adaptive performance governor**: measure frame time on the first ripple per device and step down (fewer echoes, then drop only, then no refraction); start one level down on coarse-pointer / low-memory / save-data devices; check Safari support for SVG displacement on HTML and skip silently if absent. It also needs a rebase onto `main`, because it touches `views/login/login.go`, which PR #30 changed.
- **Active Experiment / Undo Instructions**: None.

## Design principle

Subtlety is the key. Linen is felt, not seen. Cards are their own cloth, told apart by weave and shadow, never by tint. Water shows as refraction, not paint.

## Next on Mind

1. Ship `v2-touch-ripple` with the adaptive governor (above).
2. **Edge tweets:** confirm the cause in Edge DevTools (look for "Tracking Prevention blocked …twitter.com"). Blocked embeds fall back to the X link card after 12 s.
3. Run `mage backup` and author Books II and III via `/book-ii/edit` and `/book-iii/edit`.
