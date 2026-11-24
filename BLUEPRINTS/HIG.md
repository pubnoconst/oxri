# Res Imperii — Human Interface Guidelines

How the site looks, moves and speaks. Read this before adding or changing any screen. Where this guide and the code disagree, the code is what ships; fix whichever one is wrong, in the same PR.

The reference reader's device is a **budget Android phone (Samsung Galaxy A02s)**. Every rule below holds on that phone first.

---

## 1. Principles

1. **Subtlety is the key.** Nothing shouts. Emphasis comes from weight, position and space, never from bright colour or size jumps.
2. **Depth, not decoration.** Every surface sits at one depth, and its height is shown by a surface colour with the shadow that goes with it, never by texture or a translucent tint (§4).
3. **Waiting is a dot matrix.** One quiet 3×3 grid, in three motions that each mean something (§6). Nothing else animates to fill time.
4. **Plain words.** Say what is happening in the reader's terms ("Confirming, please wait.", "Make Room"), never in the system's ("POST failed", "quota exceeded").
5. **Cheap to draw.** Animate only `opacity` and `transform`. No SVG filters over content and no animated layout properties (`height`, `top`, `width`). Bound every list (pagination).

---

## 2. Typography

| Role | Face | Token |
|---|---|---|
| Everything readable (headings, body, buttons, labels) | EB Garamond, fallback Georgia, serif | `--font-serif` (`--font-sans` aliases it) |
| Counters, byte figures, codes | system monospace | `--font-mono` |

- **One family.** There is no sans-serif. Hierarchy comes from size, not from switching faces.
- **No bold. Bold is not the answer; bigger text is.**
  - Everything is set in weight 400. It's the only weight loaded (with its true italic), and `font-synthesis-weight: none` stops the browser faking a heavier one, so a default bold (`strong`, `b`, `th`, a heading) stays regular. `TestNoBold` fails on any other weight.
  - **To make something stand out, make it larger** (the space-to-free figure on Make Room is 1.2× its sentence), or give it full ink beside dimmer text (a clipping's author name beside the handle).
  - **Strong emphasis in prose is small capitals:** `**Causal Succession**` reads in small caps, the book way to stress a phrase. Lighter emphasis is italic (`*…*`).
- **Quoted matter is smaller and softer.** A post clipping's text is 0.93rem in ink mixed 84% toward the slip, against the author's own words at 1rem and up. EB Garamond has no weight below 400, so "lighter" always means size and ink, never a thinner face or opacity.
- **Buttons** are set at 0.95rem with a 0.02em letter-spacing; small buttons at 0.8rem.
- **Monospace is for figures that line up**, such as `0 / 3`, `167 / 2000` and `1.7 KB`. Use tabular numbers wherever digits change in place.

## 3. Colour

The palette is a warm neutral page and ink, plus three accents used only for state.

| Token | Light | Dark | Use |
|---|---|---|---|
| `--page` | `#f5f4f0` | `#0f1113` | page ground |
| `--page-ink` | `#34332f` | `#d6d6d3` | all text, the dot matrix, the primary button's fill |
| `--accent-active` | `#3b6e44` | `#87b382` | on and positive state (published, live) |
| `--accent-warn` | `#c98a2c` | `#e6b450` | caution |
| `--accent-danger` | `#c74e4a` | `#e06c68` | destructive actions and errors |

- **Surfaces and borders are derived** from page and ink with `color-mix`: `--depth-sunk-surface` (and the other depth surfaces, §4), `--border-subtle`, `--border-card`, `--border-input`, `--border-focus`. Never pick a new grey; derive it.
- **Accents never decorate.** A red that isn't destructive or an error, or a green that isn't "on", is a bug.
- **Both themes are first-class.** The theme follows the system. A double-click on the page background toggles light and dark for the tab (`theme-toggle.js`, kept in `sessionStorage`). Every colour must come from a token that both themes define.

## 4. Surfaces, space and shape

- **Depth.** Every surface sits at one depth. Each depth is a pair of tokens, a surface colour and the shadow that goes with it, defined for both themes in `site.css`. Use the pair together, and never invent an in-between depth.

  | Depth | Surface | Shadow | For |
  |---|---|---|---|
  | sunk | `--depth-sunk-surface` | none | wells cut into a surface: inputs, code, quoted posts |
  | 0 | `--depth-0-surface` | none | the page |
  | 1 | `--depth-1-surface` | `--depth-1-shadow` | cards and panels, lifted off the page; buttons |
  | 2 | `--depth-2-surface` | `--depth-2-shadow` | things laid on a card: post clippings, embedded media |

  - **Light comes from above.** In light, each step up is a little nearer white. In dark, each step up is a little lighter (4.5% toward white at depth 1, 8% at depth 2), because dark interfaces show height that way.
  - **The shadow carries the height.** Depth 1's is long and soft (the card floats); depth 2's is short and close (laid on the card).
  - **Borders are for edges, not depth.** A hairline `--border-*` may outline a surface, but height comes only from the pair.
  - *History:* surfaces were once woven linen (SVG noise), introduced so the water ripple would read as real without an artificial shadow. The ripple was retired (RATIONALE 047), so the linen went too (RATIONALE 059).
- **Posts from other sites are clippings** (`views/tweet`): a paper slip laid on the card, the site and date in small capitals along the top, the author signed under a dashed rule with "View on … ↗". Sites are named in text, never with their logos. A direct link to an image, video or audio file is not a clipping: it stays plain framed media.
- **Radii:** cards `--radius-card` (0.5rem), controls `--radius-control` (0.35rem), chips and pills `--radius-pill`.
  - **Everything laid on a card** (depth 2: a post clipping, a raw image, video or iframe, a link card) uses `--radius-card`, the same as the card, so a clipping and a raw media item beside it read as the same kind of object.
  - **Anything nested inside those** (the images and the quoted post inside a clipping) steps down one, to `--radius-control`.
  - **Surfaces take radii only from the tokens** (or `50%` for a circle). `TestRadiiComeFromTokens` fails on any other literal, apart from two named exceptions:
    - X's own embed (`.tweet-embed-container`, 14 px), which matches the frame X draws inside its iframe;
    - the checkbox's drawn parts (its box and the indeterminate dash), which are a glyph, not a surface.
- **Space:**
  - page padding: `--space-page-y` / `--space-page-x` (fluid);
  - card padding: `--space-card-pad` (1.65rem), or `--space-card-pad-sm` (1.15rem) on small cards;
  - header: `--space-header-top` / `--space-header-bottom`.
  - Lay out siblings with flex or grid `gap`, not per-element margins.
- **Phone width first.** Rows wrap; nothing is wider than the screen.

## 5. Controls

### Links and buttons: which is which

**A link goes somewhere; a button does something.** If following it changes the address and nothing else, it's a link. If it changes state (saves, deletes, publishes, signs out, filters a form), it's a button. Each kind has exactly one look and one response, defined once in `site.css` (§3b) and used by class, never restyled per page.

| Kind | Class | At rest | Hover / focus | Pressed | Use |
|---|---|---|---|---|---|
| Prose link | (automatic inside entries, books) | underlined, full ink | underline thickens | — | links written into content |
| Plain link | `.link-plain` | full ink, no underline | underline appears | — | primary navigation: the landing menu, "Return home" |
| Quiet link | `.link-quiet` | ink at 65% (`--quiet-opacity`) | full ink, underline appears | — | page chrome: Back, Author sign in, Clear, an entry's date, "View on X", "More tags", "Or sign in with…" |
| Button | `.btn` + kind (below) | a control on depth 1 | tints, edge darkens | sinks 1 px | actions |
| Pill | `tag-filter-pill`, `tag-suggestion-pill` | small rounded control | tints 6% ink, edge darkens | — | filters and suggestions |
| Whole card | `.journal-card.is-collage` | a card | edge darkens | — | a card that opens its entry |

- **Nothing moves or grows on hover or focus.** No scaling, no sliding, no lifting. The only movement is a button sinking 1 px while pressed. `TestHoverAndFocusNeverMove` fails the build otherwise.
- **One focus ring for everything you can press or follow:** a 2 px `--border-focus` outline, 2 px out (`:where(a, button, summary, …):focus-visible`). Text fields use their focus border instead (§5 Forms).
- **One timing:** `--interact-duration` (140 ms), colour and opacity only.
- **Exceptions, deliberately:**
  - A state-changing action in the page chrome (Sign out, Log out) is a `<button>` wearing `.link-quiet`, because it belongs with the links around it but must POST.
  - A navigation that is the main action of a toolbar ("Add new entry", Edit, pagination) is a link wearing `.btn`, because it sits among buttons.
  - In both cases the element is chosen by what it does, and the class by where it sits.

### Buttons

There are three kinds, ranked by how strongly they invite a click. Use **at most one encouraged button per view.**

| Class | Look | Use |
|---|---|---|
| `.btn-encouraged` | solid ink, page-coloured text | the one action the page exists for: "Publish", "Save entry now" |
| `.btn-neutral` | card surface, quiet border | everything else: Edit, Bookmark, Search, pagination |
| `.btn-destructive` | card surface, danger text and border | Delete, Delete all, Delete selected |

- `.btn-sm` is for toolbars and card footers.
- **Hover:** neutral tints 6% ink and its border darkens; destructive tints 10% danger and its border turns danger; encouraged deepens its fill. No opacity changes, no movement.
- **Pressed** nudges the button down 1 px. **Disabled** is 45% opacity with a not-allowed cursor; a disabled control stays in place so the layout doesn't shift.
- **Labels are verbs naming exactly what happens:** "Publish selected", "Clear space automatically", "Save Book II now". No "OK", "Submit" or "Continue".

### Links and navigation

- **Back:** `layout.BackLink(href, label)`. A quiet link: a chevron plus the destination's name ("Journals", "Back").
- **Pagination:** "← Newer · Page n of m · Older →" under the feed, only when there is more than one page.
  - Entries are newest first, so the previous page is "Newer".
  - A disabled end keeps its place.
  - Page links keep the search and tag filters; changing a filter returns to page 1.
- **After an action, return the reader to where they were:** the same catalog page and filters.

### Forms

Every field follows the same anatomy, top to bottom:

1. **Label row:** the label on the left, a live **counter** on the right in monospace (`n / max`). The counter turns red when the value is over the limit.
2. **Input**, with `maxlength` wherever a limit exists. The limit comes from the Go constant, and JS reads it from the attribute: one source of truth.
3. **Hint:** the rule in one plain sentence ("Up to 2,000 characters.").
4. **Error:** under the hint, `aria-live="polite"`, empty until needed. It says what is wrong and the limit, for example "Too long: 2100 characters. The limit is 2000."

- **Placeholders say what to type, never example data.** For example: "Tag names, separated by commas", "https://…", "Search entries…". Example values in a placeholder read as real data and get mistaken for defaults.
- **Suggestions are real data.** For example, the tag "Suggestions:" pills list tags that already exist.
- **The server re-validates everything.** Client checks are a courtesy.

### Tags

- Up to **3** unique tags per entry, each up to **8** ASCII letters or digits (A–Z, 0–9), separated by commas, stored in Title Case.
- Tags show as quiet chips (`journal-tag-chip`) and as filter pills (`tag-filter-pill`, `is-active` when selected).
- **Tags are ordered by use** (`db.TagsByUse`), most used first. The catalog shows the 12 most used (`journals.CatalogTagLimit`) plus any selected tag; the rest wait, alphabetical, behind "More tags (N)" (`<details>`). The editor's suggestions lead with the most used ("Most used:" while the field is empty), then, while typing, tags that start with the text before tags that contain it.

## 6. Loading: the dot matrix

Waiting is shown with exactly one thing: a 3×3 matrix of 4 px dots, in `--page-ink`, beside a short label. It is drawn by `assets/js/dot-matrix.js` (`ResImperiiDots`) and by Go's `views/loading`.

**The motion is the message.** Pick it by what the reader is waiting for:

| Motion | Means | Use for | Label |
|---|---|---|---|
| **Sliding puzzle** (seeded) | the site is working on something you did | saving or editing an entry, Make Room, signing in, saving a book, any POST | "Confirming, please wait." / "Working" |
| **Twinkle** (seeded) | something is downloading | tweet previews, the catalog, following a link | "Loading preview" / "Loading entries…" / "Loading" |
| **Comet** | it is your turn | waiting for a passkey or security key | "Waiting for your Passkey…" |

- **Seeded** means each instance differs: a page of cards loading together never flashes in unison. Pass a stable seed when one exists (the tweet ID); otherwise a random seed is used.
- **In Go,** use `loading.Dots(label, loading.Twinkle)`, or `loading.Puzzle` / `loading.Comet`. The type admits no other motion.
- **In JS,** use `ResImperiiDots.create(label, "twinkle", seed)`.
- **Where the loader sits:**
  - **Inline**, where the result will appear: the tweet slot's centred "Loading preview" line.
  - **Overlay:** "Confirming, please wait." covers the page for form saves (`data-confirm-loading`).
  - **Navigation:** the page dims to 60% at once. If the next page hasn't arrived after 180 ms, a small matrix appears at the bottom.
  - **Page still loading its content** (the journals catalog waiting on the database): the loader stands in that same bottom-centre spot (`.page-dot-loading`), never at the top of the page, so following a link and waiting for the content read as one continuous wait.
- **No box and no shadow around a loader.** A placeholder card is never drawn for content that hasn't arrived.
- **Reduced motion:** the dots stand still at 60% opacity, and the label carries the meaning.

## 7. Motion

| What | How | Timing |
|---|---|---|
| Content arriving (a tweet) | FLIP: the slot opens in one layout, then unrolls (bottom clip) while the content below slides from its old place | 460 ms, `cubic-bezier(0.22, 1, 0.36, 1)` |
| Page to page | cross-document view transition: old fades out, new eases in | 160 ms out, 220 ms in |
| Control feedback | colour, opacity, an underline, or a 1 px press (§5 Links and buttons) | 140 ms (`--interact-duration`) |
| Navigation received | page dims to 60% | immediate |

- **Arrivals push; they never pop.** Nothing appears at full size over existing content, and no content jumps.
- **Interrupted motion continues from where it visibly is,** never from where the layout says it is (RATIONALE 047).
- **`prefers-reduced-motion`:** animations collapse to their end state.

## 8. Voice and copy

- **Write from the reader's side.** For example, "Your journal archive is full", "your changes to Book II will be saved automatically".
- **Short, active, specific.** Name the thing and the number: "Remove a few older journal entries to free up at least 1.7 KB".
- **Errors say what is wrong and how to fix it.** For example: "Must be a valid http:// or https:// URL.", "Too long: 2100 characters. The limit is 2000."
  - No apologies, no blame, no codes.
  - **Server error pages** show a plain sentence and a reference ID, never internals.
- **Destructive confirmations name the consequence:** "Delete this journal entry permanently?"
  - They are asked in place (`views/confirm`, fetched with HTMX into a slot beside the control), never with the browser's `confirm()`. "Keep" takes focus, so a stray Enter never confirms; Keep or Escape closes the prompt.
  - The server refuses a destructive POST that doesn't carry `confirmed=1` from the prompt's own button.
- **Units and numbers:**
  - characters, counted as Unicode characters (Greek and accented letters count as one);
  - bytes, formatted B / KB / MB / GB;
  - thousands with a comma (2,000);
  - en dashes in ranges (1–128).
- **Names:**
  - the site is "RES IMPERII" in titles and "Res Imperii" in prose;
  - the sections are Journals, Book I, Book II and Book III;
  - the space-freeing page is "Make Room".

**Known inconsistencies to align** when next touched:
- **Two "Required:" messages** in the editor read as labels, not sentences ("Required: journal entry cannot be empty."). Prefer "Write something before publishing."
- **Some error-page sentences are formal** ("The submitted payload exceeds the permitted size limit."). Prefer "That's too large to save."

## 9. Accessibility

- **Status regions:** loaders and live messages use `role="status"` or `aria-live="polite"`. The dot matrix itself is `aria-hidden`; the label speaks.
- **Focus is always visible:**
  - **Text fields** swap the outline for a `--border-focus` border.
  - **Everything else you can press or follow** (links, buttons, pills, summaries, checkboxes) shares one ring: a 2 px `--border-focus` outline, 2 px out (§5 Links and buttons).
  - **Never `outline: none` without a replacement.**
- **Not colour alone:** state always has a word or shape too (a counter, a label, `aria-pressed` on tag pills).
- **Reduced motion is honoured everywhere** (§6, §7).
- **Touch targets:** buttons are at least 2.6rem tall; `.btn-sm` is only for dense toolbars.

## 10. Performance as design

- **Paginate:** 12 entries per catalog page (`journals.CatalogPageSize`). Never render an unbounded list.
- **Animate cheaply:** only opacity and transform, which the compositor runs even while the main thread is busy with tweets.
- **No SVG filters on content,** and none over cross-origin iframes (RATIONALE 039).
- **Measure on the phone:** a fast laptop hides problems. Test on the S9+ with resolution set to HD+, with "Profile GPU rendering" on.

## 11. Checklist for a new screen

- [ ] Colours come from tokens, and the screen reads in both themes.
- [ ] At most one encouraged button; every label is a verb naming what happens.
- [ ] Fields follow label, counter, input, hint, error, and placeholders hold instructions, not example data.
- [ ] Every wait uses the dot matrix with the motion that matches its meaning, and has a label.
- [ ] Only opacity and transform animate, and reduced motion is honoured.
- [ ] Lists are bounded, and the screen works at 360–400 px wide.
- [ ] The copy is plain, active and specific; errors say how to fix the problem.
