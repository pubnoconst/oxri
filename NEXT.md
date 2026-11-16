# Next

Upcoming work:

- Add private server-side logging for error reference codes and failures, then
  verify that error responses never expose exception details or stack traces.
- Configure hosting and reverse-proxy error pages and timeouts to match the
  app's safe responses; verify provider-generated failures contain no traces.
- Implement WebAuthn passkey authentication and credentials via `.env` once a domain and HTTPS are available.
- Design the Turso data model, search index, storage limit behavior, and
  migrations before connecting persistence.
- Implement the journal catalog and protected management flows after those
  designs are settled, keeping the visual treatment consistent with the editor.
- Review the catalog, empty state, and editor together once the catalog exists.

For the latest session context and commit reference, see [HANDOFF.md](HANDOFF.md).
