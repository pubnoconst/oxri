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
- `/book-i` displays Book I, whose source text is in `rawcontent/Book-I.txt`.
- `/journal-editor` is a Development-only, browser-side editor prototype. It
  has an editable source pane and a rendered preview pane, but does not save drafts.
- `/journals` is currently a Development-only empty state with an Add New Entry
  link to the editor prototype.
- Book II, Book III, and Journals are navigation placeholders; their routes
  have not been implemented yet.

## Run locally

Install the .NET SDK that supports the target framework in `oxri.fsproj`, then
run:

```powershell
dotnet restore
dotnet run
```

ASP.NET Core loads `appsettings.json` and, when running in Development, also
loads `appsettings.Development.json`. Keep credentials out of committed
configuration files; no Turso credentials or connection settings are currently
implemented in this scaffold.

## Project notes

- `Views/Page.fs` defines the shared document frame, typography, and stylesheets.
- `Views/Landing.fs` and `Views/BookI.fs` define the current page views.
- `Views/Misc/Back.fs` contains the reusable Back link.
- `wwwroot/` contains public static assets. Local asset URLs use a `?v=N`
  query version; increment the version when changing an asset to refresh browser
  caches.
- `project-decicions-journal.txt` records design decisions and their rationale
  for future contributors.
- `rawcontent/` is excluded from version control by `.gitignore`; preserve the
  author's source material and do not change that policy without direction.

## License

This is proprietary software. See [LICENSE](LICENSE) for the usage terms.
