# Res Imperii

Res Imperii is a private blog where the author publishes articles and
journals. Articles are long-form works presented on the site. A journal entry
contains the entry's content and a link to a third-party renderable version of
that content. The author can edit a journal, hide or show it, and delete it.

The project is built with ASP.NET Core, F#, and Oxpecker. Bulma provides the
CSS foundation, with project styles in `wwwroot/css/overrides.css`.

## Content storage

The planned content database is hosted by [Turso](https://turso.tech/). The
author controls the published journals, including whether each one is visible
and whether it should be deleted.

The configured maximum content database size is `max_content_db_size` in
`appsettings.json`. Its value is expressed in bytes and is currently
`1073741824` bytes (1 GiB). If storing an edition would exceed this limit, the
author must delete journals to make room.

The current application scaffold does not yet connect to Turso, implement
journal management, or enforce the configured size limit. Those behaviors are
the intended product design and require implementation before they are active.

## Current pages

- `/` displays the landing page and links to the planned book and journal areas.
  Double-clicking its empty background toggles light/dark mode for the current
  browser tab; the operating-system preference remains the default.
  Same-origin page changes use a brief fade where supported; pending-navigation
  dimming is limited to dark mode, and the transition backdrop follows the
  active page color.
- `/book-i` displays Book I, whose source text is in `rawcontent/Book-I.txt`.
- `/journal-editor` is a browser-side editor prototype available in all
  environments. It has an editable content pane with media-link and tags
  fields, plus a live rendered preview. Drafts are not saved, and publishing is
  disabled.
- `/journals` is an empty state with an Add New Entry link to the editor
  prototype, available in all environments.
- Error responses use a minimal page with the HTTP status and an opaque request
  reference. Its QR code contains only that reference; exceptions never appear
  in the response.
- Book II, Book III, and Journals are navigation placeholders; their routes
  have not been implemented yet.

## Run locally

Install the .NET SDK that supports the target framework in `oxri.fsproj`, then
run:

```powershell
dotnet restore
dotnet run
```

## Rust MASH experiment

The `rust-experiment` branch contains a parallel prototype using Maud, Axum,
SQLx, and HTMX. It preserves the current page routes and visual assets while
leaving the .NET application on `main` as the canonical implementation. The
SQLx dependency has no database driver or TLS backend enabled yet; choose those
alongside the database before implementing persistence.

The branch pins the Rust channel to stable for the current Windows ARM64 target.
Run it with `cargo run` once the Rust toolchain and Visual Studio C++ Build
Tools are installed. The Rust linker requires the MSVC linker (`link.exe`), even
though this initial feature set does not require Clang or a C compiler. Adding
SQLx SQLite or a rustls `ring` backend may introduce additional native build
requirements and should be evaluated separately.

ASP.NET Core loads `appsettings.json` and, when running in Development, also
loads `appsettings.Development.json`. Keep credentials out of committed
configuration files; no Turso credentials or connection settings are currently
implemented in this scaffold.

## Project notes

- `Views/Page.fs` defines the shared document frame, typography, and stylesheets.
- `Views/Error.fs` defines the shared 4xx/5xx error page.
- `wwwroot/css/overrides.css` contains shared theme and component styles;
  `wwwroot/css/pages/` contains stylesheets for individual routes.
- `wwwroot/js/theme-toggle.js` restores the tab's theme choice on each page and
  handles the landing-page background gesture.
- `Views/Landing.fs` and `Views/BookI.fs` define the current page views.
- `Views/Misc/Back.fs` contains the reusable Back link.
- `wwwroot/` contains public static assets. Local asset URLs use a `?v=N`
  query version; increment the version when changing an asset to refresh browser
  caches.
- `DECISIONS.md` records product and implementation decisions and their rationale.
- `NEXT.md` is the short list of upcoming work.
- `HANDOFF.md` is an append-only session log. Each entry points to the commit
  containing the work being handed off, so Git remains the source of truth for
  code changes.
- `rawcontent/` is excluded from version control by `.gitignore`; preserve the
  author's source material and do not change that policy without direction.

## License

This is proprietary software. See [LICENSE](LICENSE) for the usage terms.
