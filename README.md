# Res Imperii

Res Imperii is a private blog for long-form articles and journals. The
`rust-experiment` branch is now a standalone Rust application using the MASH
stack: Maud, Axum, SQLx, and HTMX. The .NET/F# application remains on `main`.

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
src/
  main.rs
  pages/
    shared.rs
    landing/
      mod.rs
      style.css
    book_i/
      mod.rs
      style.css
    journals/
      mod.rs
      style.css
    journal_editor/
      mod.rs
      style.css
    error/
      mod.rs
      style.css
```

Each page's Rust view and page-only stylesheet sit together in
`src/pages/<page>/`. Page modules embed `style.css` at compile time and serve it
through an explicit, versioned `/page-css/...` route. This keeps page styling
easy to find without exposing Rust source through the static file server.

Shared public files live in `assets/`: Bulma and shared project overrides are
under `assets/css/`, browser scripts are under `assets/js/`, and the favicon is
at `assets/favicon.png`. The `/assets` route serves only this directory. Keep
shared styling in `overrides.css`; leave minified Bulma untouched. Bump a local
asset's `?v=N` query version when changing its contents.

## Run locally

Install Rust stable and Visual Studio 2022 Build Tools with the C++ ARM64 tools
and Windows SDK. On Windows ARM64, initialize the Visual Studio developer
environment before running Cargo so it can find `link.exe` and the SDK:

```powershell
cmd.exe /d /s /c 'call "C:\Program Files (x86)\Microsoft Visual Studio\2022\BuildTools\Common7\Tools\VsDevCmd.bat" -arch=arm64 -host_arch=arm64 && cargo run'
```

The current SQLx dependency has no database driver or TLS backend enabled. The
database and connection features remain to be selected before persistence is
implemented. The current feature set builds without Clang; adding SQLite or a
rustls `ring` backend may introduce native toolchain requirements.

## Current pages

- `/` is the landing page. Double-clicking its empty background toggles light
  or dark mode for the current browser tab. The operating-system preference is
  the default. Same-origin page changes use navigation feedback where supported.
- `/book-i` renders the initial Rust Book I page. The canonical prose remains
  author-owned source material and is not stored in Git.
- `/journals` is an empty state with a link to the editor.
- `/journal-editor` is a browser-only prototype with editable Markdown, tags,
  and a live media/content preview. Drafts are not saved and Publish remains
  disabled.
- Unknown routes and caught panics use a generic error page. Exception details
  and stack traces are never included in the response.

Turso is the intended hosted content database. The product limit is 1 GiB, but
the Rust application does not yet connect to a database, enforce the limit, or
implement journal management.

## Project journals

- [DECISIONS.md](DECISIONS.md) records durable product and implementation
  choices, including the page-local CSS layout.
- [NEXT.md](NEXT.md) lists upcoming work.
- [HANDOFF.md](HANDOFF.md) is an append-only session journal tied to Git
  commits.
- `rawcontent/` is excluded by `.gitignore`; preserve the author's source
  material.

## License

This is proprietary software. See [LICENSE](LICENSE) for the usage terms.
