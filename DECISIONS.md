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
- The Development-only journal editor prototype uses two panes: editable
  source and a rendered preview card. The left pane collects Markdown content,
  a media URL, and comma-separated tags; the right pane previews those fields
  as the published card would display them, with media centered above the body.
  It does not write drafts to storage; keep it off production routes until
  authentication and persistence are implemented.
- The editor preview renders Markdown with raw HTML disabled and sanitizes the
  generated markup before inserting it into the page. Tags are inserted as text.
- Media previews are browser-side and limited to recognizable YouTube and Vimeo
  links plus direct image, video, and audio files. Provider embeds are built from
  validated video IDs; arbitrary iframe URLs are not accepted. Unsupported URLs
  show a fallback message. Previewing remote media asks the visitor's browser
  to load it from the provider or host.
- The Publish entry button remains disabled while authentication and persistence
  are absent. Do not enable it until there is a real, protected save workflow.
- The Development-only /journals page currently has an empty state and an
  Add New Entry link to the Development-only editor prototype.

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
- Adopted a three-document workflow: NEXT.md holds upcoming work, DECISIONS.md
  holds durable rationale, and HANDOFF.md records append-only session handoffs
  tied to Git commits.

When unsure
-----------
Preserve the minimal composition, warm-neutral palette, regular-weight type,
canonical raw content, and shared page components. Ask only when a proposed
change would alter the source text's meaning or the established visual direction.
