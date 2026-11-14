# Session handoff log

This file is an append-only log for passing work between sessions. Git commits
are the source of truth for code and document changes; entries here summarize
the state and point to the commit that contains the work being handed off.

## How to add an entry

1. If the work follows a new decision, add its stable ID and rationale to
   `DECISIONS.md`, then commit that record. Keep the decision commit separate
   from implementation commits when practical.
2. Implement the work in a commit whose message references the decision ID.
   Git creates each commit hash locally, before pushing.
3. Append a dated handoff entry that links the decision ID to its decision
   commit and related implementation commit(s), and records outcomes, remaining
   work, and anything the next session should know.
4. Commit the appended handoff entry and sync the commits with the remote.
5. Do not edit or remove older entries. Add a new entry to correct or supersede
   an earlier handoff. To undo implementation, use `git revert <hash>` so the
   decision rationale and audit trail remain in history.

Keep upcoming tasks in `NEXT.md` and durable product or implementation rationale
in `DECISIONS.md`; avoid copying their full contents into this timeline.

## 2026-11-15 - `3fb2d31` Add commit-linked handoff journal

- Renamed `TODO.md` to `NEXT.md` for the actionable work queue and corrected
  the misspelled decision-journal filename to `DECISIONS.md`.
- Added this append-only handoff log and documented the three files in README.
- Updated the README's editor description to match the current media and tags
  fields, live preview, and disabled publishing state.
- The editor remains a Development-only prototype; it does not save drafts.
- No application code changed; tests were not run for this documentation-only
  work.
- Next: design author authentication and the Turso data model before building
  protected journal management. See `NEXT.md` for the current queue.

## 2026-11-15 - `1330ccd` Add session theme toggle on landing page

- Added a double-click gesture on the landing page's empty background to switch
  between light and dark themes. Interactions with navigation and content do not
  trigger the toggle.
- The choice is stored in `sessionStorage`, restored across site navigation,
  and expires with the browser tab session. The system preference remains the
  default when no explicit choice has been made.
- Updated shared theme tokens and color-scheme handling, including reduced-motion
  support for the transition, and documented the behavior in README and
  `DECISIONS.md`.
- No tests were run.
- Next: design author authentication and the Turso data model before protected
  journal management; see `NEXT.md`.

## 2026-11-15 - Decision traceability workflow

- Decision ID: `D-20261115-001`.
- Decision record commit: `0b5da2d` (`Document commit-linked decision workflow`).
- This change documents the workflow; it has no separate application
  implementation commit.
- New decisions receive stable IDs. Related implementation commits reference
  those IDs, and this log records both sets of hashes after the commits exist
  locally and before they are pushed.
- Use `git revert <implementation-hash>` to undo implementation while keeping
  the decision rationale and commit trail.
