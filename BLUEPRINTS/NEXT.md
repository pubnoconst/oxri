# Next

- **Latest Commit**: `06a591c` (`Merge pull request #12 from pubnoconst/v2-security-hardening`)
- **Active Branch**: `main` (promoted from `v2`; previous generation archived at `v1` @ `53b509e`)
- **Active Experiment / Undo Instructions**: None.

## Completed Milestones (v2 -> main)

1. **Principia & Date-Partitioned PR Workflow** (`BLUEPRINTS/Principia.md`, `BLUEPRINTS/RATIONALE/Y2026/M11/D22/pubnoconst/001..013`).
2. **Phase 1 — Pure Go Skeleton & Build System** (`CGO_ENABLED=0` `magefile.go`, `assets/css/site.css`, vendored HTMX, Landing page).
3. **Phase 2 — Domain Modeling & Capability Tokens** (`internal/domain`: `MediaURL`, `Tag`, `TagSet`, `Prose`, `SafePath`, `ValidatedEntry`, `AdminCap`).
4. **Phase 3 — Remote Turso / libSQL Database & 4.5 GB Quota** (`internal/db`: transactional versioned migrations, strict character whitelisting, prioritized `AutoVacateBytes`).
5. **Phase 4 — Hardened Loopback SSH Auth & Browser Binding** (`internal/auth`: 256-bit HttpOnly browser binding cookie, exact 6-digit token matching, zero `pty-req` noise, `/login`, `/pilot/user/settings`).
6. **Phase 5 — Views, Journals System, Make Room & Security Middleware** (`views/errorpage`, `views/booki`, `views/loading`, `views/editor`, `views/journals`, `routes/middleware.go` CSP/CSRF/nosniff/rate-limiting, vendored `markdown-it` & `DOMPurify`).

## Next on Mind

1. **Future Milestone — Anti-AiTM Phishing-Resistant Auth (TOTP + Channel/Key Binding & WebAuthn)**:
   - Bind TOTP / SSH challenges to client origin / WebAuthn passkeys (`rp_id`) once deployed over public HTTPS to prevent live relay (Adversary-in-the-Middle) phishing.
2. **Books II & III**:
   - Author and embed canonical Markdown essays for `/book-ii` and `/book-iii`.
