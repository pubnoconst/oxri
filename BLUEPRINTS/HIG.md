# Res Imperii — Human Interface Guidelines

How the site looks, moves and speaks. Read this before adding or changing any screen. Where this guide and the code disagree, the code is what ships; fix whichever one is wrong, in the same PR.

The reference reader's device is a **budget Android phone (Samsung Galaxy A02s)**. Every rule below holds on that phone first.

---

## 1. Principles

1. **Subtlety is the key.** Nothing shouts. Emphasis comes from weight, position and space, never from bright colour or size jumps.
2. **Depth, not decoration.** Every surface sits at one depth, and its height is shown by a surface colour with the shadow that goes with it, never by texture. The one translucent tint is an overlay's frosted glass (§4).
3. **Waiting is a dot matrix.** One quiet 3×3 grid, in three motions that each mean something (§6). Nothing else animates to fill time.
4. **Plain words.** Say what is happening in the reader's terms ("Confirming, please wait.", "Make Room"), never in the system's ("POST failed", "quota exceeded").
5. **Cheap to draw.** Animate only `opacity` and `transform`. No SVG filters over content and no animated layout properties (`height`, `top`, `width`). Bound every list (pagination).
6. **Stable beats peak.** A steady frame rate is worth more than a higher best case; a predictable wait is worth more than a sometimes-faster one. Judge every performance change by the variance it adds or removes, on the A02s, before its best-case speed.
   - **Prefer:** fixed, cheap costs every time (the page's 100 ms opacity fade-in; the shell's instant first paint); work done once, up front, off the scroll (pagination, server-rendered snapshots); anything that removes a jump (fonts served from the site with size-matched fallbacks, so text never reflows when a font arrives; content in the first response instead of popped in later).
   - **Avoid:** speed that only sometimes happens (speculative prerendering: instant when it guesses right, slower and busier on a budget phone when it doesn't); work moved into scrolling (`content-visibility: auto`, which lays content out as it scrolls in and makes the scrollbar jump); anything whose timing depends on the network once the page is showing.
   - **Measured, not argued.** The yardstick is Google's Core Web Vitals at the 75th percentile of page views, mobile first: LCP ≤ 2.5 s, INP ≤ 200 ms, CLS ≤ 0.1. Field data from readers' browsers (Settings → Field Performance) is the evidence; a change that may move them is compared before shipping with the lab profile (`mage lab`), reading the spread and worst run as well as the median (PROFILING.md).
   - **The worst variance is the one the reader waits through:** a cold server that is sometimes instant and sometimes a long wake-up outweighs any front-end gain.

7. **Pages arrive whole and still (owner, 2026-11-25).**
   - **No sudden displacement.** Nothing moves once it is drawn. Content that arrives after the page gets its final space first: from what it is (a video is 16:9; an image sits in a fixed frame, fitted inside), so the text around it never shifts. CLS stays at 0; the CI performance job fails on any shift over 0.1.
   - **New pages ease in at once.** One response carries the page and its content (never an empty frame waiting on a second request: RATIONALE 076), and it appears with the short page fade-in.
   - **Content eases in stably.** Anything that arrives later fades in (opacity) in the space already reserved for it. Never unroll it by animating height: that relays the page out on every frame, which is the stutter §1.6 rules out on a budget phone.
   - **Nothing plays until it is tapped (owner, 2026-11-26).** A video, a GIF or an embedded player (YouTube, Vimeo) first appears as a still tile with the play mark, in its reserved space. The still is a thumbnail the server made once (internal/thumb: a GIF's first frame, a video's poster), served from this site, so the reader's browser asks the media's host for nothing until the tap. Until a still exists the tile shows the media's name ("GIF", "Video"). Only the tap loads the player and starts it. An audio player loads nothing until played. A clipping's video or GIF already waits behind its poster frame.
   - **Images come at the size they show.** A catalog card asks X for its small images (680 px), an entry's own page for medium (1200 px). A card's linked image is the server's 680 px still; the entry's own page shows the image itself.
   - **A catalog card's contents are centred** (owner, 2026-11-26): date, media, tags, preview text and actions. A quoted clipping keeps its own left-aligned text.
   - **The theme never flashes.** The light/dark switch sits at the top right of every page (`layout.ThemeToggle`: the moon in light mode, the sun in dark, the theme a tap switches to; a 44 px target). Its row is reserved (`--top-reserve`), so it never overlaps a page's own controls on a phone, and pages sized to the screen subtract it (`--full-height`). The first visit follows the device; a tap is remembered (`localStorage`) and applied before the first paint by the one inline script (`assets.ThemeInitJS`, allowed by its hash in the CSP). The double-click on empty background also switches.
   - **Every tap is acknowledged at once, by the control itself.** The tapped link or button turns pending (`.is-pending`, opacity) the moment it is pressed; the dots follow only if the wait passes 180 ms. The page is never dimmed.

---

## 2. Typography

| Role | Face | Token |
|---|---|---|
| The books: book pages, the landing's title and book links | EB Garamond, Latin and its own Greek (family `Res Book`) | `--font-serif` |
| The site's own UI: journals, login, settings, editor, buttons, labels | Figtree; all Greek from Didact Gothic (`Res UI`) | `--font-ui` (`--font-sans` aliases it) |
| An entry's own words, wherever they appear: the full entry, catalog excerpts, Make Room's list, the editor preview | Playpen Sans; all Greek from Didact Gothic (`Res Entry`) | `--font-entry` |
| Quoted matter: tweet clippings, Bible passages, blockquotes in entries | Short Stack; all Greek from Didact Gothic (`Res Quote`) | `--font-quote` |
| Counters, byte figures, codes | system monospace | `--font-mono` |

- **The face says who wrote it.** The books are set like a book; the site speaks in a plain sans; the author's entries in a casual hand; other people's words in a different hand. Within a role, hierarchy still comes from size, not from switching faces.
- **Greek: one face per phrase.** Each role is one family assembled in `assets/fonts.go` by `unicode-range`: in the three sans roles every Greek letter, plain or polytonic, comes from Didact Gothic, so ἐν ἀρχῇ ἦν ὁ λόγος never changes face midway. The book face uses EB Garamond's own Greek.
- **Self-hosted, never a third party.** The fonts are Google Fonts' own subsets of OFL families (licences beside them in `assets/fonts/`), served from the site with content-hashed URLs cached for a year. `fonts.css` is generated, so no font URL is written by hand. Each face loads only when the page has text that needs it. None is preloaded: in the lab a preloaded face held the text that uses it back to 4.6 s (PROFILING.md). The CSP allows fonts from `'self'` only.
- **Every reader sees the site's own typefaces, first visit included (owner, 2026-11-25).** Every face is `font-display: swap`: text is drawn at once in the matched fallback, and the web font replaces it as soon as it arrives, in the same space (below), so nothing visibly moves; it costs one relayout frame at load. `font-display: optional` was tried and reverted: it kept a first visit in the fallbacks (RATIONALE 079, 082). No face may leave `font-display` unset (`TestFontFacesAreRegularAndHashed`).
- **Fallbacks match the web fonts' size** (§1.6). Each role has a chain of fallback faces, one per system font family a reader may have, each resized with `size-adjust`, `ascent-override` and `descent-override` to the web font's metrics measured against that system font: Georgia, Times New Roman (also Linux's Liberation Serif) and Noto Serif (Android) for the book; Arial (also Helvetica and Liberation Sans) and Roboto (Android) for the sans roles. One face tuned to one system font is wrong on another: tuned only to Georgia, the book shifted 0.194 on Linux in CI. The role stacks are generated with the faces in `fonts.css`, so they cannot drift. A paragraph set in the fallback takes the same height as in the web font (checked for all four roles), so a page set in the fallbacks takes the same space as in the web fonts. Re-measure (method in `assets/fonts.go`) whenever a face changes.
- **No bold. Bold is not the answer; bigger text is.**
  - Everything is set in weight 400. It's the only weight loaded (with its true italic), and `font-synthesis-weight: none` stops the browser faking a heavier one, so a default bold (`strong`, `b`, `th`, a heading) stays regular. `TestNoBold` fails on any other weight.
  - **To make something stand out, make it larger** (the space-to-free figure on Make Room is 1.2× its sentence), or give it full ink beside dimmer text (a clipping's author name beside the handle).
  - **Strong emphasis in prose is small capitals:** `**Causal Succession**` reads in small caps, the book way to stress a phrase. Lighter emphasis is italic (`*…*`).
- **Quoted matter is smaller and softer.** A post clipping's text is 0.93rem in ink mixed 84% toward the slip, against the author's own words at 1rem and up. It is also set in its own face (`--font-quote`). Only weight 400 is loaded, so "lighter" always means size and ink, never a thinner face or opacity.
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
  | overlay (float glass) | `--depth-overlay-surface` + `--depth-overlay-blur` | `--depth-overlay-shadow` | things laid over part of the page: a select's list, the Book list |
  | overlay (veil glass) | `--depth-veil-surface` + `--depth-veil-blur` | none | full-screen overlays: the Confirming screen |

  - **Every overlay is frosted glass. No exceptions.** Anything that floats above the page (a list that opens, a menu, a full-screen wait) is glass; nothing floats solid. Lists open as overlays over the content below, not in place pushing it down.
  - **There are exactly two glasses, and they are the only blur on the site.** Both are one kind: a Gaussian blur of what lies beneath (`backdrop-filter: blur()`) plus a gentle saturation lift (`--glass-saturate`, 1.2) so colour behind stays alive instead of going grey, under a translucent tint. Glass is warm like the paper: anything it is lifted toward is `--glass-light` (`#f5f0e6`), never pure white, which reads cooler than the site. Use them only through their tokens; `TestBlurComesFromGlassTokens` fails on any other `backdrop-filter` (and on `filter: blur`).

    | Glass | For | Tint (intensity) | Blur radius | Saturation | Edge and shadow | Fallback without blur |
    |---|---|---|---|---|---|---|
    | **Float** (`--depth-overlay-*`) | things laid over part of the page: a select's list, the Book list, menus | the depth-2 surface lifted 14% toward `--glass-light`, at 45% | 5 px | 1.2 | `--depth-overlay-edge` hairline (14% `--glass-light` into the input border); `--depth-overlay-shadow` with a 10% `--glass-light` top highlight | `--depth-overlay-solid` (the depth-2 surface) |
    | **Veil** (`--depth-veil-*`) | full-screen overlays: the Confirming screen | the page colour at 62% | 14 px | 1.2 | none: it fills the screen | `--depth-veil-solid` (the page) |

    - **Why these numbers.** Over a plain card, a denser tint or a blur past about 8 px reads as solid, so a float is light enough that the fields beneath show as soft shapes. A veil covers text the reader must not try to read, so it is denser and blurred past legibility.
    - **Warmth is checked by number.** The glass over a card must be no cooler than the card: red minus blue of the composite within 1 of the card's (light: card +2, glass +3; dark: card −4, glass −3). A saturation lift amplifies the backdrop's cast, which is why it stays at 1.2.
    - **Radius of corners** is unchanged by glass: floats take `--radius-control`, like the controls they open from.
    - **Cost.** Blur only overlays: they are small or short-lived, so the cost on a budget phone is bounded. Never blur a card, the page, or anything that stays on screen while scrolling.
    - Use the `@supports (backdrop-filter: blur(1px))` guard: the solid fallback first, the glass inside the guard.
  - **A list that opens from a field is connected to it**, never floating apart: one outline runs around the field and the list. While open, the field takes `--border-focus`, drops its lower corners and its focus ring; the list hangs flush below it (no gap), drops its upper corners, shares the side borders and is divided from the field by a `--border-subtle` hairline. It opens downward only, as tall as the room below allows, scrolling for more; it never flips above the field, which would detach it.
  - **A select's list is drawn by the page** (`appearance: base-select`, Chrome 135+), never left to the system's blue highlight; the chosen option is marked with ✓ at the end, as in the book list. Browsers without it keep the system list.

  - **Light comes from above.** In light, each step up is a little nearer white. In dark, each step up is a little lighter (4.5% toward white at depth 1, 8% at depth 2), because dark interfaces show height that way.
  - **The shadow carries the height.** Depth 1's is long and soft (the card floats); depth 2's is short and close (laid on the card).
  - **Borders are for edges, not depth.** A hairline `--border-*` may outline a surface, but height comes only from the pair.
  - *History:* surfaces were once woven linen (SVG noise), introduced so the water ripple would read as real without an artificial shadow. The ripple was retired (RATIONALE 047), so the linen went too (RATIONALE 059).
- **Entry cards are frozen (owner, 2026-11-25).** Signed in, every card in the catalog carries the author's actions in its footer: **Edit**, **Bookmark/Unbookmark**, **Publish/Unpublish**, **Delete** (its prompt opens in place). An entry is managed from the catalog without opening it, and every action but Edit returns to the same catalog page, with its search, tags and page kept. Readers see none of these. Change the card's look freely within this guide, but not this set of actions or where they return. `TestCatalogCardsKeepTheirActions` fails if any is lost.
- **Posts from other sites are clippings** (`views/tweet`). Sites are named in text, never with their logos. A direct link to an image, video or audio file is not a clipping: it stays plain framed media.
  - *What ships today:* a paper slip laid on the card, the site and date in small capitals along the top, the author signed under a dashed rule with "View on … ↗".
  - *Next design (owner, 2026-11-27; prototyped on `card-preview-ideas` at tag `preview-cards-v1`, `d5e8e63`, queued in `NEXT.md`):* **universal preview cards and clippings.** Every element keeps its relative position, and a card grows strictly as it holds more content:
    1. **Universal top-left author anchor:** inside a post clipping, the circular avatar and stacked `Name` / `@handle` (with a quiet external `↗` on the right) always sit at the **top-left**. There is no duplicate post date inside the card and no bottom dashed-rule byline.
    2. **Three post archetypes:**
       - **Caption (`media` and `≤ 80` characters, including media-only posts — `1C` full-bleed overlay):** the media fills the clipping edge to edge. Both the top-left frosted author pill and the bottom caption scrim stay hidden (`opacity: 0`) at rest so the image stands uncluttered; they ease in (§7) on cursor hover, keyboard or tap focus (`:focus` / `:focus-within`), or when scrolled into the central viewport focal band on touch displays (`(hover: none), (pointer: coarse)`), and ease back out on leave.
       - **Newspaper snippet (`media` and `> 80` characters):** identical to a pure statement—same top-left author row, same right-hand description—adding only a compact square thumbnail (`~4.85rem`, `--radius-control`, with a play mark for video/GIF or `1/N` count for multiple photos) on the left.
       - **Pure statement (no media):** circular avatar and `Name` / `@handle` at the top-left, with full-width statement prose directly underneath.
    3. **Outer catalog card (`views/journals`):** collapses into three left-aligned layers that grow only when present: (1) quoted clipping, passage, server still, or link slip at the top, (2) the entry's excerpt (`--font-entry`), and (3) one quiet single-line footer (`Date · Tags` on the left, subtle `Draft` and `★` on the right), followed when signed in by the frozen author action bar above.
- **Full journal entries — newspaper reading layout** (*owner, 2026-11-27; prototyped on `card-preview-ideas` at tag `newspaper-entry-v1`, `a6bd1c5`, queued in `NEXT.md` for `/journals/{id}`*):
  - **Default — newspaper float (`wrap-left`):** inside a `display: flow-root` stage, the unclamped source material sits at the **top-left** (`float: left`, `clamp(15.5rem, 43%, 22.5rem)`), and the author's commentary starts immediately to its **right** on the same top horizon. Once the prose clears the bottom of the source, the remaining paragraphs expand across the **full 100% horizontal measure** underneath. On narrow phone viewports (`< 680 px`), the float releases to a single column.
  - **Exception 1 — media with a rich description (`stack-rich-source`):** when the source combines media with a rich description (`> 80` characters, or media plus a quoted post), the source itself already pairs a left plate with right-hand text across the top, so the author's commentary starts cleanly **at the bottom** across the full width.
  - **Exception 2 — early code body (`wrap-compact-code` vs. `stack-early-code`):**
    - When a `<pre><code>` block appears early (before `< 420` characters of prose have cleared the float) **and the source is concise** (`≤ 140` characters, no media), the top-left source slip (`clamp(12.5rem, 29%, 15.25rem)`) and the opening prose + `<pre><code>` column (`~71%`) pair inside a stretched two-column band (`align-items: stretch`): the source card stretches to match the bottom edge of the code block, its contents sit **centred vertically inside the card** (`justify-content: center`), and any prose past that code block spans the **full 100% width** below.
    - Only when an early `<pre><code>` meets a **long (`> 140` characters) or media-bearing source** that cannot compress cleanly does the commentary start at the bottom (`stack-early-code`).
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
- **In Go,** the matrix is drawn by `views/loading` (the Confirming overlay); its `Motion` type admits only the puzzle and the twinkle. The comet is only ever started in the browser, by `passkey.js` while a passkey prompt is open, so it has no Go handle; add one only when a server-rendered page needs it.
- **In JS,** use `ResImperiiDots.create(label, "twinkle", seed)`.
- **Where the loader sits:**
  - **Inline**, where the result will appear: the tweet slot's centred "Loading preview" line.
  - **Overlay:** "Confirming, please wait." covers the page for form saves (`data-confirm-loading`).
  - **Navigation and in-place results** (the catalog's search, tags and pages): the tapped control turns pending at once (§1.7). If the next page or the new results haven't arrived after 180 ms, a small matrix appears at the bottom centre. The page is not dimmed; in-place results stay visible until replaced, and dim only if the answer is slow.
- **No box and no shadow around a loader.** A placeholder card is never drawn for content that hasn't arrived.
- **Reduced motion:** the dots stand still at 60% opacity, and the label carries the meaning.

## 7. Motion

| What | How | Timing |
|---|---|---|
| Media arriving after the page (an image, a video, an audio player) | eases in (opacity) in the space reserved from its kind | 300 ms |
| Caption clipping's author pill and caption (`1C`, queued) | hidden at rest; eases in on cursor hover or touch/keyboard focus, eases out on leave (`opacity` + 4 px `transform`) | 240 ms, `cubic-bezier(0.22, 1, 0.36, 1)` |
| A post on X with no stored snapshot (rare: snapshots are taken on save) | X's embed, whose height is unknown: the slot opens in one layout, then unrolls (bottom clip) while the content below slides from its old place (FLIP) | 460 ms, `cubic-bezier(0.22, 1, 0.36, 1)` |
| Page to page | cross-document view transition: old fades out, new eases in | 160 ms out, 100 ms in |
| Results swapped in place (search, tags, pages) | replaced in a single frame, no fade | none |
| Control feedback | colour, opacity, an underline, or a 1 px press (§5 Links and buttons) | 140 ms (`--interact-duration`) |
| Tap received | the tapped control turns pending (opacity 0.55) | immediate |

- **Arrivals never displace (§1.7).** Their space is reserved and they ease in. The one exception is X's own embed, whose height cannot be known: it unrolls rather than popping. Nothing appears at full size over existing content.
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
