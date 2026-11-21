# Next

- **Latest Commit**: `cb9d677` (`Initialize fresh branch inheriting only .gitignore`)
- **Active Branch**: `v2`
- **Active Experiment / Undo Instructions**: None (clean slate on `v2`; previous v1 history remains intact on branch `main`).

## Next on Mind

1. **Authenticate GitHub CLI (`gh auth login`)**:
   - Connect `gh` (`v2.105.0`) to GitHub so major features and decisions can be proposed and merged via Pull Requests.
2. **Plan & Rewrite Res Imperii from Scratch**:
   - Maintain small, focused modules (no monoliths in code or documentation).
   - Keep all user-facing copy free of internal engineering or technical jargon.
   - Build security in from commit one (strict input whitelisting, browser-bound auth challenges, security headers, self-hosted static scripts, and remote Turso persistence via `../oxri-parent/db.env`).
