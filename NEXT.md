# Next

Upcoming work:

- **Storage budget & auto-eviction (D-20261120-003)**: operate up to 90% of
  Turso's 5 GB free tier (4.5 GB). When exceeded, prompt the author to either
  delete entries manually first or auto-delete the earliest unpublished entries
  (`published = 0` ordered by `created_at ASC, id ASC`).
- Add private server-side logging for error reference codes and failures, then
  verify that error responses never expose exception details or stack traces.
- Configure hosting and reverse-proxy error pages and timeouts to match the
  app's safe responses; verify provider-generated failures contain no traces.
- Production deployment & passkey bootstrap (D-20261117-008, D-20261117-013):
  upon live HTTPS deployment, activate WebAuthn registration in the
  `/pilot/user/settings` profile dashboard, enroll an initial primordial passkey,
  register durable credentials (Proton Pass, security keys), and retire the
  primordial key.
- Review the populated catalog, empty state, permalink, and editor together in
  browser visual verification.

For the latest session context and commit reference, see [HANDOFF.md](HANDOFF.md).
