RES IMPERII - PROJECT DECISIONS
===============================

Purpose
-------
This file records user-approved product and implementation decisions so
future agents can keep changes consistent with the project's direction. It is
a rationale log, not a replacement for the source code or canonical prose in
rawcontent/Book-I.txt.

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
- `max_content_db_size` is configured in appsettings.json in bytes. The current
  configured maximum is 1,073,741,824 bytes (1 GiB). If an edition would exceed
  the maximum, journals must be deleted to free content storage. Enforcement
  and deletion flows are not implemented in the current scaffold.
- The repository uses a proprietary license. Keep the owner placeholder in
  LICENSE until the copyright owner's name is supplied.

Book pages
----------
- Book I is served at /book-i and uses a centered, medium-width reading column
  with article-like typography and generous section spacing.
- rawcontent/Book-I.txt is the canonical source text. Preserve its wording
  unless the user explicitly asks for editorial changes; the view may add
  semantic structure such as headings, paragraphs, and lists.
- Provide a reusable Back link at both the beginning and end of the article.
  Its circular left chevron is drawn in CSS to avoid an icon-library dependency.
- The shared Back link lives at Views/Misc/Back.fs so additional pages can use
  the same treatment.
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
- Views/Page.fs owns the common HTML document frame, font, favicon, and shared
  stylesheets. Page-specific content belongs in its own Views module.
- wwwroot/css/overrides.css is layered after Bulma. Document meaningful CSS
  sections and keep project styling there instead of editing minified Bulma.
  Keep editor card surfaces, focus states, breakpoints, and reduced-motion
  behavior consistent with the shared warm-neutral theme.
- Local asset URLs use a ?v=N query parameter as a manual cache buster.
  Increment the relevant URL version when its asset changes so browsers such
  as Edge request the updated file.
- Program.fs registers page routes explicitly. Add a route when a linked page
  is implemented; a link alone does not create its destination.
- oxri.fsproj uses explicit F# compile ordering. List each file after modules
  it depends on and before the pages that consume it.
- appsettings.Development.json currently duplicates the base logging settings.
  It is unnecessary until local-only configuration differs.

Recent decisions
----------------
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
