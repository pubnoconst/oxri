# TODO

Carry these items into the next session:

- Polish the Journals UI. Keep the project's extreme minimalism and review the
  catalog, empty state, and editor flow together.
- Design author authentication before implementing protected journal actions.
- Design the Turso data model, search index, storage limit behavior, and
  migrations before connecting persistence.
- Review the resulting changes, then create a Git commit and push it.

Current handoff notes:

- The journal editor is a Development-only prototype with two panes and a
  disabled Save entry button. It does not persist drafts.
- The Development-only `/journals` page is an empty state with an Add New Entry
  link to the editor.
- The app has not yet implemented author authentication or Turso integration.
- The working tree has uncommitted changes. Review them before committing or
  pushing.
