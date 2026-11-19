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

## 2026-11-17 - Restore native navigation and shared article measure

- Decision: `D-20261117-001`, recorded in commit `32dce1b5e084fcb55619ee5034edc46186138019`.
- Implementation: commit `78dac388b072769cd5ffa0042787f6010c43d5a2` (`Restore native navigation and shared article measure D-20261117-001`).
- Removed `hx-boost` from the landing page navigation. HTMX body swapping was
  bypassing `<head>` updates on same-origin navigation, causing `/book-i` and
  `/journals` to miss their page-specific stylesheets, breaking layout until
  a full document reload. It also intercepted clicks before navigation feedback
  and cross-document view transitions could run, and suppressed 4xx/5xx responses
  (preventing the 404 error page from rendering on `/book-ii`).
- Added centered article measure rules (`width: min(100%, 42rem); margin: 0 auto;`)
  to shared `overrides.css`, ensuring Book I and any future book-n reading columns
  consistently format as a Medium-style centered reading column. Bumped
  `overrides.css` cache version to `v=22` in `src/pages/shared.rs`.
- Added unit tests in `src/main.rs` covering landing page native navigation,
  `/book-i` article styling and stylesheet links, `/journals` rendering and stylesheet
  links, `/book-ii` 404 error page rendering, panic handling, and request timeouts.
- `cargo fmt` and `cargo test` succeeded for Windows ARM64 (7 passed, 0 failed).

## 2026-11-17 - Adopt Alpine.js for client-side manipulation with HTMX

- Decision: `D-20261117-002`, recorded in commit `a28733ac69508b0eefe3301f18cecf25a5afca0a`.
- Implementation: commit `c835df24580609eb85989572517ca417ca6bcf8e` (`Add Alpine.js to shared page shell D-20261117-002`).
- Permitted Alpine.js for lightweight client-side state and UI reactivity alongside
  HTMX. Alpine 3.x uses MutationObserver internally, enabling clean automatic
  initialization of dynamic components across HTMX swaps.
- Added Alpine.js 3.14.8 (`defer`) to `src/pages/shared.rs` after HTMX.
- Updated `src/main.rs` tests to verify Alpine and HTMX availability in the shell.
- `cargo fmt` and `cargo test` succeeded for Windows ARM64 (7 passed, 0 failed).

## 2026-11-17 - Scaffold Go GOTH experiment with Magefile

- Decision: `D-20261117-003`, recorded in commit `91c65ca28329b8994b539a9b6ac014441a9b3966`.
- Implementation: commit `f625a237429bd68a72522eccac5bca9358be3ab9` (`Scaffold Go GOTH stack with Magefile D-20261117-003`).
- Branched to `go-experiment` and translated the full web application to Go using
  the GOTH stack (Go 1.27, Templ, HTMX, Alpine.js, and Bulma).
- Implemented standard Go project layout hygiene: `cmd/server/main.go`, `internal/app/`
  for routing and middleware, `internal/pages/shared/` for layout, and colocated
  `.templ` views with compile-time embedded `style.css` in `internal/pages/<page>/`.
- Embedded `rawcontent/book-i.md` and parsed with Goldmark (safely omitting raw HTML).
- Created `magefile.go` providing `mage run` (emulates `cargo run`), `mage build`,
  `mage test`, `mage generate`, and `mage clean`.
- Added unit and integration tests in `internal/app/server_test.go` covering all
  routes, stylesheet delivery, 404/500/504 error handling, panic recovery, and
  30-second timeouts. All tests pass (`ok github.com/oxri/oxri-go/internal/app 0.124s`).

## 2026-11-17 - Adopt Chi router and clean standalone Go repository

- Decision: `D-20261117-004`, recorded in commit `444fe4d715eb470029a6b54d13aeb8f0d42eb056`.
- Implementation: commit `4c9ff8961b85e5586ad1414155e38573314d12d4` (`Implement Chi router, refresh gitignore and update README D-20261117-004`).
- Adopted `github.com/go-chi/chi/v5` for routing, not-found/method-not-allowed
  handling, and middleware composition.
- Removed all legacy Rust artifacts (`Cargo.toml`, `Cargo.lock`, `rust-toolchain.toml`,
  `src/`, and `target/`), making `go-experiment` completely standalone.
- Refreshed `.gitignore` for Go binaries, test artifacts, `.mage/` cache, and
  IDE/OS patterns. Updated `README.md` to document the Go GOTH stack and Mage targets.
- Ran `mage test` and verified all 8 unit and integration tests pass cleanly.

## 2026-11-17 - Promote Go stack to main and preserve F# as fsharp-legacy

- Decision: `D-20261117-005`, recorded in commit `2c5b740f013ee3c390e15f077dcebe982d7a9dfb`.
- Implementation: commit `0e7e81d40fcf935f1ecf9dd90f133c9d22e16da5` (`Update README reflecting Go stack on main D-20261117-005`).
- Promoted the Go GOTH application from `go-experiment` to `main`.
- Archived the original .NET 9 / F# implementation on `fsharp-legacy` (both locally
  and on `origin/fsharp-legacy`).
- Cleaned up obsolete remote `go-experiment` branch on origin.
- Updated `README.md` and project decisions. All 8 tests pass via `mage test`.

## 2026-11-17 - SSH loopback authentication for single admin

- Decision: `D-20261117-006`, recorded in commit `ca2bb26` (`Decide SSH loopback authentication D-20261117-006`).
- Implementation: commit `668cf00` (`Implement SSH loopback authentication and session management D-20261117-006`).
- Implemented single-admin authentication using an embedded loopback SSH server
  (`golang.org/x/crypto/ssh`) listening by default on `127.0.0.1:2222` (configurable
  via `SSH_PORT` or `SSH_ADDR`).
- Authorizes client connections against the embedded author public key
  (`ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAILGeK+yBmZxTOmOW/qnIO9ErziQ7i3KyeBpnt3Ns0sc8 git.dev.noconst@outlook.com`),
  overridable via `ADMIN_SSH_PUBKEY` in the environment.
- Persisted private host key to `.ssh_host_key` (auto-generated, gitignored) to
  preserve stable host identity across server restarts.
- Built `internal/auth/` containing `auth.go` (session management, challenge pairing,
  and SSH server lifecycle), `middleware.go` (`SessionMiddleware`, `RequireAuth`,
  and `IsAuthenticated`), and `auth_test.go` (unit and integration tests verifying
  challenge lifecycles, authorized key verification, and unauthorized key rejection).
- Built quiet, typography-focused `/login` view in `internal/pages/login/` with
  embedded `style.css`, copyable command `ssh -p 2222 <token>@localhost` (with
  Alpine.js clipboard button), and HTMX polling on `/auth/status`.
- Added endpoints `/login`, `/auth/status`, and `/logout`. Upon successful SSH
  verification, the server issues an HTTP-only session cookie (`res_imperii_session`)
  and sends `HX-Redirect: /journals`.
- Updated `/journals` and `/journal-editor` to display authenticated author state
  and sign-in / sign-out links.
- Added `.env.example` documenting all configuration keys.
- Ran `mage test` - all 12 unit and integration tests pass cleanly across
  `internal/app` and `internal/auth`. Binary builds cleanly via `mage build`.

## 2026-11-17 - Landing login flow and CLI instructions

- Decision: `D-20261117-007`, recorded in commit `7451d76` (`Decide landing page login entry and root redirect D-20261117-007`).
- Implementation: commit `0881202` (`Add landing page login entry, clear CLI instructions and root redirect D-20261117-007`).
- Added a quiet, theme-consistent `Log in` (or `Log out` when authenticated) link
  to the landing page navigation stack below Journals in `internal/pages/landing/`.
- Updated default redirect destination upon login approval to `/` so completing
  the SSH loopback verification automatically brings the admin back to the home page.
- Redesigned the `/login` view in `internal/pages/login/` with numbered, crystal-clear
  CLI instructions:
  1. Open your terminal.
  2. Run the copyable command `ssh -p 2222 <token>@localhost` (with one-click "Copy Command" button).
  3. Identity verification completes via OpenSSH or Proton Pass agent, unlocking the tab and returning to `/`.
  Also shows the direct alternative command `ssh -p 2222 admin@localhost`.
- Updated unit and integration tests in `internal/app/server_test.go` verifying the
  landing page login link, authenticated logout state, and root redirect flow.
- Refined SSH terminal response (commit `74a33cf`): removed noisy ASCII banners and
  ensured standard `\r\n` line endings to prevent raw PTY staircase formatting.
- Privacy enhancement (commit `529e191`): removed author email comment from the
  `/login` template, stripped comment from `DefaultAuthorizedKey` and `.env.example`,
  and added an automated regression check ensuring email addresses are never leaked
  into the HTML.
- Ran `mage test` - all 12 tests pass cleanly across `internal/app` and `internal/auth`.

## 2026-11-17 - Production passkey bootstrap and author profile dashboard planning

- Decision: `D-20261117-008`, recorded in commit `941af3c` (`Decide production passkey bootstrap and profile dashboard lifecycle D-20261117-008`).
- Defined the roadmap for WebAuthn passkey enrollment upon production HTTPS deployment:
  1. Register an initial primordial bootstrap passkey to establish first-time administrative ownership on the live domain origin (`rp_id`).
  2. Implement an author profile dashboard to manage credentials, register durable security keys/passkeys (Proton Pass, FIDO2/WebAuthn), and inspect active sessions.
  3. Retire and remove the primordial bootstrap credential once daily credentials are verified in the dashboard.
- Updated `NEXT.md` actionable roadmap. Application codebase remains fully functional under Go GOTH and SSH loopback authentication. All 12 tests pass via `mage test`.

## 2026-11-17 - Twitter/X post embeds in journal media links

- Decision: `D-20261117-009`, recorded in commit `d2c9247` (`Decide Twitter/X post embeds in journal media links D-20261117-009`).
- Implementation: commit `db40949` (`Implement Twitter/X post embeds in journal media links D-20261117-009`).
- Added support for embedding Twitter and X post URLs (`x.com`, `twitter.com`, and mobile variants) in journal media links.
- Implemented client-side parsing of numeric post IDs (`/status/<id>`) in `assets/js/journal-editor.js`.
- Integrated Twitter's official widgets API (`widgets.js` / `twttr.widgets.createTweet`) with privacy Do-Not-Track (`dnt: true`) and dynamic theme matching the active light/dark state.
- Provided a quiet, styled fallback card for offline or script-blocked environments so the post link remains accessible.
- Updated `internal/pages/editor/style.css` so Twitter embeds use dynamic auto-height instead of fixed 16:9 video aspect ratio.
- Updated `internal/pages/editor/editor.templ` field hint, loaded `widgets.js`, and bumped cache versions.
- Added test coverage in `internal/app/server_test.go`. All 12 unit and integration tests pass cleanly via `mage test`.

## 2026-11-17 - Tactile media mounting on journal canvas

- Decision: `D-20261117-010`, recorded in commit `e3003d5` (`Decide tactile media mounting on journal canvas D-20261117-010`).
- Implementation: commit `542f3f7` (`Implement tactile media mounting and transparent canvas embedding D-20261117-010`).
- Removed rigid solid background coloring and letterboxing from `.journal-preview-media`,
  allowing the journal card surface to flow seamlessly with `background: transparent`.
- Styled media items (photos, videos, and Twitter/X embeds) like physical artifacts
  mounted on top of paper, with soft drop shadows (`filter: drop-shadow` / `box-shadow`)
  and consistent rounded corners (`border-radius: 0.55rem` on images/video, native ~14px
  for Twitter embeds).
- Allowed Twitter's rounded-corner cards to float naturally over the canvas without
  harsh rectangular cutouts or contrasting background bleeding.
- Replaced the solid empty media box with an understated dashed placeholder inset.
- Added a `MutationObserver` in `assets/js/journal-editor.js` to re-render embedded
  tweets when the active page theme is toggled.
- Updated unit and integration tests in `internal/app/server_test.go`. All 12 tests pass cleanly via `mage test`.

## 2026-11-17 - Global theme gesture, button standards, and /journals/new canonical route

- Decision: `D-20261117-011`, recorded in commit `8077d14` (`Decide global theme toggle, button standards, and /journals/new D-20261117-011`).
- Implementation: commit `6c83fab` (`Implement global theme toggle, button standards, and /journals/new D-20261117-011`).
- Expanded background double-click theme toggle site-wide across all views in `assets/js/theme-toggle.js` (bumped to `?v=2`), ignoring interactions with interactive elements (`a, button, input, textarea, select, label, code, pre, iframe, video, audio`) and active text selections.
- Standardized button hierarchy into three semantic variants in `assets/css/overrides.css` (bumped to `?v=23`):
  1. `.btn-encouraged` / `.button.is-encouraged`: Primary ink presence for primary call-to-action (Publish).
  2. `.btn-neutral` / `.button.is-neutral`: Quiet border and transparent surface for utility/secondary actions (Copy command, New Entry link).
  3. `.btn-destructive` / `.button.is-destructive`: Understated crimson/rose for destructive or irreversible actions.
- Renamed `/journal-editor` to canonical `/journals/new` and added a 301 permanent redirect from `/journal-editor` in `internal/app/server.go`.
- Redesigned `/journals/new` layout in `internal/pages/editor/style.css` and `editor.templ`:
  - Removed "Live preview" in favor of clean minimal headings (`Compose` and `Preview`).
  - Equalized card width (`repeat(2, minmax(0, 1fr))`) and matched card heights, with `.journal-rendered-content` flex-stretching and scrolling smoothly to align with the compose card.
  - Positioned the Publish button centered at the bottom of the page (`.journal-editor-actions`).
- Updated unit and integration tests in `internal/app/server_test.go` and `internal/auth/auth_test.go` to assert `/journals/new` semantics, 301 redirect from `/journal-editor`, and button classes.
- Updated `README.md` documentation. All 12 tests pass cleanly via `mage test` and server compiles via `mage build`.

## 2026-11-17 - Untrack generated Templ Go files

- Decision: `D-20261117-012`, recorded in commit `8e98196` (`Decide ignore and untrack generated Templ Go files D-20261117-012`).
- Implementation: commit `7428b22` (`Untrack generated Templ Go files and update Clean target D-20261117-012`).
- Added `*_templ.go` to `.gitignore` so generated Go template source files are ignored.
- Untracked all seven `*_templ.go` files from the Git index via `git rm --cached`, eliminating machine-generated clutter and noisy diffs from version control.
- Updated `mage clean` in `magefile.go` to purge generated `*_templ.go` files in addition to `bin/`.
- Tested the workflow: `mage clean` removes all generated Go files; `mage test` and `mage build` automatically regenerate templates on demand via `mg.Deps(Generate)` and pass all tests cleanly.

## 2026-11-17 - Protected pilot route namespace and user settings dashboard

- Decision: `D-20261117-013`, recorded in commit `ceb0ca1` (`Decide protected pilot route namespace and settings dashboard D-20261117-013`).
- Implementation: commit `d8f1de6` (`Implement protected pilot settings dashboard for auth methods D-20261117-013`).
- Designated `/pilot` as the dedicated route namespace for experimental author features, security prototypes, and sandbox environments in Res Imperii.
- Implemented the administrative profile dashboard at `/pilot/user/settings` (`internal/pages/pilot/`), enforced by `auth.RequireAuth(authSvc)`. Unauthenticated access redirects to `/login?next=%2Fpilot%2Fuser%2Fsettings`, and accessing `/pilot` redirects to `/pilot/user/settings`.
- Displayed active loopback SSH credentials and future FIDO2/WebAuthn passkeys (hardware security keys, biometrics, Proton Pass).
- Rendered "Add Passkey", "Add SSH Key", and credential "Remove" controls in a disabled state (`disabled`, `aria-disabled="true"`) accompanied by a clear explanatory callout that WebAuthn passkey enrollment requires a registered domain origin (`rp_id`) and HTTPS transport.
- Added `AuthorizedKeyType()` method to `auth.Service` to query configured key types dynamically.
- Surfaced a quiet "Settings" link in the landing page and journals page navigation stacks when authenticated.
- Added unit and integration tests in `internal/app/server_test.go` (`TestPilotUserSettingsPage`) verifying redirect flows, authenticated rendering, button disabled attributes, and stylesheet delivery.
- Updated `README.md` and `NEXT.md`. All 13 tests pass cleanly via `mage test` and the server compiles via `mage build`.

## 2026-11-20 - Turso database persistence, schema migration, UI constraints, and journal CRUD

- Decision: `D-20261120-001`, recorded in commit `3ed14f6` (`Decide Turso persistence, schema migration, and journal CRUD D-20261120-001`).
- Implementation: commit `1f448f7` (`Implement Turso persistence, schema constraints, and journal CRUD D-20261120-001`).
- Added `*.env` to `.gitignore`, reformatted untracked `db.env` with `TURSO_DATABASE_URL` and `TURSO_AUTH_TOKEN`, and documented both variables in `.env.example`.
- Installed `github.com/tursodatabase/libsql-client-go/libsql` (pure-Go Turso connectivity) and `modernc.org/sqlite` (pure-Go local SQLite/in-memory testing fallback).
- Created `internal/db` package:
  - `LoadEnvFiles` (`internal/db/env.go`) loads `db.env` and `.env` at startup (`cmd/server/main.go`).
  - `Store` (`internal/db/db.go`) connects via `libsql` or local `sqlite`, runs idempotent schema initialization (`EnsureSchema`), and migrated the live Turso `journals` table (`ALTER TABLE journals ADD COLUMN published INTEGER NOT NULL DEFAULT 1 CHECK(published IN (0,1))`).
  - `ValidateInput` enforces schema limits server-side (`media_url` valid http/https URL 1–128 chars, up to 3 tags <= 4 chars each, non-empty Markdown body).
- Extracted shared media mounting into `assets/js/journal-media.js` so editor previews, `/journals` feed cards, and `/journals/{id}` permalinks share identical tactile YouTube, Vimeo, X/Twitter, image, audio, and video rendering.
- Updated `/journals/new` and added `/journals/{id}/edit` (`editor.templ`, `editor.go`, `journal-editor.js`) with live constraint counters (`0 / 128`, `0 / 3`), inline errors, publish/bookmark checkboxes, and submit locking until valid & authenticated.
- Updated `/journals` catalog and added `/journals/{id}` permalink (`journals.templ`, `journals.go`, `journals-catalog.js`):
  - Public visitors see only `published = 1` entries (returns 404 on unpublished permalinks).
  - Logged-in admin sees all entries with status badges, Edit, Bookmark toggle, Publish/Unpublish toggle, Delete, and multi-select batch toolbar (`Publish selected`, `Unpublish selected`, `Delete selected`, `Unpublish all`, `Delete all`).
- Added unit and integration tests across `internal/db/db_test.go` and `internal/app/server_test.go` (`TestJournalPersistenceAndManagement`). All 18 tests pass via `mage test` and binary builds via `mage build`.

## 2026-11-20 - Collage-style journal catalog and settings separation

- Decision: `D-20261120-002`, recorded in commit `cef7af0` (`Decide collage-style journal catalog and settings separation D-20261120-002`).
- Implementation: commit `0b539b7` (`Implement collage-style journal catalog and settings separation D-20261120-002`).
- Removed the `Settings` link (`/pilot/user/settings`) from `/journals`, keeping user settings scoped to the landing page auth navigation stack.
- Transformed `/journals` into a compact multi-column preview collage (`grid-template-columns: repeat(auto-fill, minmax(min(100%, 19.5rem), 1fr))`) displaying tactile compact media previews, tag chips, and up to 280-character plain-text excerpts (`Excerpt280`).
- Implemented accessible stretched-card links (`entry-card-link::after`) so clicking anywhere on a collage card opens its full-page reading view (`/journals/{id}`), while preserving elevated `z-index` for interactive embeds, selection checkboxes, and author controls.
- Added `TestExcerpt280` in `internal/pages/journals/journals_test.go` and updated catalog assertions in `internal/app/server_test.go`. All 19 tests pass via `mage test` and binary builds via `mage build`.
- Queued next-session priority items at the top of `NEXT.md`:
  1. Tag uniqueness per journal entry (`tag1`, `tag2`, `tag3`).
  2. Journals keyword search.
  3. Filter by tag(s).
  4. Fluid catalog layout (`auto-fit` / balanced row filling for low entry counts, avoiding centering or lone trailing cards).
- Recorded `D-20261120-003` (commit `1daf85d`): replaced the old 1 GiB storage cap with up to 90% of Turso's 5 GB free tier (4.5 GB), with an overflow policy prompting the author to either manually delete entries first or auto-evict the earliest unpublished entries (`published = 0` ordered by `created_at ASC, id ASC`).

## 2026-11-20 - Title Case tag schema, uniqueness, autocomplete, search/filter, and fluid catalog

- Decision: `D-20261120-004`, recorded in commit `83e65ba` (`Decide Title Case tag schema, uniqueness, autocomplete, search, and fluid catalog D-20261120-004`).
- Implementation: commit `91d9911` (`Implement Title Case tag schema, uniqueness, autocomplete, search, and fluid catalog D-20261120-004`).
- Upgraded the live Turso `journals` table and `db.EnsureSchema` table-rebuild migration to enforce Title Case (`tagN = upper(substr(tagN, 1, 1)) || lower(substr(tagN, 2))`) and per-entry tag uniqueness (`tag1 != tag2`, `tag1 != tag3`, `tag2 != tag3`) via SQLite `CHECK` constraints.
- Added `FormatTagTitleCase`, duplicate tag rejection in `ValidateInput`, `DistinctTags`, and `ListFiltered` (keyword search + multi-tag filter) in `internal/db/db.go`.
- Added live in-place Title Case auto-capitalization, duplicate tag detection/blocking, and interactive tag autocomplete pills (`#tag-suggestions`, selectable via click or `Tab`) in `assets/js/journal-editor.js` and `internal/pages/editor/editor.templ`.
- Added keyword search (`?q=...`) and toggleable tag filter pills (`?tag=...`) to `/journals` (`internal/pages/journals/journals.templ`, `journals.go`).
- Switched `.journals-feed.is-collage-grid` to a fluid flexbox layout (`display: flex; flex-wrap: wrap;` with `.journal-card.is-collage { flex: 1 1 19.5rem; }`) so low journal counts and trailing row items expand to fill the row naturally without centering or leaving lone narrow cards.
- Updated unit and integration tests in `internal/db/db_test.go` and `internal/app/server_test.go`. All tests pass via `mage test` and binary builds via `mage build`.
