# Session handoff log

This file is an append-only log for passing work between sessions. Git commits
are the source of truth for code and document changes; entries here summarize
the state and point to the commit that contains the work being handed off.

## How to add an entry

1. Review and commit the work for the session.
2. Append a dated entry that names that commit and records outcomes, remaining
   work, and any issue the next session should know about.
3. Commit the appended entry and sync both commits with the remote.
4. Do not edit or remove older entries. Add a new entry to correct or supersede
   an earlier handoff.

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
