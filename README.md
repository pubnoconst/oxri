# Res Imperii (`oxri`)

**Res Imperii** is a minimalist, security-hardened personal philosophical archive and journal system. It is engineered as a single, statically compiled pure-Go binary (`CGO_ENABLED=0`) with zero runtime C dependencies, zero code-generation steps, and zero external CSS/JS framework bloat.

---

## 1. How to Navigate This Repository (For Humans & Agents)

This repository is structured so that any engineer or AI agent can pick up full context within seconds without reading monolithic logs:

1. **[`BLUEPRINTS/Principia.md`](BLUEPRINTS/Principia.md) — Start Here**
   The 8 foundational axioms governing architecture, UX minimalism, input distrust (*"All Inputs are evil"*), build simplicity, under-the-hood concealment, type-system enforcement, and the Pull Request workflow.
2. **[`BLUEPRINTS/NEXT.md`](BLUEPRINTS/NEXT.md) — Current Session Pointer**
   Tracks the latest milestone, active branch, undo instructions for any in-flight experiment, completed features, and what is next on mind.
3. **[`BLUEPRINTS/RATIONALE/`](BLUEPRINTS/RATIONALE/) — Date-Partitioned Commit Rationales**
   Unifies architectural decisions and handoff notes into chronological entries organized by date and author:
   ```text
   BLUEPRINTS/RATIONALE/Y<YYYY>/M<MM>/D<D>/<git-committer>/<NNN>-<short-commit-hash>.md
   ```
   Every major decision or feature is developed on a dedicated branch, verified in UI and automated tests, and merged via GitHub CLI Pull Requests (`gh pr create` / `gh pr merge`).

---

## 2. Directory Structure

```text
oxri/
├── .github/workflows/ci.yml         # GitHub Actions CI (CGO_ENABLED=0 test + Linux amd64 static build)
├── Dockerfile                       # Multi-stage CGO_ENABLED=0 distroless container (~15 MB)
├── render.yaml                      # Render Web Service deployment manifest
├── magefile.go                      # Pure-Go build, test, setup & backup targets (CGO_ENABLED=0)
├── BLUEPRINTS/
│   ├── Principia.md                 # Core engineering, UX, and security axioms (#1–#8)
│   ├── NEXT.md                      # Active branch pointer, milestones, and roadmap
│   └── RATIONALE/                   # Date-sorted rationale entries (Y2026/M11/D22/...)
├── cmd/
│   ├── server/main.go               # HTTP server entrypoint (0.0.0.0:$PORT in prod, 127.0.0.1:$PORT local)
│   ├── totpgen/main.go              # TOTP seed & QR generator (`mage totp`)
│   ├── passgen/main.go              # Loopback Argon2id passphrase setup tool (`mage password`)
│   └── backup/main.go               # Offline Turso JSON snapshot export & restore CLI (`mage backup`)
├── internal/
│   ├── auth/                        # FIDO2/WebAuthn Passkeys, Argon2id password, RFC 6238 TOTP & sessions
│   ├── db/                          # Remote Turso/libSQL store (schema v6, journals, books, passkeys, auth_state, backup, quota)
│   └── domain/                      # "Parse, Don't Validate" opaque domain types & AdminCap capability token
├── serverfns/
│   ├── doc.go                       # Encapsulated server functions boundary
│   └── auth.go                      # Session verification, fresh step-up auth checks, AdminCap minting & redirect sanitization
├── routes/
│   ├── routes.go                    # Top-level Chi HTTP router, auth flows & component CSS mounts
│   ├── books.go                     # Book I/II/III reader and Markdown editor handlers
│   ├── journals.go                  # Journal catalog shell, HTMX fragment, editor & vacate-space handlers
│   ├── passkeys.go                  # WebAuthn registration, login & step-up gated deletion handlers
│   └── middleware.go                # Strict CSP, HSTS, CSRF origin check, 1 MiB body cap, proxy IP trust & rate limits
├── views/
│   ├── layout/                      # Base HTML5 document shell & pure-CSS BackLink primitive
│   ├── landing/                     # Landing page view (landing.go + landing.css)
│   ├── booki/                       # Book I/II/III long-form reader & live editor (+ booki.css)
│   ├── journals/                    # Journals catalog, permalink & Make Room views (+ journals.css)
│   ├── editor/                      # 2-column compose & live preview editor (+ editor.css)
│   ├── loading/                     # "Confirming, please wait." transition UI (+ loading.css)
│   ├── login/                       # Passkey-first + Password/TOTP login & 30s cooldown views (+ login.css)
│   ├── pilot/                       # Administrative security settings & passkey management (+ settings.css)
│   └── errorpage/                   # Client-safe 4xx/5xx error card with UUID & pure-Go QR code
└── assets/
    ├── css/site.css                 # Modern CSS reset, canonical design tokens, checkbox theming & view transitions
    ├── js/                          # Locally vendored HTMX, markdown-it, DOMPurify & vanilla JS scripts
    └── favicon.png                  # Brand icon
```

---

## 3. Technology Stack & Security Architecture

- **Pure Go (`CGO_ENABLED=0`)**: Single-step compilation via Go and Mage (`magefile.go`). No C/C++ compiler toolchain (`gcc`/`clang`), no dynamic linking, and no external template code generation.
- **Gomponents (`maragu.dev/gomponents`)**: Views are type-checked Go functions in `views/<component>/`. Each view owns its scoped stylesheet (`views/<component>/<component>.css`) layered over global tokens in `assets/css/site.css`.
- **Vendored Frontend Assets & Strict CSP**: HTMX (`htmx.min.js`), `markdown-it.min.js`, and `purify.min.js` are vendored locally in `assets/js/`. `Content-Security-Policy` enforces zero `'unsafe-eval'` and zero `'unsafe-inline'` scripts (`script-src 'self' https://platform.twitter.com`).
- **Authentication & Step-Up Access Control**:
  - **FIDO2 / WebAuthn Passkeys** (`github.com/go-webauthn/webauthn`): Primary passwordless login keyed by `RPID` in Turso (`passkeys` table). Adding or removing passkeys in `/pilot/user/settings` requires **fresh reauthentication within 5 minutes** (`RequireFreshAuth`).
  - **Argon2id Password + RFC 6238 TOTP Fallback**: Mandatory 30-second server-side per-IP confirmation cooldown (`StartTOTPVerification`), persistent TOTP replay watermark saved monotonically in Turso (`auth_state` table, schema v6), and **per-IP failed-attempt lockout** (never global lockout).
  - **CSRF & State-Changing Methods**: `POST`-only `/logout` (`GET` returns `405 Method Not Allowed`), `Sec-Fetch-Site` / `Origin` verification, HttpOnly/SameSite/Secure session cookies, and session rotation on login/reauth.
  - **Hardened Reverse-Proxy Trust**: `TRUST_PROXY=true` honors `True-Client-IP`, `CF-Connecting-IP`, or rightmost untrusted `X-Forwarded-For` hop only when `r.RemoteAddr` belongs to a trusted loopback, RFC 1918/4193 private, CGNAT (`100.64.0.0/10`), or `TRUSTED_PROXY_CIDRS` address.
- **Independent Server-Side Input Sanitization (*"All Inputs are evil"*)**:
  - Printable ASCII URLs (`1..128` chars, anti-SSRF host/IP checks), Title-Cased **ASCII alphanumeric tags** (max 3 tags, `1..8` chars each, unique per entry), and Latin/Greek/digit/standard-punctuation Markdown prose (zero emojis, zero zero-width/bidi spoofing characters).
- **Remote Turso Database (`schema v6`) & 4.5 GB Auto-Vacate**:
  - Books I, II, and III, journals, WebAuthn passkeys, and TOTP replay watermarks all live in remote Turso/libSQL (`libsql://` or `https://`; local `file:` databases are disabled, `:memory:` is used for unit tests).
  - Enforces a 4.5 GB UTF-8 storage quota inside serialized write transactions with prioritized eviction (`AutoVacateBytes`: unpublished drafts first, then published un-bookmarked entries, oldest first).

---

## 4. External Secrets & Offline Backups

Secrets and offline backups live **outside** the Git repository in `../oxri-parent/`:

- `../oxri-parent/db.env`:
  ```ini
  TURSO_DATABASE_URL=libsql://your-database.turso.io
  TURSO_AUTH_TOKEN=your-turso-jwt
  TURSO_MAX_STORAGE_BYTES=4500000000
  ADMIN_TOTP_SECRET=YOURBASE32TOTPSECRET
  ```
- `../oxri-parent/pw.env`:
  ```ini
  ADMIN_PASSWORD_HASH=$argon2id$v=19$...
  ```
- `../oxri-parent/backups/`: Timestamped JSON snapshots created with `0600` permissions via `mage backup` (`go run ./cmd/backup`). Restore with `go run ./cmd/backup -restore path/to/snapshot.json`.

---

## 5. Build, Test, Setup & Backup Commands

All commands enforce `CGO_ENABLED=0`:

```powershell
# Run all unit and integration tests
mage test

# Run tests and compile the static binary into bin/server.exe
mage check

# Compile the static binary into bin/server.exe
mage build

# Start the local server (127.0.0.1:8080)
mage run

# Initialize or view TOTP secret + QR code in ../oxri-parent/
mage totp

# Generate an Argon2id password hash into ../oxri-parent/pw.env
mage password

# Export an offline JSON backup snapshot from Turso into ../oxri-parent/backups/
mage backup

# Remove compiled artifacts
mage clean
```

---

## 6. Deployment & Branch Lineage

- **Production Deployment**: Distroless Docker container (`Dockerfile`, `render.yaml`) deployed on Render from the `prod` branch, fronted by Render's ingress proxy (`APP_ENV=prod`, `TRUST_PROXY=true`).
- **Continuous Integration**: `.github/workflows/ci.yml` runs `CGO_ENABLED=0 go test -v ./...` and verifies the static `GOOS=linux GOARCH=amd64` binary build on every push and PR to `main`.
- **Lineage**:
  - **`main`** (and **`v2`**, **`prod`**): Active pure-Go + Gomponents + Vanilla JS + Modern CSS architecture.
  - **`v1`**, **`fsharp-legacy`**, **`rust-experiment`**: Archived prior iterations.
