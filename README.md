# Res Imperii

Res Imperii is a private blog for long-form articles and journals. The
application on `main` is a standalone Go application using the GOTH stack:
Go, Templ, HTMX, and Tailwind/Bulma (Bulma + overrides in this project), paired
with Alpine.js and Chi routing. The Rust experiment remains on `rust-experiment`
and the legacy .NET/F# application is preserved on `fsharp-legacy`.

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
  auth/
    auth.go
    auth_test.go
    middleware.go
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
    login/
      login.templ
      login.go
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

- `/` is the landing page. Double-clicking an empty background on any page
  toggles light or dark mode for the current browser tab. The operating-system
  preference is the default. Same-origin page changes use navigation feedback
  where supported.
- `/book-i` renders the Markdown in `rawcontent/book-i.md`, the canonical Book I
  source tracked in this private repository. Raw HTML in the source is excluded
  from rendered output.
- `/journals` renders a compact multi-column preview collage of published journal
  entries from Turso/SQLite (showing compact media embeds, tags, and a 280-character
  excerpt; clicking any card navigates to `/journals/{id}`). When authenticated as
  the author, unpublished drafts are also shown alongside Edit, Bookmark,
  Publish/Unpublish, Delete, and multi-select batch management controls.
- `/journals/{id}` renders the full-page journal entry with full Markdown prose
  and uncropped tactile media (unpublished entries return 404 unless authenticated).
- `/journals/new` (and `/journals/{id}/edit` when editing) provides a two-pane
  composition environment with schema-enforced live validation (required media
  link 1–128 chars, up to 3 tags <= 4 chars each, required Markdown body) and
  scroll-synchronized preview (supporting YouTube, Vimeo, audio/video/images,
  and X / Twitter post embeds).
- Standardized buttons: interactive actions use semantic classes (`.btn-encouraged`
  for primary actions like publishing, `.btn-neutral` for utility/secondary
  actions, and `.btn-destructive` for deletions).
- `/pilot` is the dedicated route namespace for experimental author features,
  prototypes, and administrative tools (redirects to `/pilot/user/settings`).
  Access requires an authenticated session.
- `/pilot/user/settings` is the experimental profile dashboard for security and
  authentication method management. It displays active loopback SSH credentials
  and future FIDO2/WebAuthn passkeys (Proton Pass, hardware keys, biometrics);
  credential addition and removal are visually represented but disabled until a
  domain and HTTPS are configured.
- `/login` is the loopback SSH authentication gateway. Running the copyable
  `ssh -p 2222 <token>@localhost` or `ssh -p 2222 admin@localhost` command
  authenticates via your local Ed25519 key (OpenSSH or Proton Pass agent) and
  unlocks the browser tab via HTMX polling.
- `/logout` clears the authenticated session cookie and redirects to `/`.
- Unknown routes, forbidden responses, timeouts, and caught panics use a shared
  formatted error page with the HTTP status, an opaque reference code, and a QR
  code containing only that reference. Application route work has a 30-second
  deadline. Exception details and stack traces are never included in responses.

## Authentication (SSH loopback)

Res Imperii uses loopback SSH public-key authentication for single-admin operations:

1. Navigating to `/login` creates an ephemeral 6-digit challenge token (valid 5 minutes).
2. The page displays `ssh -p 2222 <token>@localhost` (with a copy button) and polls `/auth/status` via HTMX.
3. The server runs an embedded loopback SSH server (`golang.org/x/crypto/ssh`).
4. Running the SSH command verifies the client's Ed25519 public key against the configured authorized key.
5. Upon verification, the server marks the session approved and exits the SSH connection.
6. The browser automatically receives an HTTP-only session cookie (`res_imperii_session`) and redirects to `/journals`.

Environment variables (see `.env.example`):
- `SSH_PORT`: SSH listening port (default `2222`).
- `SSH_ADDR`: SSH listening address (default `127.0.0.1:2222`).
- `ADMIN_SSH_PUBKEY`: Authorized SSH public key (defaults to author's Ed25519 key).
- `HOST_KEY_PATH`: Path to persist auto-generated host key (defaults to `.ssh_host_key`).
- `TURSO_DATABASE_URL`: Turso/libSQL database URL (`libsql://...` or local `file:...` fallback; reads `db.env` or `.env`).
- `TURSO_AUTH_TOKEN`: Turso database JWT token (stored in untracked `db.env`).

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
