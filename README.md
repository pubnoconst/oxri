# Res Imperii

Res Imperii is a private blog for long-form articles and journals. The
`go-experiment` branch is a standalone Go application using the GOTH stack:
Go, Templ, HTMX, and Tailwind/Bulma (Bulma + overrides in this project), paired
with Alpine.js and Chi routing. The Rust experiment remains on `rust-experiment`
and the .NET/F# application remains on `main`.

## Project structure

```text
assets/
  css/
    bulma.min.css
    overrides.css
  js/
    journal-editor.js
    navigation-feedback.js
    theme-toggle.js
  favicon.png
cmd/
  server/
    main.go
internal/
  app/
    server.go
    middleware.go
    server_test.go
  pages/
    shared/
      layout.templ
    landing/
      landing.templ
      style.css
    booki/
      booki.templ
      booki.go
      style.css
    journals/
      journals.templ
      style.css
    editor/
      editor.templ
      style.css
    errorpage/
      error.templ
      error.go
      style.css
rawcontent/
  book-i.md
  booki.go
magefile.go
go.mod
go.sum
```

Each page's Templ view and page-only stylesheet sit together in
`internal/pages/<page>/`. Page modules embed `style.css` at compile time with
`//go:embed` and serve it through an explicit, versioned `/page-css/...` route.
This keeps page styling easy to find without exposing source files through the
static file server.

Shared public files live in `assets/`: Bulma and shared project overrides are
under `assets/css/`, browser scripts are under `assets/js/`, and the favicon is
at `assets/favicon.png`. The `/assets` route serves only this directory. Keep
shared styling in `overrides.css`; leave minified Bulma untouched. Bump a local
asset's `?v=N` query version when changing its contents.

## Run locally

The project uses [Mage](https://magefile.org/) as the task runner, emulating `cargo run`:

```powershell
# Emulate cargo run (generates templates and launches the server)
mage

# Or run tests
mage test

# Or build the binary into bin/server.exe
mage build
```

If `mage` is not in your global `PATH`, invoke it via Go:

```powershell
go run github.com/magefile/mage run
```

## Current pages

- `/` is the landing page. Double-clicking its empty background toggles light
  or dark mode for the current browser tab. The operating-system preference is
  the default. Same-origin page changes use navigation feedback where supported.
- `/book-i` renders the Markdown in `rawcontent/book-i.md`, the canonical Book I
  source tracked in this private repository. Raw HTML in the source is excluded
  from rendered output.
- `/journals` is an empty state with a link to the editor.
- `/journal-editor` is a browser-only prototype with editable Markdown, tags,
  and a live media/content preview. Drafts are not saved and Publish remains
  disabled.
- Unknown routes, forbidden responses, timeouts, and caught panics use a shared
  formatted error page with the HTTP status, an opaque reference code, and a QR
  code containing only that reference. Application route work has a 30-second
  deadline. Exception details and stack traces are never included in responses.

Turso is the intended hosted content database. The product limit is 1 GiB.

## Project journals

- [DECISIONS.md](DECISIONS.md) records durable product and implementation
  choices.
- [NEXT.md](NEXT.md) lists upcoming work.
- [HANDOFF.md](HANDOFF.md) is an append-only session journal tied to Git
  commits.
- `rawcontent/book-i.md` is tracked as the canonical Book I source; keep its
  wording aligned with the author's intent.

## License

This is proprietary software. See [LICENSE](LICENSE) for the usage terms.
