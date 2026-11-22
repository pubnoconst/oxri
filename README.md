# Res Imperii (`oxri`)

**Res Imperii** is a minimalist, security-hardened personal philosophical archive and journal system. It is engineered as a single, statically compiled pure-Go binary (`CGO_ENABLED=0`) with zero runtime C dependencies, zero code-generation steps, and zero external CSS/JS framework bloat.

---

## 1. How to Navigate This Repository (For Humans & Agents)

This repository is structured so that any engineer or AI agent can pick up full context within seconds without reading monolithic logs:

1. **[`BLUEPRINTS/Principia.md`](BLUEPRINTS/Principia.md) — Start Here**
   The 8 foundational axioms governing architecture, UX minimalism, input distrust (*"All Inputs are evil"*), build simplicity, under-the-hood concealment, type-system enforcement, and the Pull Request workflow.
2. **[`BLUEPRINTS/NEXT.md`](BLUEPRINTS/NEXT.md) — Current Session Pointer**
   Tracks the latest commit hash, active branch, undo instructions for any in-flight experiment, completed milestones, and what is next on mind.
3. **[`BLUEPRINTS/RATIONALE/`](BLUEPRINTS/RATIONALE/) — Date-Partitioned Commit Rationales**
   Unifies architectural decisions and handoff notes into immutable, chronological entries organized by date and author:
   ```text
   BLUEPRINTS/RATIONALE/Y<YYYY>/M<MM>/D<D>/<git-committer>/<NNN>-<short-commit-hash>.md
   ```
   Every major decision or feature is developed on a dedicated branch, documented with a rationale entry, tested in the UI, and merged via GitHub CLI Pull Requests (`gh pr create` / `gh pr merge`).

---

## 2. Directory Structure

```text
oxri/
├── BLUEPRINTS/
│   ├── Principia.md                 # Core engineering, UX, and security axioms (#1–#8)
│   ├── NEXT.md                      # Active session pointer, latest commit hash, and roadmap
│   └── RATIONALE/                   # Date-sorted rationale entries (Y2026/M11/D22/...)
├── cmd/
│   └── server/
│       └── main.go                  # HTTP & Loopback SSH server entrypoint with graceful shutdown
├── internal/
│   ├── auth/                        # Loopback SSH challenge listener & 256-bit browser binding
│   ├── db/                          # Remote Turso/libSQL store, versioned migrations & 4.5 GB quota
│   └── domain/                      # "Parse, Don't Validate" opaque domain types & AdminCap token
├── serverfns/
│   ├── doc.go                       # Encapsulated server functions boundary
│   └── auth.go                      # Session verification, AdminCap minting & redirect sanitization
├── routes/
│   ├── routes.go                    # Top-level HTTP router & component CSS mounts
│   ├── journals.go                  # Journal catalog, editor, and vacate-space handlers
│   └── middleware.go                # CSP, nosniff, CSRF, 1 MiB body cap, dir-listing denial & rate limits
├── views/
│   ├── layout/                      # Base HTML5 document shell & pure-CSS BackLink primitive
│   ├── landing/                     # Landing page view (landing.go + landing.css)
│   ├── booki/                       # Book I long-form reader view (booki.go + booki.css)
│   ├── journals/                    # Journals catalog, permalink & Make Room views (+ journals.css)
│   ├── editor/                      # 2-column compose & live preview editor (+ editor.css)
│   ├── loading/                     # "Confirming, please wait." magnifying-wave UI (+ loading.css)
│   ├── login/                       # 3-step SSH authentication view (+ login.css)
│   ├── pilot/                       # Administrative security & credential settings (+ settings.css)
│   └── errorpage/                   # Client-safe 4xx/5xx error card with UUID & pure-Go QR code
├── assets/
│   ├── css/
│   │   └── site.css                 # Modern CSS reset, canonical design tokens & view transitions
│   ├── js/                          # Locally vendored HTMX, markdown-it, DOMPurify & vanilla JS scripts
│   └── favicon.png                  # Brand icon
├── rawcontent/
│   ├── book-i.md                    # Canonical Markdown source for Book I ("On Cause")
│   └── booki.go                     # Go embed directive for Book I
└── magefile.go                      # Pure-Go build targets enforcing CGO_ENABLED=0
```

---

## 3. Technology Stack & Design Highlights

- **Pure Go (`CGO_ENABLED=0`)**: Single-step compilation via Go and Mage (`magefile.go`). No C/C++ compiler toolchain (`gcc`/`clang`), no dynamic linking, and no external template code generation.
- **Gomponents (`maragu.dev/gomponents`)**: Views are type-checked Go functions in `views/<component>/`. Each view owns its scoped stylesheet (`views/<component>/<component>.css`) served at `/views/<component>/<component>.css`, layered over global design tokens in `assets/css/site.css`.
- **Vendored Frontend Assets**: HTMX (`htmx.min.js`), `markdown-it.min.js`, and `purify.min.js` are vendored locally in `assets/js/`. Zero third-party script CDNs and zero `'unsafe-eval'` in `Content-Security-Policy`.
- **Independent Server-Side Input Sanitization (*"All Inputs are evil"*)**:
  - Strict character-set whitelisting enforced independently on both client and server: printable ASCII URLs (`1..128` chars, anti-SSRF host checks), Title-Cased Latin/Greek/digit tags (max 3 tags, `1..4` runes each, unique per entry), and Latin/Greek/digit/standard-punctuation Markdown prose (zero emojis, zero zero-width/bidi spoofing characters).
  - Invalid inputs on `422 Unprocessable Entity` are stripped of disallowed characters before re-rendering the editor form so evil payloads are never echoed into the DOM.
- **Browser-Bound Loopback SSH Authentication**:
  - `/login` issues a 6-digit challenge token paired with a 256-bit HttpOnly browser-binding cookie (`res_imperii_login_binding`); only its SHA-256 hash is stored on the server.
  - Signing the exact 6-digit token via `ssh -p 2222 <token>@localhost` unlocks only the initiating browser session.
- **Remote Turso Database & 4.5 GB Auto-Vacate**:
  - Connects exclusively to a remote Turso `libsql://` or `https://` database (local disk `file:` databases are disabled; ephemeral `:memory:` is used for unit tests).
  - Enforces a self-imposed 4.5 GB storage budget with prioritized automatic eviction (`AutoVacateBytes`: unpublished drafts first, then published un-bookmarked entries, oldest first) via `/journals/vacate-space/`.

---

## 4. External Secrets Configuration

Secrets are stored **outside** the Git repository in the sibling directory `../oxri-parent/`:

- `../oxri-parent/db.env`:
  ```ini
  TURSO_DATABASE_URL=libsql://your-database.turso.io
  TURSO_AUTH_TOKEN=your-turso-jwt
  TURSO_MAX_STORAGE_BYTES=4500000000
  ADMIN_SSH_PUBKEY=ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAA...
  ```
- `../oxri-parent/.ssh_host_key`:
  Ed25519 private host key for the loopback SSH authentication gateway (auto-generated with `0600` permissions if absent).

---

## 5. Build, Test & Run Commands

All commands enforce `CGO_ENABLED=0`:

```powershell
# Run all unit and integration tests
mage test

# Run tests and compile the static binary into bin/server.exe
mage check

# Compile the static binary into bin/server.exe
mage build

# Build and run the HTTP (127.0.0.1:8080) and SSH (127.0.0.1:2222) servers
mage run

# Remove compiled artifacts
mage clean
```

---

## 6. Branch Lineage

- **`main`** (and **`v2`**): Current pure-Go + Gomponents + Vanilla JS + Modern CSS architecture.
- **`v1`**: Archived first-generation Go implementation (Templ + Bulma + AlpineJS).
- **`fsharp-legacy`**: Archived F# / Giraffe / .NET implementation.
- **`rust-experiment`**: Archived Rust prototype.
