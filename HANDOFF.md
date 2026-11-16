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

## 2026-11-15 - Theme-aware navigation feedback

- Decision ID: `D-20261115-007`.
- Decision record commit: `97d38dc` (`Decide theme-aware navigation feedback`).
- Implementation commit: `321691f` (`Keep navigation feedback theme-aware`).
- Limited pending-navigation dimming to dark mode, including its reduced-motion
  fallback. The cross-document view-transition layer now uses the active page
  canvas color, preventing the fade from exposing a black backdrop in light mode.
- Documented the behavior in README and added CSS comments explaining both
  theme-aware dimming and the transition backdrop.
- `git diff --check` passed. Browser visual verification remains outstanding.

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

## 2026-11-15 - Disable landing text selection

- Decision: `D-20261115-005`, recorded in commit `78ef0fd`.
- Implementation: commit `d6bb83f` (`Disable landing text selection`).
- Set the landing canvas to `user-select: none` (including the WebKit
  property), so double-clicking near the navigation does not select text.
  Links remain interactive. Bumped the landing stylesheet cache version.
- Tests were not run.

## 2026-11-15 - Pending navigation feedback

- Decision: `D-20261115-006`, recorded in commit `6c77808`.
- Implementation: commit `371b2eb` (`Add pending navigation feedback`).
- Added a subtle cross-document fade where browser view transitions are
  supported, plus delayed breathing dim feedback for same-origin link clicks.
  Modified clicks, new tabs, downloads, and external links do not trigger it.
- Reduced-motion preferences disable the page transition and use a steady slight
  dim for pending navigation. The pending state clears on page restore or after
  35 seconds if the navigation does not complete.
- Bumped the shared stylesheet cache version. `git diff --check` passed; tests
  were not run.

## 2026-11-16 - Register journal routes in release

- Decision: `D-20261116-001`, recorded in commit `98647b2`.
- Implementation: commit `932fd8c` (`Register journal prototype routes in release`).
- Added `/journals` and `/journal-editor` directly to the canonical `endpoints`
  list, so both routes are registered in every environment. Removed the
  Development-only endpoint condition. The editor remains browser-only, does
  not save drafts, and keeps Publish disabled.
- README and current decision guidance now describe both routes as available in
  all environments.
- `dotnet build --no-restore -c Release` succeeded with no warnings or errors;
  tests were not run.

## 2026-11-16 - Start Rust MASH experiment

- Decision: `D-20261116-002`, recorded in commit `50a1f5589a81a3264a80c23f923880dd3ef86c1e`.
- Implementation: commit `29360142528442452c5487c32b602c8968ef8951` (`Scaffold Rust MASH experiment D-20261116-002`).
- Created a parallel Rust application scaffold on `rust-experiment` using Maud,
  Axum, SQLx, and HTMX. Added the shared page shell, the landing, Book I,
  journals, and journal-editor routes, static asset services, a branded 404,
  and a generic panic response that omits exception details.
- Ported the current editor's browser-only Markdown, tags, and media preview;
  publishing remains disabled. Added `Cargo.lock`, a stable ARM64 MSVC toolchain
  file, and `/target/` to `.gitignore`. README documents the experiment and
  build prerequisites.
- Rustup stable `aarch64-pc-windows-msvc` and rustfmt are installed. Cargo
  resolved dependencies and generated the lockfile. Cargo metadata
  (`--no-deps --offline`) and `cargo fmt` succeeded. `cargo check` could not complete because
  this machine has no MSVC `link.exe`; Clang is not currently required by the
  selected dependency features. Tests were not run.
- SQLx has no database driver or TLS backend enabled. Add these when selecting
  and implementing the database connection. The .NET app on `main` remains the
  canonical version.

## 2026-11-16 - Install Windows ARM64 C++ build tools

- Follow-up to decision `D-20261116-002` and implementation commit `29360142528442452c5487c32b602c8968ef8951`.
- Installed Microsoft Visual Studio Build Tools 2022 (17.14.45) with the C++
  workload, ARM64 tools, and recommended SDK components. `link.exe` is available
  through the ARM64 developer environment initialized by `VsDevCmd.bat`.
- `cargo check` and `cargo build` both succeeded for `aarch64-pc-windows-msvc`
  from that environment. Both report one expected warning: SQLx is included as
  part of the MASH stack but remains unused until persistence is implemented.
- README now notes the developer environment needed to run Cargo commands.
  Clang is not needed for the current feature set; no Rust tests were run.

## 2026-11-16 - Make Rust branch standalone and colocate page styles

- Decision: `D-20261116-003`, recorded in commit `d3ed93f7400d165252b313b684a3114b03cdf1fb`.
- Implementation: commit `781a5c8f6cbc90456b808407212aaf7b69a0e0b4` (`Refactor standalone Rust project layout D-20261116-003`).
- Each Rust page now has `src/pages/<page>/mod.rs` and `style.css` together.
  Page styles are embedded with `include_str!` and returned from explicit CSS
  routes, keeping Rust source out of the static file service. Shared CSS,
  browser scripts, and the favicon now live in `assets/`.
- Removed the F#/Oxpecker project, appsettings, old `Views` and `wwwroot` trees,
  and local .NET `bin`/`obj` output from this branch. `main` is unchanged, and
  `rawcontent/` remains excluded from Git. Replaced the old .NET-heavy ignore
  template with Rust, local secrets, author source, IDE, OS, and leftover build
  output rules.
- README and current implementation guidance in `DECISIONS.md` describe the
  layout. Cargo formatting, metadata, `cargo check`, and `cargo build` succeeded
  for Windows ARM64. The only warning is the intentionally not-yet-used SQLx
  dependency. Tests were not run.

## 2026-11-16 - Publish Book I source and restore shared error behavior

- Decision: `D-20261116-004`, recorded in commit `d7ed0414359be3a068445cd05ee10e03ee089feb`.
- Implementation: commit `7b69afde21330af957daa0b4dd8f4a77388bfaaf` (`Restore Book I, errors, and page styling D-20261116-004`).
- Removed `rawcontent/` from `.gitignore` and added `rawcontent/book-i.md` to
  the private repository. `/book-i` now embeds and renders the complete Markdown
  source. The parser preserves Markdown formatting and drops raw HTML.
- Added a status-page middleware for every application 4xx/5xx response, a
  30-second route deadline that returns 504, and generic panic handling. The
  formatted page shows the status, a random opaque reference, and a locally
  generated QR encoding only that reference; a plain safe page is used if the
  formatted renderer fails. Exception details and stack traces remain hidden.
- Restored the Back-link icon and label markup. Updated the Book I CSS for
  rendered Markdown headings and spacing, and refined the Journals empty-state
  layout and action styling.
- `cargo fmt`, `cargo check --offline`, and `cargo build --offline` succeeded
  for Windows ARM64. The expected unused-SQLx warning remains. Tests were not
  run.
