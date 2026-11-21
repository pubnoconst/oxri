# Next

Upcoming work:

1. **Pre-deployment security hardening** (from security review):
   - Remove `ApproveLatestPendingLogin` (`admin@` shortcut) and bind login
     challenges to an HttpOnly cookie; rate-limit `/auth/status` per IP.
   - Upgrade Go toolchain (`go1.27.4+`) to resolve stdlib CVEs.
   - Add security headers (CSP, `X-Content-Type-Options`, `Referrer-Policy`,
     HSTS), vendor htmx/Alpine locally, fix `DistinctTags(ctx, isAuth)` on
     `/journals/new`, and harden the SSH listener (deadlines, connection cap).
2. **Hosting, reverse-proxy error alignment & production deployment**:
   - Align upstream reverse-proxy timeouts and error pages.
   - Production passkey enrollment and bootstrap (`/pilot/user/settings`).

For the latest session context and commit reference, see [HANDOFF.md](HANDOFF.md).
