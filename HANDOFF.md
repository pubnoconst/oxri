# Session handoff log

This file is an append-only log for passing work between sessions. Git commits
are the source of truth for code and document changes; entries here summarize
the state and point to the commit that contains the work being handed off.

## How to add an entry

1. If the work follows a new decision, add its stable ID and rationale to
   `DECISIONS.md`, then commit that record. Keep the decision commit separate
   from implementation commits when practical.
2. Implement the work in a commit whose message references the decision ID.
   Git creates each commit hash locally, before pushing.
3. Append a dated handoff entry that links the decision ID to its decision
   commit and related implementation commit(s), and records outcomes, remaining
   work, and anything the next session should know.
4. Commit the appended handoff entry and sync the commits with the remote.
5. Do not edit or remove older entries. Add a new entry to correct or supersede
   an earlier handoff. To undo implementation, use `git revert <hash>` so the
   decision rationale and audit trail remain in history.

Keep upcoming tasks in `NEXT.md` and durable product or implementation rationale
in `DECISIONS.md`; avoid copying their full contents into this timeline.

## 2026-11-15 - `3fb2d31` Add commit-linked handoff journal

- Renamed `TODO.md` to `NEXT.md` for the actionable work queue and corrected
  the misspelled decision-journal filename to `DECISIONS.md`.
- Added this append-only handoff log and documented the three files in README.
- Updated the README's editor description to match the current media and tags
  fields, live preview, and disabled publishing state.
- The editor remains a Development-only prototype; it does not save drafts.
- No application code changed; tests were not run for this documentation-only
  work.
- Next: design author authentication and the Turso data model before building
  protected journal management. See `NEXT.md` for the current queue.

## 2026-11-15 - `1330ccd` Add session theme toggle on landing page

- Added a double-click gesture on the landing page's empty background to switch
  between light and dark themes. Interactions with navigation and content do not
  trigger the toggle.
- The choice is stored in `sessionStorage`, restored across site navigation,
  and expires with the browser tab session. The system preference remains the
  default when no explicit choice has been made.
- Updated shared theme tokens and color-scheme handling, including reduced-motion
  support for the transition, and documented the behavior in README and
  `DECISIONS.md`.
- No tests were run.
- Next: design author authentication and the Turso data model before protected
  journal management; see `NEXT.md`.

## 2026-11-15 - Decision traceability workflow

- Decision ID: `D-20261115-001`.
- Decision record commit: `0b5da2d` (`Document commit-linked decision workflow`).
- This change documents the workflow; it has no separate application
  implementation commit.
- New decisions receive stable IDs. Related implementation commits reference
  those IDs, and this log records both sets of hashes after the commits exist
  locally and before they are pushed.
- Use `git revert <implementation-hash>` to undo implementation while keeping
  the decision rationale and commit trail.

## 2026-11-15 - `d3dbe60` Plan exception handling and passkey authentication

- Added the no-stack-trace error-page requirement and deferred private server
  logging to `NEXT.md`.
- Added a planning task for one-admin passkey authentication, bootstrap,
  recovery, session policy, and credential management. No authentication
  provider or implementation approach has been selected yet.
- Research points to Cloudflare Access passkeys as a possible free temporary
  whole-site gate (free tier currently covers teams under 50 users), and to
  ASP.NET Core Identity as a native passkey option on the project's framework.
  An Access passkey is tied to that relying party and should not be assumed
  transferable to app-native authentication.
- Further study used official sources: [WebAuthn](https://www.w3.org/TR/webauthn-3/),
  [ASP.NET Core passkeys](https://learn.microsoft.com/aspnet/core/security/authentication/passkeys/?view=aspnetcore-10.0),
  [Cloudflare Access passkeys](https://developers.cloudflare.com/cloudflare-one/integrations/identity-providers/passkeys/),
  and [Cloudflare Access origin validation](https://developers.cloudflare.com/cloudflare-one/access-controls/applications/http-apps/authorization-cookie/application-token/).
- No application code was changed. The architecture remains a planning question.

## 2026-11-15 - Error pages, request timeouts, and safe QR references

- Decision: `D-20261115-003`, recorded in commit `e79bd82`.
- Implementation: commit `f8b3590` (`Add safe error pages and request timeouts`).
- Added a minimal shared error template for application-generated 404, 408, 504,
  and 500 responses. It shows the HTTP status, a generic message, a home link,
  and the ASP.NET request trace identifier as a reference code.
- The QR encodes only that reference code. A future reporting URL could be more
  actionable, but no report endpoint or support contact exists yet.
- Configured a 30-second timeout for application endpoints. Errors created by
  a CDN, reverse proxy, host, or HTTP server before they reach the app require
  matching configuration at that layer; see the open item in `NEXT.md`.
- Added a plain generic fallback if the styled page or QR rendering fails. The
  exception handler is used in Development and production; it does not display
  exception text or stack traces.
- `dotnet build --no-restore` succeeded with no warnings. Tests were not run.
- Next: add private logging for error references and verify proxy/host error
  pages; both are tracked in `NEXT.md`.

## 2026-11-15 - Route-specific stylesheet ownership

- Decision: `D-20261115-004`, recorded in commit `c27367d`.
- Implementation: commit `cf4941d` (`Split route stylesheets`).
- `Views.Page.frame` now takes a page stylesheet path and adds it after the
  shared stylesheet. Each page view passes its own versioned CSS URL.
- Split page-only rules into `wwwroot/css/pages/landing.css`, `book-i.css`,
  `journal-editor.css`, `journals.css`, and `error.css`. Shared theme, back-link,
  tag-chip, and visually-hidden rules remain in `wwwroot/css/overrides.css`.
- The shared stylesheet is now 128 lines, down from 658. The README describes
  the stylesheet ownership convention.
- `dotnet build --no-restore` succeeded with no warnings; tests were not run.
