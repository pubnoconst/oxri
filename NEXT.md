# Next

Upcoming work:

- Implement central exception handling before authentication or journal actions:
  every environment must return a friendly error page without exception details
  or stack traces. Add private server-side error logging later, and verify that
  responses never expose trace information.
- Plan single-admin passkey authentication: compare a temporary whole-site
  access gate with native app authentication; define first-passkey enrollment,
  recovery, session lifetime, and adding/removing multiple credentials.
- Design author authentication before implementing protected journal actions.
- Design the Turso data model, search index, storage limit behavior, and
  migrations before connecting persistence.
- Implement the journal catalog and protected management flows after those
  designs are settled, keeping the visual treatment consistent with the editor.
- Review the catalog, empty state, and editor together once the catalog exists.

For the latest session context and commit reference, see [HANDOFF.md](HANDOFF.md).
