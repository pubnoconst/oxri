# Next

- **Latest Commit**: `40ed717` (`feat: implement journals catalog, editor, vacate-space page, and awaiting confirmation UI`)
- **Active Branch**: `v2-journals-system` (PR open for review and UI test)
- **Active Experiment / Undo Instructions**: None.

## Next on Mind

1. **Journals Catalog, Editor, Vacate-Space & Confirmation UI Review**:
   - Keep Pull Request open for user UI testing and green light.
   - Test `/journals`, `/journals/new`, `/journals/vacate-space/`, and `/loading`.

2. **Security Headers, Request Body Caps & Rate-Limiting Hardening**:
   - Strict HTTP security headers middleware (CSP, `X-Content-Type-Options: nosniff`, `Referrer-Policy`, `Permissions-Policy`, `frame-ancestors 'none'`).
   - Request body size cap (`1 MiB`) and per-IP rate limiting on authentication endpoints.

3. **Future Milestone — Anti-AiTM Phishing-Resistant Auth (TOTP + Channel/Key Binding & WebAuthn)**:
   - Bind TOTP / SSH challenges to client origin / WebAuthn passkeys to prevent live relay (Adversary-in-the-Middle) phishing.
