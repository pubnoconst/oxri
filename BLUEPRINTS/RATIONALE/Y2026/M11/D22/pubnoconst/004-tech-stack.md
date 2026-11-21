# 004 — Technology Stack Selection: Gomponents, HTMX, Vanilla JS, and Zero-Framework Modern CSS

- **Date**: 2026-11-22
- **Committer**: `pubnoconst`
- **Branch**: `v2-stack-rationale` (targeting `v2`)

## Rationale

This decision formalizes the architectural stack for Res Imperii v2, strictly guided by the foundational axioms in [BLUEPRINTS/Principia.md](../../../../Principia.md):

### 1. View Engine: Gomponents (`maragu.dev/gomponents`) over Templ
- **Zero Buildstep / Pure Standard Go (*Principia* #1 & #4)**:
  Templ requires a separate external CLI binary (`templ.exe`), an extra pre-compilation generation phase (`templ generate`), and generated `*_templ.go` files that pollute the workspace and git history. Gomponents is 100% pure standard Go code: standard `go build`, `go test`, and `gopls` work natively in a single step with zero code generation.
- **Deep Integration with Compile-Time Domain Types**:
  Because Gomponents view nodes are regular Go functions, HTML constructors can directly accept opaque `internal/domain` types (e.g. `SafePath`, `MediaURL`, `RenderedMarkdown`) rather than raw strings or untyped casts, enforcing "Parse, Don't Validate" directly into markup construction.

### 2. Client Runtime: Vendored HTMX + Modern Vanilla JS (Dropping AlpineJS and Lit)
- **Zero Bundler / Zero Node.js (*Principia* #1 & #4)**:
  Lit requires npm packages, a Node.js runtime, and a JavaScript bundler (Vite/Rollup/esbuild) to produce deployable assets. Vendoring a single static `htmx.min.js` file into `assets/js/` eliminates JavaScript buildsteps entirely.
- **Strict CSP / No `unsafe-eval` (*Principia* #3 & #5)**:
  AlpineJS relies on `new Function()` / dynamic evaluation for attribute directives (`x-data`, `@click`), requiring `'unsafe-eval'` in the HTTP `Content-Security-Policy` header. Vanilla JS eliminates this vulnerability, enabling a strict, locked-down CSP header.
- **Focused Simplicity**:
  Client interactions in Res Imperii (clipboard copy, theme double-click toggle, tactile media embed mounting, live character whitelisting, tag autocomplete, and the storage progress bar) require only lightweight, focused modern JavaScript (~30–80 lines each) without framework overhead.

### 3. Styling: Pure Modern CSS over Bulma
- **98% Payload Reduction (Dropping 678 KB of Unused CSS) (*Principia* #1 & #2)**:
  On the `main` branch, `bulma.min.css` added 678 KB while none of its grid, components (navbar, hero, modal), or form elements were actually used. Our custom Flexbox/Grid layouts, card surfaces, and buttons (`.btn-encouraged`, `.btn-neutral`, `.btn-destructive`) were already custom-crafted.
- **Zero Specificity Fights**:
  Using a clean ~30-line modern reset alongside our custom CSS custom property tokens (`design.css` using `color-mix()`, `clamp()`, and Ayu light/dark palettes) gives us complete control, eliminates `!important` and specificity overrides, and renders significantly faster.
