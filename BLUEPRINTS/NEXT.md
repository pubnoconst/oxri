# Next

- **Base Commit on `main`**: `ea9247b` (PR #24 — settings moved to `/settings`, `/pilot` removed)
- **Active Branches**:
  - `v2-rationale-reconstruction` (PR #25): RATIONALE 015–027 reconstructed from git/GitHub history; Principia #7 now requires git-sourced dates and full timestamps.
  - `v2-tweet-reveal` (local, awaiting UI green light): fixes cut-off tweets in the catalog and the two-phase tweet pop-in.
- **Active Experiment / Undo Instructions**: None. `v2-tweet-reveal` is uncommitted working-tree changes only; `git restore .` on that branch discards them.

## Tweet reveal (in progress on `v2-tweet-reveal`)

- Measured Twitter's postMessage lifecycle: `initialized → results → resize(h) + rendered`. `createTweet()` resolves with the iframe already at its exact height; photo tweets send one more `resize` once media loads.
- No settle timer: the slot animates from the ripple spacer to the iframe's height as soon as it exists, then the tweet eases in. Later resizes retarget the same height transition, so they push the body smoothly instead of jumping.
- Removed the collage `max-height: 22rem` clip; `widgets.js` downloads in parallel with the Turso fetch.

## Next on Mind

1. UI green light on `v2-tweet-reveal` → PR, merge, sync `prod`/`v2`, RATIONALE 028.
2. Run `mage backup` and author Books II and III via `/book-ii/edit` and `/book-iii/edit`.
