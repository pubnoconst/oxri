# Principia

The foundational principles governing **Res Imperii**:

1. **Runtime dependencies are to be avoided as much as possible.**
   - Ship a self-contained static binary (`CGO_ENABLED=0`) free from dynamic linking, external C toolchains, or host runtime requirements. Static assets are embedded (package `assets`).
   - **Single instance by design**: sessions, sign-in counters (holds, per-IP lockout, global failure budget), rate limiters, WebAuthn ceremonies and the pending vacate submission live in process memory. Running two instances would silently split that state, halving brute-force protection and losing sessions between instances; a restart (including a free-tier spin-down) signs everyone out. Scaling out requires moving that state to shared storage first.
2. **Minimalism is the core essence of our UX.**
   - Every view, typography choice, and interaction must remain quiet, restrained, and free of visual clutter or unnecessary controls.
3. **All Inputs are evil. Men are, by nature, political creatures.**
   - Never trust any byte arriving from a browser, URL, form, header, or external source. Enforce strict whitelisting and structural parsing at the boundary before any data enters the system.
4. **The project should comprise of minimal buildsteps.**
   - Compilation, code generation, testing, and execution must be fast, deterministic, and reproducible with a single command.
5. **The users are to never be treated as tech literate. At the same time malicious.**
   - Speak to the user in plain, effortless human language where nothing ever needs to be looked up—while simultaneously assuming every request on the wire is crafted by a determined adversary.
6. **The users should never be able to see what is under the hood.**
   - Never leak stack traces, database names, storage providers, internal error causes, framework details, or raw engineering metrics to the client.
7. **Every major decision and feature flows through a documented Pull Request cycle.**
   - **No Monoliths (Code or Docs)**: Keep both source files and documentation partitioned into small, focused units.
   - **Feature Branches & GitHub CLI PRs**: Develop each decision or feature on a dedicated branch, open a Pull Request via GitHub CLI (`gh pr create`), and merge it (`gh pr merge --merge --delete-branch`) so Git history remains reviewable and self-contained.
   - **Date-Sorted Commit Rationale (`BLUEPRINTS/RATIONALE/`)**: Every merged PR gets an entry, committed inside that PR, at `BLUEPRINTS/RATIONALE/Y<YYYY>/M<MM>/D<D>/<git committer name>/<Chronological Number>-<commit hash>.md`, where `<commit hash>` is the PR's head feature commit.
     - **Dates come from Git, never from the session**: `Y/M/D` is that commit's own committer date as printed by `git log -1 --format=%ci <hash>` (local offset, `+1000`). Work spanning several days lands in several day folders; never file a later commit under an earlier day's folder.
     - **Record the time, not just the day**: the entry header carries `Committed: YYYY-MM-DD HH:MM:SS ±ZZZZ` for the head commit, lists every commit in the PR with its full timestamp, and names the PR number (plus merge hash and merge time when backfilling).
     - `<Chronological Number>` is a single global sequence across all folders (001, 002, …), ordered by commit time.
   - **Session Continuity (`BLUEPRINTS/NEXT.md`)**: Always keep `BLUEPRINTS/NEXT.md` updated with the latest commit hash, what is next on mind, and—if a session pauses mid-experiment—exact instructions on how to undo the experiment.
   - **External Secrets (`../oxri-parent/`)**: Secrets (`db.env`, `pw.env`) and offline backups (`backups/`) live strictly outside the Git repository in `../oxri-parent/`.
8. **Modularization and the Type System: Make erroneous states unrepresentable.**
   - **Context windows are strictly limited**: Large files, sprawling packages, and mental-discipline conventions inevitably fail when context is constrained. Every module must be small, cohesive, and have a single clear responsibility.
   - **The type system is our primary defense**: Do not rely on runtime checks or human memory to catch invalid data or unauthorized actions. Use opaque unexported-field types, boundary parsers ("Parse, Don't Validate"), compile-time capability tokens (`AdminCap`), and sealed outcome types so erroneous states fail at compile time.
   - **Thorough codebase documentation with commit hashes**: Document every module, type invariant, and architectural boundary thoroughly in code comments and rationale entries, referencing commit hashes so humans and agents can easily trace the lineage of every mechanism.


