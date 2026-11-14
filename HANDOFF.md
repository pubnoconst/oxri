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
