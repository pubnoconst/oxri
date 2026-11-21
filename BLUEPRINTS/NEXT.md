# Next

- **Latest Commit**: `9d88765` (`Add BLUEPRINTS/Principia.md foundational charter`)
- **Active Branch**: `v2`
- **Active Experiment / Undo Instructions**: None.

## Next on Mind

1. **Scaffold Go Module & `CGO_ENABLED=0` Build System (`go.mod`, `magefile.go`)**:
   - Initialize the Go module on Go `1.27.4+` (resolving stdlib CVEs) and a minimal `magefile.go` that enforces `CGO_ENABLED=0` across `build`, `run`, and `test`.
2. **Design & Implement `internal/domain` ("Parse, Don't Validate")**:
   - Implement opaque, unexported-field domain types (`MediaURL`, `Tag`, `TagSet`, `Prose`, `SafePath`, `ValidatedEntry`) enforcing *Principia* #3, #5, and #6 at compile time.
3. **Design & Implement Hardened Authentication & Storage Modules**:
   - Build capability-gated (`AdminCap`) remote Turso persistence (`../oxri-parent/db.env`) and browser-bound authentication with security headers and zero external CDN script dependencies.
