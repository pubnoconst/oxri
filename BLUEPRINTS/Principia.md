# Principia

The foundational principles governing **Res Imperii**:

1. **Runtime dependencies are to be avoided as much as possible.**
   - Ship a self-contained static binary (`CGO_ENABLED=0`) free from dynamic linking, external C toolchains, or host runtime requirements.
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
   - **Date-Sorted Commit Rationale (`BLUEPRINTS/RATIONALE/`)**: For each commit, record the reasoning in `BLUEPRINTS/RATIONALE/Y<YYYY>/M<MM>/D<D>/<git committer name>/<Chronological Number>-<commit hash>.md`.
   - **Session Continuity (`BLUEPRINTS/NEXT.md`)**: Always keep `BLUEPRINTS/NEXT.md` updated with the latest commit hash, what is next on mind, and—if a session pauses mid-experiment—exact instructions on how to undo the experiment.
   - **External Secrets (`../oxri-parent/`)**: Secrets (`db.env`, `.ssh_host_key`) live strictly outside the Git repository in `../oxri-parent/`.

