RES IMPERII - PROJECT DECISIONS
===============================

Purpose
-------
This file records user-approved product and implementation decisions so
future agents can keep changes consistent with the project's direction. It is
a rationale log, not a replacement for the source code or canonical prose in
rawcontent/book-i.md.

How to use this file
--------------------
Record decisions that should guide future work, together with their rationale
and current implementation status where relevant. Keep the handoff timeline in
HANDOFF.md and the actionable queue in NEXT.md. When a decision changes, add the
new direction and explain what it supersedes instead of silently rewriting the
historical rationale.

Starting with new records, give each decision a stable ID in the form
D-YYYYMMDD-NNN and include its rationale, scope, and any decision it supersedes.
See the commit-traceability decision below for how to link decisions to Git.

Product direction
-----------------
- ALL INPUTS ARE EVIL. Treat every user-supplied or external input (form fields,
  URLs, query parameters, tags, and prose) as untrusted at both the UI layer and
  the server layer. Enforce strict character-set whitelisting permitting only
  English alphanumeric, Latin, and Greek characters (plus standard prose/Markdown
  punctuation where applicable), and reject emojis, control characters, zero-width
  / bidi spoofing codepoints, and non-whitelisted scripts.
- Avoid monoliths. Keep the codebase structured as small, focused modules with
  clear single responsibilities (e.g., separate route wiring from domain route
  handlers, isolate shared CSS/JS primitives from page-specific views).
- Minimalism is a deliberate constraint. Keep the interface quiet and focused;
  avoid adding decoration, controls, or explanatory copy without a clear need.
- EB Garamond is the primary typeface. It is loaded from Google Fonts in the
  shared page frame and selected through Bulma's --bulma-family-primary token.
- The title and article headings use regular weight. The user prefers thin,
  restrained typography over bold emphasis.
- The shared palette follows the operating system's color preference. Light
  mode uses very subtle off-white (#f5f4f0) and muted dark text (#34332f). Dark
  mode uses near-black (#0f1113), faintly warm, with soft light text (#d6d6d3).
  Orange should remain extremely subtle, in the spirit of Ayu Dark.
- On the landing page, double-clicking the empty background toggles an explicit
  light/dark override for the current browser tab. Use session storage so the
  choice survives navigation and reloads in that tab, then expires with the
  tab session. The operating-system preference remains the default until the
  visitor chooses a theme; clicks on navigation or page content do not toggle it.
- Respect reduced-motion preferences for animated interactions.

Landing page
------------
- The landing page is a minimal navigation list in a right-side column, placed
  above the vertical midpoint. The title is plain, non-clickable text.
- Book I, Book II, Book III, and Journals are links. Link text scales only
  slightly on hover or keyboard focus. Keep the divider very low contrast.
- The Book II, Book III, and Journals destinations are currently placeholders;
  add their routes when those pages are implemented.

Publication model
-----------------
- Res Imperii is a private blog for the author's articles and journals.
- A journal stores its content and a link to a third-party renderable version.
  The author can edit, hide or show, and delete journal entries.
- Turso is the intended hosted content database. The current application has
  not yet implemented its connection or journal management flows.
- The product maximum content database size is 1,073,741,824 bytes (1 GiB). If
  an edition would exceed the maximum, journals must be deleted to free content
  storage. The Rust experiment has not selected database configuration or
  implemented enforcement and deletion flows.
- The repository uses a proprietary license. Keep the owner placeholder in
  LICENSE until the copyright owner's name is supplied.

Book pages
----------
- Book I is served at /book-i and uses a centered, medium-width reading column
  with article-like typography and generous section spacing.
- `rawcontent/book-i.md` is the canonical Book I source. It is tracked because
  the author has chosen to ship it from the private repository. Preserve its
  wording unless explicitly asked for editorial changes; the view may render
  its Markdown structure as semantic HTML.
- Provide a reusable Back link at both the beginning and end of the article.
  Its circular left chevron is drawn in CSS to avoid an icon-library dependency.
- The Rust page frame provides a shared Back link helper in
  `src/pages/shared.rs` so article and journal pages can use the same treatment.
- The browser-side journal editor prototype uses two panes: editable
  source and a rendered preview card. The left pane collects Markdown content,
  a media URL, and comma-separated tags; the right pane previews those fields
  as the published card would display them, with media centered above the body.
  It does not write drafts to storage. The route is available in all
  environments; keep publishing disabled until authentication and persistence
  are implemented.
- The editor preview renders Markdown with raw HTML disabled and sanitizes the
  generated markup before inserting it into the page. Tags are inserted as text.
- Media previews are browser-side and limited to recognizable YouTube and Vimeo
  links plus direct image, video, and audio files. Provider embeds are built from
  validated video IDs; arbitrary iframe URLs are not accepted. Unsupported URLs
  show a fallback message. Previewing remote media asks the visitor's browser
  to load it from the provider or host.
- The Publish entry button remains disabled while authentication and persistence
  are absent. Do not enable it until there is a real, protected save workflow.
- The /journals page currently has an empty state and an Add New Entry link to
  the browser-side editor prototype. Both routes are available in all
  environments.

Implementation conventions
---------------------------
- `src/main.rs` owns application startup and explicit Axum routes. Add a route
  when a linked page is implemented; a link alone does not create its
  destination.
- Each route lives in `src/pages/<page>/mod.rs`, beside its `style.css`. A page
  module serves its stylesheet from an embedded compile-time string at a
  versioned `/page-css/...` route. This keeps page logic and presentation
  together without exposing Rust source files through the static server.
- `src/pages/shared.rs` owns the common Maud document frame. Shared public files
  live under `assets/`: Bulma and `overrides.css` in `assets/css/`, shared
  browser scripts in `assets/js/`, and the favicon at `assets/favicon.png`.
  Keep shared project styles in `overrides.css`, not in minified Bulma.
- Keep editor surfaces, focus states, breakpoints, and reduced-motion behavior
  consistent with the shared warm-neutral theme.
- Local asset URLs use a `?v=N` query parameter as a manual cache buster.
  Increment the relevant URL version when changing an asset.
- This branch is a standalone Rust application; do not retain the .NET/F#
  project, generated build output, or duplicate source asset tree here. The
  `main` branch remains the home of the .NET application.

Recent decisions
----------------
- D-20261120-006 - Modular handler split, race-free timeout buffering, HTTP hardening, and shared media CSS deduplication:
  - Enforce the architectural rule: do not create monoliths; organize code into
    small, focused modules (`server.go` for router wiring, `handlers_journals.go`,
    `handlers_auth.go`, and `handlers_pilot.go` for domain handlers).
  - Fix the concurrent `ResponseWriter` data race in `TimeoutMiddleware` by
    buffering handler output inside a mutex-guarded `timeoutWriter`, and log
    opaque error reference codes (`ref`) with their internal cause privately to
    `os.Stderr` in `RenderErrorWithCause`.
  - Harden the HTTP server with explicit `ReadHeaderTimeout`, `ReadTimeout`,
    `WriteTimeout`, `IdleTimeout`, a 1 MiB request body cap (`MaxBodyBytesMiddleware`),
    and cross-origin `POST` protection (`CrossOriginMiddleware` checking `Sec-Fetch-Site`
    and `Origin`).
  - Deduplicate shared tactile media CSS rules (`.journal-preview-media`,
    `.journal-media-slot`, `.tweet-embed-container`, `.tweet-fallback-card`) into
    `assets/css/overrides.css`.
- D-20261120-005 - "ALL INPUTS ARE EVIL" input sanitization and Latin/Greek/alphanumeric character-set whitelist:
  - Adopt the core security axiom `"ALL INPUTS ARE EVIL"` in `DECISIONS.md` and
    `README.md`.
  - Enforce strict character-set whitelisting at both the UI layer (`journal-editor.js`)
    and the server layer (`db.ValidateInput` and query-parameter sanitizers):
    - Tags (`tag1..tag3`): only English/Latin (`\p{Latin}`), Greek (`\p{Greek}`),
      and ASCII digits (`0–9`). No punctuation, spaces, or emojis.
    - Media URL (`media_url`): only printable ASCII URL characters (`0x21–0x7E`)
      with `http://` or `https://` scheme. No emojis, whitespace, or non-ASCII
      homographs.
    - Entry prose (`body`) and search queries (`q`): only English/Latin letters,
      Greek letters, ASCII digits, standard whitespace (`space, \n, \r, \t`), and
      standard ASCII/typographic prose & Markdown punctuation. Explicitly block
      all emojis, variation selectors, control characters, zero-width/bidi
      spoofing characters, and non-Latin/non-Greek scripts.
    - Never render or echo disallowed characters in the live preview pane or
      validation notices (strip disallowed characters before previewing and
      report offending characters by Unicode codepoint `U+XXXX`).
- D-20261120-004 - Title Case tag schema constraint, per-entry tag uniqueness, tag autocomplete, search/filter, and fluid catalog layout:
  - Upgrade the `journals` table schema via idempotent table-rebuild migration to
    enforce Title Case (`tagN = upper(substr(tagN, 1, 1)) || lower(substr(tagN, 2))`)
    and per-entry tag uniqueness (`tag1 != tag2`, `tag1 != tag3`, `tag2 != tag3`)
    at the SQLite `CHECK` level.
  - Auto-capitalize tags into Title Case and reject duplicate tags in both the
    client-side editor (`journal-editor.js`) and server-side validation (`db.ValidateInput`).
  - Provide live tag autocomplete suggestions in `/journals/new` and `/journals/{id}/edit`
    populated from `SELECT DISTINCT tag` across existing `tag1..tag3` columns.
  - Add keyword search (`q`) and multi-tag filtering (`tag`) on `/journals`, paired
    with a fluid flexbox collage layout (`flex: 1 1 19.5rem`) so low entry counts
    and trailing row items fill the row naturally without gaps or centering.
- D-20261120-003 - Turso 90% free-tier storage budget and earliest-unpublished eviction policy:
  - Supersede the earlier 1 GiB storage limit with up to 90% of Turso's free-tier
    storage capacity (4.5 GB out of the 5 GB free tier).
  - When database usage exceeds the 90% threshold during entry creation or update,
    prompt the author to either manually delete existing entries first or
    automatically delete the earliest unpublished entries (`published = 0` ordered
    by `created_at ASC, id ASC`) to reclaim capacity.
- D-20261120-002 - Collage-style compact journal catalog and navigation separation:
  - Remove `Settings` (`/pilot/user/settings`) navigation link from `/journals`,
    keeping user settings isolated on the landing page auth navigation stack.
  - Transform `/journals` from a full-prose feed into a responsive multi-column
    preview collage displaying compact tactile media previews, up to 280-character
    plain-text excerpts, and tag chips.
  - Link each collage card directly to its full-page entry view (`/journals/{id}`)
    via an accessible stretched link while keeping interactive embeds, checkboxes,
    and admin actions directly clickable.
- D-20261120-001 - Turso persistence, schema migration, UI constraints, and journal CRUD:
  - Ignore `*.env` files in `.gitignore` and load `db.env` / `.env` environment
    files on startup (`internal/db`). Document `TURSO_DATABASE_URL` and
    `TURSO_AUTH_TOKEN` in `.env.example`.
  - Connect to Turso using `github.com/tursodatabase/libsql-client-go/libsql`, with
    pure-Go `modernc.org/sqlite` as a local `file:` and `:memory:` fallback for
    offline development and unit tests.
  - Extend the `journals` table safely via idempotent startup migration (`EnsureSchema`)
    by adding `published INTEGER NOT NULL DEFAULT 1 CHECK(published IN (0,1))` so
    entries can be unpublished without deletion.
  - Enforce database schema constraints in both client-side UI (`/journals/new`,
    `/journals/{id}/edit`) and server-side validation:
    - `media_url`: required valid `http`/`https` URL, 1–128 characters.
    - `tag1`, `tag2`, `tag3`: max 3 comma-separated tags, each <= 4 characters.
    - `body`: required non-empty Markdown content.
  - Implement public catalog reading (`/journals`, `/journals/{id}`) showing
    published entries, and protected author management (`/journals/new`,
    `/journals/{id}/edit`, single/selected/all publish, unpublish, bookmark,
    and delete actions).
- D-20261117-013 - Protected pilot route namespace and user settings dashboard:
  - Designate `/pilot` as the dedicated protected namespace for experimental author
    features, security prototypes, and sandbox environments in Res Imperii.
  - Introduce an administrative profile dashboard at route `/pilot/user/settings`,
    enforced by `auth.RequireAuth(authSvc)`. If accessed directly, `/pilot` provides
    an index of experimental modules (or routes to `/pilot/user/settings`).
  - Display configured and available authentication methods (current active loopback
    SSH key, and future FIDO2/WebAuthn passkeys for hardware keys, biometrics, and
    Proton Pass).
  - Expose "Add Passkey", "Add SSH Key", and credential "Remove" controls in a
    disabled state (`disabled`, `aria-disabled="true"`) with an informative callout
    explaining that WebAuthn credential enrollment requires a public domain and HTTPS.
  - Surface a `Settings` link on the landing page auth stack when authenticated.
- D-20261117-012 - Ignore and untrack generated Templ Go files:
  - Add `*_templ.go` to `.gitignore` and untrack all generated `*_templ.go` files
    from the repository index.
  - Mage handles template compilation on demand across `Generate`, `Build`, `Run`,
    and `Test` targets; committing generated Go code creates noisy diffs, potential
    merge conflicts, and pollutes Git history with machine-generated artifacts.
  - Update `mage clean` to purge generated `*_templ.go` files alongside `bin/`.
- D-20261117-011 - Global theme gesture, button standards, and editor refinement:
  - Expand background double-click theme toggle site-wide across all views,
    ignoring interactions with links, buttons, inputs, pre/code blocks, and text selections.
  - Standardize button hierarchy into three semantic variants in `overrides.css`:
    encouraged (`.btn-encouraged` / `.button.is-encouraged`), neutral (`.btn-neutral` /
    `.button.is-neutral`), and destructive (`.btn-destructive` / `.button.is-destructive`).
  - Rename route `/journal-editor` to canonical `/journals/new` with a 301 redirect
    from `/journal-editor`.
  - In `/journals/new`, eliminate the phrase "Live preview" in favor of minimal
    headings (`Compose` and `Preview`). Equalize the width and height of the two
    cards, stretching and scrolling the rendered journal body preview to match
    the compose card.
  - Position the Publish entry action at the bottom of the page, centered horizontally.
- D-20261117-010 - Tactile media mounting on journal canvas:
  - Remove rigid rectangular background coloring and square letterboxing from
    the journal preview media area (`.journal-preview-media`).
  - Present media items (photos, videos, audio, and Twitter cards) as physical
    artifacts mounted/stapled directly onto the journal card canvas.
  - Apply soft physical drop shadows (`box-shadow` / `filter: drop-shadow`) and
    rounded corners (`border-radius: 0.55rem` for photos/video, native ~14px for
    Twitter embeds) so rounded embeds float seamlessly over the canvas without
    corner artifacts or contrasting square cutout borders.
  - When empty, replace the solid block with an understated dashed mount placeholder.
- D-20261117-009 - Twitter/X post embeds in journal media links:
  - Expand browser-side media previews in journal links to support Twitter and X
    post URLs (`twitter.com`, `x.com`, and their mobile variants).
  - Parse and validate numeric post IDs (`/status/<id>`) from recognizable paths.
    Arbitrary iframe URLs are not accepted.
  - Embed posts via Twitter's official widgets API (`widgets.js` / `twttr.widgets.createTweet`),
    passing `dnt: true` to respect reader privacy and matching the embed theme
    (`dark` or `light`) to the active page theme.
  - Provide a quiet fallback card with post metadata and direct link if widgets.js
    is blocked, fails to load, or the client is offline.
  - Update `.journal-preview-media` styles so Twitter embeds use dynamic auto-height
    instead of the 16:9 aspect ratio reserved for video players.
- D-20261117-008 - Production passkey bootstrap and author profile dashboard:
  - Plan the post-deployment WebAuthn lifecycle for production (under HTTPS):
    1. Bootstrap administrative access on the live domain using an initial
       primordial passkey.
    2. Build an author profile dashboard to manage credentials, register durable
       passkeys (e.g. in Proton Pass or security keys), and inspect active sessions.
    3. Once durable production credentials are enrolled and verified, remove the
       primordial bootstrap passkey to finalize credential hardening.
- D-20261117-007 - Landing page login entry and root landing redirect:
  - Add a quiet, theme-consistent `Log in` (or `Log out` when authenticated) link
    to the landing page navigation stack below Journals.
  - Set the default post-authentication redirect destination to `/` so completing
    SSH loopback verification immediately lands the admin back on the home page.
  - Present explicit step-by-step CLI instructions on `/login` (command syntax,
    port flag `-p 2222`, token argument, and automatic tab unlocking).
- D-20261117-006 - SSH loopback authentication for single admin:
  - Implement single-admin authentication using an embedded loopback SSH server
    (`golang.org/x/crypto/ssh`) listening on a local port (default `:2222`,
    configurable via `SSH_PORT` or `SSH_ADDR`).
  - WebAuthn passkeys are deferred until a custom domain and HTTPS are provisioned,
    as browser passkey managers (including Proton Pass) strictly require `https:`
    and bind credentials cryptographically to domain origins (`rp_id`).
  - The server authorizes client connections against an embedded default public
    key (`ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAILGeK+yBmZxTOmOW/qnIO9ErziQ7i3KyeBpnt3Ns0sc8 git.dev.noconst@outlook.com`),
    overridable at runtime via the `ADMIN_SSH_PUBKEY` environment variable.
  - The SSH server persists an auto-generated private host key to `.ssh_host_key`
    (gitignored) to ensure stable host identity across server restarts and avoid
    OpenSSH host identification warnings.
  - The `/login` page displays a short ephemeral challenge code (valid 5 minutes)
    and copyable command `ssh -p 2222 <token>@localhost`, while polling `/auth/status`
    via HTMX every second. Running the command presents the admin's SSH key
    (stored in OpenSSH or Proton Pass agent) to the server.
  - Upon cryptographic verification of the public key, the SSH server marks the
    challenge session approved and cleanly exits. The browser's next poll consumes
    the approved challenge, sets a secure HTTP-only session cookie (`res_imperii_session`),
    and redirects the admin via `HX-Redirect` to `/journals`.
  - Protected routes verify the session cookie via `AuthMiddleware`. Visiting
    `/logout` invalidates the active session and clears the cookie.
- D-20261117-005 - Promote Go GOTH stack to main and preserve F# as fsharp-legacy:
  - Promote the Go GOTH application (Go 1.27, Chi, Templ, HTMX, Alpine, Bulma)
    to `main`.
  - Archive the original .NET 9 / F# implementation on `fsharp-legacy`.
  - Update `README.md` to reflect `main` as the canonical Go application.
- D-20261117-004 - Adopt Chi router and clean standalone Go repository:
  - Adopt `github.com/go-chi/chi/v5` for routing and middleware composition.
  - Remove legacy Rust artifacts (`Cargo.toml`, `Cargo.lock`, `rust-toolchain.toml`,
    `src/`, `target/`), making `go-experiment` a clean, standalone Go project.
  - Refresh `.gitignore` for Go binaries (`/bin/`, `*.exe`), test artifacts,
    Mage cache (`/.mage/`), and IDE/OS ignore patterns.
  - Update `README.md` to document the Go application structure and Mage targets.
- D-20261117-003 - Parallel Go GOTH experiment:
  - Explore the GOTH stack (Go, Templ, HTMX, Alpine.js, and Bulma) on the
    `go-experiment` branch.
  - Structure the Go application following standard project layout hygiene:
    `cmd/server/main.go` for the entrypoint, `internal/app/` for server and
    middleware, and `internal/pages/<page>/` for colocated `.templ` views and
    `style.css` stylesheets embedded via `//go:embed`.
  - Provide a `magefile.go` task runner to emulate `cargo run` (`mage run`),
    `cargo build` (`mage build`), `cargo test` (`mage test`), and `templ generate`.
  - Port all existing routes, Markdown rendering (via Goldmark), branded error
    pages with QR codes and reference IDs, and request timeout/panic middleware.
- D-20261117-002 - Adopt Alpine.js for client-side state alongside HTMX:
  - Permit Alpine.js for client-side manipulation (e.g. reactive UI, local
    state, and dynamic controls) provided it interoperates cleanly with HTMX.
  - Alpine 3.x relies on MutationObserver to automatically discover and
    initialize new `x-data` elements within HTMX-swapped fragments.
  - Include Alpine.js 3.x (`defer`) in `src/pages/shared.rs` after HTMX so it
    is available across application views.
- D-20261117-001 - Restore native navigation and shared article measure:
  - Remove `hx-boost` from landing page navigation. HTMX body swapping bypassed
    `<head>` stylesheet updates, leaving `/book-i` and `/journals` without their
    page CSS on navigation from the landing page. It also suppressed 4xx/5xx
    responses (preventing the branded 404 error page from displaying on
    unimplemented routes like `/book-ii`) and intercepted clicks before
    `navigation-feedback.js` and cross-document view transitions could run.
  - Preserve native browser navigation for same-origin routes so each page's
    embedded stylesheet loads immediately, 4xx/5xx error responses display their
    branded pages, and cross-document view transitions work cleanly.
  - Add `.article-page` and `.book-article` layout rules (max 42rem, centered)
    to shared `overrides.css` so Book I and any future book-n articles reliably
    maintain a centered, Medium-like reading column.
  - Bump `overrides.css` cache version in `src/pages/shared.rs`.
- D-20261116-004 - Ship Book I source and restore shared page behavior:
  - Track `rawcontent/book-i.md` in the private repository and render Book I
    from this source. Keep the source wording canonical.
  - Every application error response uses the branded error page with its HTTP
    status, an opaque reference code, and a QR code encoding only that code.
    This includes 403, 404, 408, 504, and 500 responses. Never include an
    exception message or stack trace; use a plain safe fallback if rendering
    the styled page fails.
  - Bound application request work to 30 seconds and return the same safe page
    with status 504 when the deadline expires.
  - Render Back links through the shared helper with its CSS chevron and label.
    Keep Book I and Journals markup aligned with their page-local stylesheets.
- D-20261116-003 - Rust page and asset layout:
  - Organize each Rust page as `src/pages/<page>/mod.rs` with its page-only
    `style.css` beside it. Embed page CSS in the binary and serve it through
    explicit versioned stylesheet routes; do not expose source directories as
    static files.
  - Keep shared browser assets in `assets/`: `assets/css/` contains Bulma and
    `overrides.css`, `assets/js/` contains shared scripts, and the favicon is
    `assets/favicon.png`.
  - Remove the F#/Oxpecker project and generated .NET artifacts from
    `rust-experiment`, making it a clean, standalone Rust application. Preserve
    the source content and project journals. The .NET project remains unchanged
    on `main`.
  - This supersedes D-20261115-004's `Views`/`wwwroot` file placement for this
    Rust branch; it does not change the .NET branch's layout.
- D-20261116-002 - Parallel Rust MASH experiment:
  - Explore Maud, Axum, SQLx, and HTMX on the `rust-experiment` branch while
    leaving the F#/Oxpecker application on `main` unchanged.
  - Begin without a database driver or rustls provider so the first prototype
    does not require bundled SQLite's C build or ring's Windows ARM64 Clang
    toolchain. Choose SQLx database and TLS features when persistence is
    implemented; Windows-native SChannel remains an option if SQLx needs TLS.
- D-20261116-001 - Journal prototype routes in all environments:
  - Register `/journals` and `/journal-editor` in release as well as
    Development. They are implemented pages, not development-only previews.
  - Keep the editor browser-only, do not persist drafts, and leave Publish
    disabled until authentication and persistence are implemented.
- D-20261115-007 - Theme-aware pending navigation feedback:
  - Keep pending-navigation dimming confined to dark mode, including the
    reduced-motion fallback. Light-mode readers should not receive a darkening
    flash while waiting for a destination.
  - Set the cross-document view-transition layer to the active page canvas
    color, so fading page snapshots do not expose a black backdrop in light
    mode. This refines the feedback introduced by D-20261115-006.
- D-20261115-006 - Pending navigation feedback:
  - Use the browser's cross-document view transition for a brief fade between
    same-origin pages where supported; normal navigation remains the fallback.
    Disable the transition animation for reduced-motion preferences.
  - Keep the current page visible and readable while a destination is pending.
    If same-origin link navigation has not completed after a brief delay, apply a
    subtle breathing dim until the destination loads. Use a steady subtle dim
    under reduced-motion preferences, and clear the state if navigation stalls.
- D-20261115-005 - Non-selectable landing page:
  - Disable text selection across the landing page so double-clicking near its
    navigation does not select the nearest text. Keep links interactive and
    keyboard accessible; this changes selection behavior only.
- D-20261115-004 - Route-specific stylesheets:
  - Keep theme tokens and components shared by multiple views in
    wwwroot/css/overrides.css. Move route-only layout and presentation rules to
    wwwroot/css/pages/<route>.css so changes remain local to their page.
  - Make Views.Page.frame take the route stylesheet path, link it after the
    shared stylesheet, and pass that path from each page view. Keep the manual
    cache-busting version in the path argument.
- D-20261115-003 - Minimal error page and QR payload:
  - Use one quiet error-page layout for application-generated 404, 408, 504,
    and 500 responses. Keep the HTTP status visible and the message generic.
  - Show the request's opaque TraceIdentifier as a reference code. The QR code
    encodes only this same reference; it contains no URL, query string, or
    exception detail. This keeps the QR self-contained until a report endpoint
    exists and allows later private logs to use the reference for lookup.
  - Bound application endpoint requests to 30 seconds and render a 504 page when
    that limit expires. Hosting-provider or reverse-proxy errors generated
    before the request reaches the app need matching configuration there.
  - Use the standard HTTP status in the response and never expose exception
    text or stack traces, including in Development.
- D-20261115-001 - Commit-linked decision and implementation history:
  - Record a decision here before implementing work that depends on it, and
    commit the decision record separately when practical.
  - Reference the decision ID in the implementation commit. Git creates that
    commit's hash locally, before any push is needed.
  - Add the decision ID and the decision and implementation commit hashes to
    the next HANDOFF.md entry. This gives each decision a direct path to the
    rationale and the code it affected.
  - Keep decision and implementation commits separate so the implementation
    can be reverted without deleting its rationale. Use `git revert <hash>` to
    undo committed work while preserving the history; add a new decision record
    when changing direction.
- Replaced the landing placeholder with the sparse navigation composition.
- Switched the primary font to EB Garamond using Bulma's family variable and a
  Google Fonts stylesheet link.
- Added light and dark theme colors in CSS properties and mapped Bulma's main
  surface and text tokens to those properties.
- Added Book I from the raw content and registered its route.
- Refined Back into a reusable view module and CSS-drawn circular chevron.
- Added Development-only /journals and /journal-editor prototypes. The catalog
  is an empty state; the editor has a responsive two-card layout, live Markdown
  preview, comma-separated tag chips, and browser-side previews for common media
  links. Publish remains disabled and no draft is saved.
- Added a landing-background double-click gesture for a tab-scoped theme choice;
  the system theme remains the default and the session override applies across
  pages.
- Adopted a three-document workflow: NEXT.md holds upcoming work, DECISIONS.md
  holds durable rationale, and HANDOFF.md records append-only session handoffs
  tied to Git commits.

When unsure
-----------
Preserve the minimal composition, warm-neutral palette, regular-weight type,
canonical raw content, and shared page components. Ask only when a proposed
change would alter the source text's meaning or the established visual direction.
