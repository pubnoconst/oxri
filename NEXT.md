# Next

Upcoming work:

- **Tag uniqueness**: enforce unique tags per journal entry across client-side UI
  validation (`journal-editor.js`) and server-side validation (`db.ValidateInput`)
  so duplicate tags cannot be submitted in `tag1`, `tag2`, `tag3`.
- **Journals search**: add keyword search across journal entry content and tags on
  `/journals`.
- **Filter by tag(s)**: allow filtering `/journals` by one or more active tags
  (and bookmark status).
- **Fluid catalog layout**: make `/journals` collage layout fluid so low journal
  counts expand to fill the row naturally (`auto-fit` / balanced flex/grid sizing)
  without awkward centering or leaving a lone entry hanging on the right.
- Add storage limit monitoring for the Turso database (1 GiB product limit).
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
