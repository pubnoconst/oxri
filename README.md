# Res Imperii (`oxri`)

**Res Imperii** is a minimalist, security-hardened personal philosophical archive and journal system. It is engineered as a single, statically compiled pure-Go binary (`CGO_ENABLED=0`) with zero runtime C dependencies, zero code-generation steps, and zero external CSS/JS framework bloat. Static assets are embedded in the binary, so a deploy is one file.

---

## 1. How to Navigate This Repository (For Humans & Agents)

This repository is structured so that any engineer or AI agent can pick up full context within seconds without reading monolithic logs:

1. **[`BLUEPRINTS/Principia.md`](BLUEPRINTS/Principia.md) — Start Here**
   The 8 foundational axioms governing architecture, UX minimalism, input distrust (*"All Inputs are evil"*), build simplicity, under-the-hood concealment, type-system enforcement, and the Pull Request workflow.
2. **[`BLUEPRINTS/NEXT.md`](BLUEPRINTS/NEXT.md) — Current Session Pointer**
   The latest base commit, the active (and any parked) branch, undo instructions for any in-flight experiment, the standing design and code principles, the modularity / type-safety roadmap, and what is next on mind.
3. **[`BLUEPRINTS/RATIONALE/`](BLUEPRINTS/RATIONALE/) — Date-Partitioned Rationales**
   One entry per merged Pull Request, committed inside that PR:
   ```text
   BLUEPRINTS/RATIONALE/Y<YYYY>/M<MM>/D<D>/<git-committer>/<NNN>-<short-commit-hash>.md
   ```
   The date folder and the timestamps in each entry come from git (`git log -1 --format=%ci <hash>`), never from the session; `<NNN>` is one global sequence. Every change is developed on a dedicated branch, UI-tested, verified by automated tests and CI, and merged via GitHub CLI Pull Requests (`gh pr create` / `gh pr merge`).

---

## 2. Directory Structure

```text
oxri/
├── .gitattributes                   # LF line endings everywhere (overrides core.autocrlf)
├── .github/workflows/ci.yml         # GitHub Actions CI (gofmt + vet + CGO_ENABLED=0 tests + Linux static build)
├── Dockerfile                       # Multi-stage CGO_ENABLED=0 distroless container (single binary)
├── render.yaml                      # Render Web Service deployment manifest
├── magefile.go                      # Pure-Go build, test, setup & backup targets (CGO_ENABLED=0)
├── BLUEPRINTS/
│   ├── Principia.md                 # Core engineering, UX, and security axioms (#1–#8)
│   ├── NEXT.md                      # Active branch pointer, principles, roadmap
│   └── RATIONALE/                   # One entry per merged PR, dated from git (Y2026/M11/D24/...)
├── cmd/
│   ├── server/main.go               # HTTP server entrypoint (0.0.0.0:$PORT in prod, 127.0.0.1:$PORT local)
│   ├── totpgen/main.go              # TOTP seed & QR generator (`mage totp`)
│   ├── passgen/main.go              # Loopback Argon2id passphrase setup tool (`mage password`)
│   └── backup/main.go               # Offline Turso JSON snapshot export & restore CLI (`mage backup`)
├── internal/
│   ├── config/                      # Secret env files: LoadSecrets("db.env", …) from ../oxri-parent/ then ../
│   ├── auth/                        # Passkeys, Argon2id password + RFC 6238 TOTP sign-in policy (login.go), sessions, AdminCap minting
│   ├── db/                          # Remote Turso/libSQL store (schema v6): journals, books, passkeys, auth_state, backup, quota; capability-gated writes
│   └── domain/                      # "Parse, Don't Validate" opaque domain types & the AdminCap capability token
├── serverfns/
│   ├── doc.go                       # Encapsulated server functions boundary
│   └── auth.go                      # Session → AdminCap verification, fresh step-up checks & redirect sanitization
├── routes/
│   ├── routes.go                    # Top-level Chi router, password+TOTP sign-in, settings & stylesheet mounts
│   ├── admin.go                     # adminOnly / freshAdminOnly: the only way to serve a handler that holds an AdminCap
│   ├── books.go                     # Book I/II/III reader and Markdown editor handlers
│   ├── journals.go                  # Journal catalog shell, HTMX fragment, editor & vacate-space handlers
│   ├── passkeys.go                  # WebAuthn registration, sign-in & step-up gated deletion handlers
│   └── middleware.go                # Strict CSP, HSTS, CSRF origin check, 1 MiB body cap, proxy IP trust & rate limits
├── views/
│   ├── layout/                      # Base HTML5 document shell & pure-CSS BackLink primitive
│   ├── landing/                     # Landing page view (landing.go + landing.css)
│   ├── booki/                       # Book I/II/III long-form reader & live editor (+ booki.css)
│   ├── journals/                    # Journals catalog, permalink & Make Room views (+ journals.css)
│   ├── editor/                      # 2-column compose & live preview editor (+ editor.css)
│   ├── loading/                     # "Confirming, please wait." submit overlay with the dot-matrix loader (+ loading.css)
│   ├── login/                       # Passkey-first sign-in with password + TOTP fallback (+ login.css)
│   ├── settings/                    # Administrative security settings & passkey management (+ settings.css)
│   └── errorpage/                   # Client-safe 4xx/5xx error card with UUID reference & QR code
└── assets/
    ├── assets.go                    # Embeds the files below; typed, content-hashed handles (assets.SiteCSS, …)
    ├── css/site.css                 # CSS reset, design tokens (depth surfaces), the dot-matrix loader, view transitions
    ├── js/                          # Locally vendored HTMX, markdown-it, DOMPurify & vanilla JS scripts
    └── favicon.png                  # Brand icon
```

---

## 3. Technology Stack & Security Architecture

- **Pure Go (`CGO_ENABLED=0`)**: Single-step compilation via Go and Mage (`magefile.go`). No C/C++ compiler toolchain (`gcc`/`clang`), no dynamic linking, and no external template code generation.
- **Gomponents (`maragu.dev/gomponents`)**: Views are type-checked Go functions in `views/<component>/`. Each view owns its scoped stylesheet (`views/<component>/<component>.css`) layered over global tokens in `assets/css/site.css`.
- **Embedded, Content-Hashed Assets (`assets/`)**: Every static file is embedded in the binary and referenced through a typed handle whose URL carries a hash of the exact bytes served (`/assets/js/journal-media.js?v=<hash>`); view stylesheets get the same via `assets.Component`. A current hash is served `Cache-Control: immutable`; anything else revalidates. Version numbers are never written by hand: tests fail if an embedded file lacks a handle or if any Go source hand-writes an `/assets/` URL or `?v=` literal.
- **Vendored Frontend Assets & Strict CSP**: HTMX (`htmx.min.js`), `markdown-it.min.js`, and `purify.min.js` are vendored locally. `Content-Security-Policy` enforces zero `'unsafe-eval'` and zero `'unsafe-inline'` scripts (`script-src 'self' https://platform.twitter.com`).
- **Design Language**: Plain surfaces at a few named depths (page, card, laid-on-card, sunk wells), each a surface colour with its own shadow, defined as CSS tokens and documented in the HIG. "Please wait" states are one quiet 3×3 dot matrix (`.dot-loading` in `site.css`), CSS-drawn and opacity-only so it stays smooth on budget phones while tweets load. (The earlier water ripple is archived on the `water-ripple` branch.) Subtlety is the key. The full UI language is in [`BLUEPRINTS/HIG.md`](BLUEPRINTS/HIG.md).
- **Authorization by Capability (`domain.AdminCap`)**: Every administrative database write demands an `AdminCap`, which only `internal/auth` can mint after verifying a session. Admin handlers are typed `func(w, r, cap)` and can only be served through `routes/admin.go` (`adminOnly`, `freshAdminOnly`), so a handler that forgets the check does not compile. Writes reject the zero value at runtime, and a `go/ast` test pins where a zero literal may appear. Deliberately uncapped: sign-in bookkeeping, startup configuration, and the offline `cmd/backup` restore (pinned to that call site).
- **Authentication & Step-Up Access Control**:
  - **FIDO2 / WebAuthn Passkeys** (`github.com/go-webauthn/webauthn`): Primary passwordless sign-in keyed by `RPID` in Turso (`passkeys` table). Adding or removing passkeys in `/settings` requires **fresh reauthentication within 5 minutes**.
  - **Argon2id Password + RFC 6238 TOTP Fallback** (`internal/auth/login.go`): **instant sign-in on success**; failures cost time via an escalating per-IP hold (2s, 4s, 8s, 16s), a per-IP lockout (5 failures / 5 min) and a **global failure budget** (30 failures / 10 min from all addresses pauses password+code sign-in; passkeys are unaffected, so the owner can never be locked out). Held, locked and paused attempts are rejected before any Argon2id work. All counters are in memory; the only write is the monotonic TOTP replay watermark in Turso (`auth_state`, schema v6), once per successful sign-in.
  - **CSRF & State-Changing Methods**: `POST`-only `/logout` (`GET` returns `405 Method Not Allowed`), `Sec-Fetch-Site` / `Origin` verification, HttpOnly/SameSite/Secure session cookies, and session rotation on sign-in/reauth.
  - **Hardened Reverse-Proxy Trust**: `TRUST_PROXY=true` honors `True-Client-IP`, `CF-Connecting-IP`, or the rightmost untrusted `X-Forwarded-For` hop only when `r.RemoteAddr` belongs to a trusted loopback, RFC 1918/4193 private, CGNAT (`100.64.0.0/10`), or `TRUSTED_PROXY_CIDRS` address.
- **Independent Server-Side Input Sanitization (*"All Inputs are evil"*)**:
  - Printable ASCII URLs (`1..128` chars, anti-SSRF host/IP checks), Title-Cased **ASCII alphanumeric tags** (max 3 tags, `1..8` chars each, unique per entry), and Latin/Greek/digit/standard-punctuation Markdown prose (zero emojis, zero zero-width/bidi spoofing characters).
- **Remote Turso Database (`schema v6`) & 4.5 GB Auto-Vacate**:
  - Books I, II, and III, journals, WebAuthn passkeys, and TOTP replay watermarks all live in remote Turso/libSQL (`libsql://` or `https://`; local `file:` databases are disabled, `:memory:` is used for unit tests).
  - Enforces a 4.5 GB UTF-8 storage quota inside serialized write transactions with prioritized eviction (`AutoVacateBytes`: unpublished drafts first, then published un-bookmarked entries, oldest first).

---

## 4. Configuration, External Secrets & Offline Backups

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

Other environment variables (all optional):

| Variable | Effect |
| --- | --- |
| `PORT` / `ADDR` | Listen port (default `8080`) or full listen address. |
| `APP_ENV=prod` (or `RENDER` set) | Listen on `0.0.0.0` instead of loopback. |
| `TRUST_PROXY=true` | Honor client-IP headers from trusted proxies (see above). |
| `TRUSTED_PROXY_CIDRS` | Extra comma-separated proxy CIDRs to trust. |
| `TRUSTED_PROXY_HOPS` | Trusted proxy hops in `X-Forwarded-For` (default `1`). |
| `WEBAUTHN_RPID` / `WEBAUTHN_ORIGIN` | Override the passkey relying-party ID and comma-separated allowed origins (otherwise derived from the request). |

---

## 5. Build, Test, Setup & Backup Commands

All commands enforce `CGO_ENABLED=0`:

```powershell
# Run all unit and integration tests
mage test

# Type-check the site's JavaScript (strict, from JSDoc; nothing is compiled).
# Needs Node once: `npm ci` installs the pinned TypeScript.
mage typecheck

# Run tests, the JavaScript type-check, and compile the static binary into bin/
mage check

# Compile the static binary into bin/
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

- **Production Deployment**: Distroless Docker container (`Dockerfile`, `render.yaml`) deployed on Render from the `prod` branch, fronted by Render's ingress proxy (`APP_ENV=prod`, `TRUST_PROXY=true`). The image holds a single binary; assets are embedded.
- **Single instance by design**: sessions, sign-in counters and rate limiters live in memory (see Principia #1). Run exactly one instance; a restart signs everyone out.
- **Continuous Integration**: `.github/workflows/ci.yml` fails on unformatted code (`gofmt -l`), runs `go vet` and `CGO_ENABLED=0 go test -v ./...`, and verifies the static `GOOS=linux GOARCH=amd64` binary build on every push and PR to `main`. Merges wait for it to pass.
- **Release flow**: feature branch → local build and UI test → PR → CI → merge to `main` → fast-forward `prod` to `main` (Render deploys `prod`).
- **Lineage**:
  - **`main`** (and **`prod`**, what Render deploys): Active pure-Go + Gomponents + Vanilla JS + Modern CSS architecture.
  - **`v1`**, **`fsharp-legacy`**, **`rust-experiment`**: Archived prior iterations.
