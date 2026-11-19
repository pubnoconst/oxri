# Next

Upcoming work:

- Add private server-side logging for error reference codes and failures, then
  verify that error responses never expose exception details or stack traces.
- Configure hosting and reverse-proxy error pages and timeouts to match the
  app's safe responses; verify provider-generated failures contain no traces.
- Production deployment & passkey bootstrap (D-20261117-008, D-20261117-013):
  upon live HTTPS deployment, activate WebAuthn registration in the
  `/pilot/user/settings` profile dashboard, enroll an initial primordial passkey,
  register durable credentials (Proton Pass, security keys), and retire the
  primordial key.
- Add search/filter by tag (`tag1..tag3`), bookmark filter, and storage limit
  monitoring for the Turso database (1 GiB limit).
- Review the populated catalog, empty state, permalink, and editor together in
  browser visual verification.

For the latest session context and commit reference, see [HANDOFF.md](HANDOFF.md).
