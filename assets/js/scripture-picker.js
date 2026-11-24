/*
 * The journal editor's scripture media (views/editor mediaField). The first
 * dropdown chooses Link or Scripture; Scripture, then Bible, shows Book*,
 * From*, To and Translation. The passage is fetched from the provider's API
 * (bible.helloao.org: a chapter file, nothing scraped), shown as the clipping
 * the journal pages draw (views/passage), and its text is put in the hidden
 * verse_text field with the translation used in verse_tr. The server fetches
 * the chapter again on save and refuses the entry if the texts differ
 * (internal/verse), so the rules here mirror it exactly.
 */
(() => {
  const API = "https://bible.helloao.org/api";
  const MAX_VERSES = 40;
  const FETCH_DELAY_MS = 350;
  // internal/verse spaces: every kind of white space.
  const SPACES = /[\t\n\v\f\r \u00a0\u1680\u2000-\u200a\u2028\u2029\u202f\u205f\u3000\ufeff]+/g;
  // internal/verse spaceBeforeClose.
  const SPACE_BEFORE_CLOSE = / ([,.;:!?)\]\u2019\u201d\u00bb])/g;

  /** @param {string} id */
  function byId(id) {
    return document.getElementById(id);
  }
  const kind = /** @type {HTMLSelectElement | null} */ (byId("journal-media-kind"));
  const scripture = /** @type {HTMLSelectElement | null} */ (byId("journal-scripture"));
  const book = /** @type {HTMLInputElement | null} */ (byId("verse-book"));
  const bookSearch = /** @type {HTMLInputElement | null} */ (byId("verse-book-search"));
  const bookList = byId("verse-book-list");
  const from = /** @type {HTMLInputElement | null} */ (byId("verse-from"));
  const to = /** @type {HTMLInputElement | null} */ (byId("verse-to"));
  const choice = /** @type {HTMLSelectElement | null} */ (byId("verse-tr-choice"));
  const used = /** @type {HTMLInputElement | null} */ (byId("verse-tr"));
  const text = /** @type {HTMLInputElement | null} */ (byId("verse-text"));
  const linkGroup = byId("media-link-group");
  const scriptureGroup = byId("media-scripture-group");
  const bibleFields = byId("bible-fields");
  const counter = byId("media-char-count");
  const error = byId("scripture-error");
  if (!kind || !scripture || !book || !bookSearch || !bookList || !from || !to || !choice || !used || !text || !linkGroup || !scriptureGroup || !bibleFields) return;

  // Book: a search box over the list of books (a combobox). Typing filters
  // the list; a click, or the arrow keys and Enter, chooses a book, whose id
  // goes in the hidden verse_book field.
  const items = /** @type {HTMLLIElement[]} */ (Array.from(bookList.querySelectorAll("li[data-id]")));
  /** @type {HTMLLIElement | null} */
  let active = null;

  /** @param {string} id */
  function bookItem(id) {
    return items.find((li) => li.dataset.id === id) || null;
  }

  /** @param {string} s */
  function fold(s) {
    return s.toLowerCase().replace(/\s+/g, " ").trim();
  }

  /**
   * Whether a book matches a search: its name or any word of it starts with
   * what was typed (spaces optional: "1co", "song"), or its id does ("jhn").
   * @param {HTMLLIElement} li
   * @param {string} q folded search
   */
  function matches(li, q) {
    if (!q) return true;
    const name = fold(li.textContent || "");
    const tight = q.replace(/ /g, "");
    return name.replace(/ /g, "").startsWith(tight) || name.split(" ").some((w) => w.startsWith(q)) ||
      (li.dataset.id || "").toLowerCase().startsWith(tight);
  }

  /** @param {HTMLLIElement | null} li */
  function setActive(li) {
    if (active) active.classList.remove("is-active");
    active = li;
    if (li) {
      li.classList.add("is-active");
      li.scrollIntoView({ block: "nearest" });
      bookSearch?.setAttribute("aria-activedescendant", li.id);
    } else {
      bookSearch?.removeAttribute("aria-activedescendant");
    }
  }

  /** @param {boolean} open */
  function setOpen(open) {
    if (bookList) bookList.hidden = !open;
    bookSearch?.setAttribute("aria-expanded", String(open));
    if (!open) setActive(null);
  }

  /**
   * Shows the books matching the search. Coming back to a chosen book
   * (browse) shows them all, starting at it, to pick another.
   * @param {boolean} browse
   */
  function filterBooks(browse) {
    const chosen = book?.value ? bookItem(book.value) : null;
    const q = fold(bookSearch?.value || "");
    const showAll = browse && !!chosen && fold(chosen.textContent || "") === q;
    let first = null;
    for (const li of items) {
      li.hidden = !showAll && !matches(li, q);
      li.setAttribute("aria-selected", String(li === chosen));
      if (!li.hidden && !first) first = li;
    }
    setActive(showAll ? chosen : first);
  }

  /** @param {HTMLLIElement} li */
  function chooseBook(li) {
    if (book) book.value = li.dataset.id || "";
    if (bookSearch) bookSearch.value = li.textContent || "";
    setOpen(false);
    changed();
  }

  bookSearch.addEventListener("input", () => {
    // Typing a book's whole name chooses it; anything else unchooses.
    const exact = items.find((li) => fold(li.textContent || "") === fold(bookSearch.value));
    book.value = exact ? exact.dataset.id || "" : "";
    filterBooks(false);
    setOpen(true);
    changed();
  });
  bookSearch.addEventListener("focus", () => {
    filterBooks(true);
    setOpen(true);
  });
  // Leaving the box closes the list, unless focus moved into the list (a
  // click or tap on a book, which chooses it and comes back).
  bookSearch.addEventListener("blur", (event) => {
    if (event.relatedTarget instanceof Node && bookList.contains(event.relatedTarget)) return;
    setOpen(false);
    changed(); // the field's error may show now
  });
  bookSearch.addEventListener("keydown", (event) => {
    const visible = items.filter((li) => !li.hidden);
    const at = active ? visible.indexOf(active) : -1;
    if (event.key === "ArrowDown" || event.key === "ArrowUp") {
      event.preventDefault();
      if (bookList.hidden) {
        filterBooks(true);
        setOpen(true);
        return;
      }
      const step = event.key === "ArrowDown" ? 1 : -1;
      setActive(visible[Math.min(Math.max(at + step, 0), visible.length - 1)] || null);
    } else if (event.key === "Enter" && !bookList.hidden && active) {
      event.preventDefault(); // choose the book, don't submit the form
      chooseBook(active);
    } else if (event.key === "Escape" && !bookList.hidden) {
      event.preventDefault();
      setOpen(false);
    }
  });
  bookList.addEventListener("click", (event) => {
    const li = event.target instanceof Element ? event.target.closest("li[data-id]") : null;
    if (!(li instanceof HTMLLIElement)) return;
    chooseBook(li);
    bookSearch.focus({ preventScroll: true });
    setOpen(false);
  });
  // Focus that leaves the list for anywhere but the search box closes it.
  bookList.addEventListener("focusout", (event) => {
    const to = event.relatedTarget;
    if (to instanceof Node && (bookList.contains(to) || to === bookSearch)) return;
    setOpen(false);
    changed();
  });

  /** @param {string} s */
  function normalize(s) {
    return s.replace(SPACES, " ").trim().replace(SPACE_BEFORE_CLOSE, "$1");
  }

  /**
   * internal/verse Assemble: the text of verses first..last of a chapter file.
   * @param {unknown} file
   * @param {number} first
   * @param {number} last
   * @returns {string | null}
   */
  function assemble(file, first, last) {
    const content = /** @type {{ chapter?: { content?: unknown } }} */ (file)?.chapter?.content;
    if (!Array.isArray(content)) return null;
    /** @type {string[]} */
    const parts = [];
    let haveFirst = false;
    let haveLast = false;
    for (const block of content) {
      if (!block || block.type !== "verse" || typeof block.number !== "number" || block.number < first || block.number > last) continue;
      haveFirst = haveFirst || block.number === first;
      haveLast = haveLast || block.number === last;
      for (const part of Array.isArray(block.content) ? block.content : []) {
        if (typeof part === "string") parts.push(part);
        else if (part && typeof part.text === "string") parts.push(part.text);
      }
    }
    const joined = normalize(parts.join(" "));
    return haveFirst && haveLast && joined ? joined : null;
  }

  /**
   * internal/verse ParseChoice, without the translation: the chosen verses,
   * or a message saying what to fix.
   * @returns {{ book: string, name: string, chapter: number, first: number, last: number } | { problem: string, quiet?: boolean }}
   *   quiet: a required field is still empty, which is not yet a mistake
   */
  function readChoice() {
    const option = book?.value ? bookItem(book.value) : null;
    if (!book?.value || !option) {
      const typed = (bookSearch?.value || "").trim() !== "";
      return { problem: typed ? "Choose a book from the list." : "Choose a book.", quiet: !typed };
    }
    const chapters = Number(option.dataset.chapters);
    const f = /^\s*(\d{1,3}):(\d{1,3})\s*$/.exec(from?.value || "");
    if (!f) return { problem: "From is chapter:verse, like 12:3.", quiet: !(from?.value || "").trim() };
    const chapter = Number(f[1]);
    const first = Number(f[2]);
    const name = option.textContent || "";
    if (chapter < 1 || chapter > chapters) return { problem: `${name} has ${chapters} chapters.` };
    let last = first;
    const rawTo = (to?.value || "").trim();
    if (rawTo) {
      const t = /^(?:(\d{1,3}):)?(\d{1,3})$/.exec(rawTo);
      if (!t) return { problem: "To is a verse, like 4, or chapter:verse in the same chapter." };
      if (t[1] !== undefined && Number(t[1]) !== chapter) return { problem: "From and To must be in the same chapter." };
      last = Number(t[2]);
    }
    if (first < 1 || last < first || last > 176) return { problem: "The verses must run forwards." };
    if (last - first + 1 > MAX_VERSES) return { problem: `Choose at most ${MAX_VERSES} verses.` };
    return { book: book.value, name, chapter, first, last };
  }

  /** The translations to try, in order: one, or the preferred ones for "auto". */
  function candidates() {
    const option = choice?.selectedOptions[0];
    if (!choice || !option) return [];
    if (choice.value === "auto") return (option.dataset.order || "").split(" ").filter(Boolean);
    return [choice.value];
  }

  /** @param {string} id */
  function labelOf(id) {
    const option = choice ? Array.from(choice.options).find((o) => o.value === id) : undefined;
    return option?.dataset.label || id;
  }

  /**
   * @param {string} tag
   * @param {string} [className]
   * @param {string} [content]
   */
  function el(tag, className, content) {
    const node = document.createElement(tag);
    if (className) node.className = className;
    if (content !== undefined) node.textContent = content;
    return node;
  }

  /** @param {HTMLElement} container @param {string} message @param {boolean} [loading] */
  function status(container, message, loading) {
    const box = el("div", "passage-status");
    if (loading && window.ResImperiiDots) {
      box.setAttribute("role", "status");
      box.append(window.ResImperiiDots.create("Loading verses", "twinkle"));
    } else {
      box.textContent = message;
    }
    container.classList.remove("is-empty");
    container.replaceChildren(box);
  }

  /**
   * @typedef {object} State
   * @property {string} key what was asked for
   * @property {"loading" | "ready" | "failed"} phase
   * @property {string} [message]
   * @property {{ reference: string, version: string, text: string }} [passage]
   */
  /** @type {State | null} */
  let state = null;
  /** @type {number | undefined} */
  let timer;

  function changed() {
    syncGroups();
    document.dispatchEvent(new CustomEvent("scripture:change"));
  }

  /** Shows the groups the dropdowns call for. */
  function syncGroups() {
    const isScripture = kind?.value === "scripture";
    if (linkGroup) linkGroup.hidden = isScripture;
    if (scriptureGroup) scriptureGroup.hidden = !isScripture;
    if (counter) counter.hidden = isScripture;
    if (bibleFields) bibleFields.hidden = scripture?.value !== "bible";
  }

  /**
   * Fetches the chapter in each candidate translation until one has the
   * verses.
   * @param {State} asked
   * @param {{ book: string, name: string, chapter: number, first: number, last: number }} c
   * @param {string[]} order
   */
  async function load(asked, c, order) {
    let message = "";
    for (const id of order) {
      let file = null;
      try {
        const res = await fetch(`${API}/${id}/${c.book}/${c.chapter}.json`, { headers: { Accept: "application/json" }, credentials: "omit" });
        if (res.ok) file = await res.json();
        else if (res.status !== 404) message = `The Bible provider answered ${res.status}.`;
      } catch {
        message = "Couldn't reach the Bible provider.";
      }
      if (state !== asked) return; // the choice changed meanwhile
      const passage = file && assemble(file, c.first, c.last);
      if (passage) {
        const reference = `${c.name} ${c.chapter}:${c.first}${c.last > c.first ? "-" + c.last : ""}`;
        asked.phase = "ready";
        asked.passage = { reference, version: labelOf(id), text: passage };
        if (used) used.value = id;
        if (text) text.value = passage;
        changed();
        return;
      }
    }
    if (state !== asked) return;
    asked.phase = "failed";
    asked.message = message || `${c.name} ${c.chapter}:${c.first}${c.last > c.first ? "-" + c.last : ""} isn't in ${order.map(labelOf).join(" or ")}.`;
    changed();
  }

  /**
   * Shows the scripture preview in container and reports whether the passage
   * is ready to save. Called on every change; it fetches only when the
   * choice changes.
   * @param {HTMLElement} container
   */
  function update(container) {
    syncGroups();
    if (scripture?.value !== "bible") {
      state = null;
      if (error) error.textContent = "";
      status(container, "Choose a scripture.");
      return false;
    }
    const c = readChoice();
    if ("problem" in c) {
      state = null;
      window.clearTimeout(timer);
      if (used) used.value = "";
      if (text) text.value = "";
      // Before anything is typed, the fields themselves say what is needed.
      const untouched = !(bookSearch?.value || "").trim() && !(from?.value || "").trim();
      // Only a mistake is shown in red: not an empty required field, and not
      // a book search still being typed.
      const searching = document.activeElement === bookSearch && !book?.value;
      if (error) error.textContent = c.quiet || searching ? "" : c.problem;
      status(container, untouched ? "Choose a book and the verses." : c.problem);
      return false;
    }
    if (error) error.textContent = "";
    const order = candidates();
    const key = `${order.join(",")}|${c.book}|${c.chapter}|${c.first}|${c.last}`;
    if (!state || state.key !== key) {
      const asked = /** @type {State} */ ({ key, phase: "loading" });
      state = asked;
      if (used) used.value = "";
      if (text) text.value = "";
      window.clearTimeout(timer);
      timer = window.setTimeout(() => void load(asked, c, order), FETCH_DELAY_MS);
    }
    if (state.phase === "loading") {
      status(container, "", true);
      return false;
    }
    if (state.phase === "failed" || !state.passage) {
      if (error) error.textContent = state.message || "";
      status(container, state.message || "Couldn't load the verses.");
      return false;
    }
    if (!container.querySelector(".passage-box") || container.dataset.passageKey !== key) {
      const p = state.passage;
      const box = el("figure", "tweet-box passage-box");
      const head = el("div", "tweet-head");
      head.append(el("span", "tweet-site", "Bible"), el("span", "passage-version", p.version));
      const quote = el("blockquote", "tweet-text passage-text");
      quote.append(el("p", "", p.text));
      const byline = el("figcaption", "tweet-byline passage-byline");
      byline.append(el("span", "passage-reference", p.reference));
      box.append(head, quote, byline);
      container.classList.remove("is-empty");
      container.dataset.passageKey = key;
      container.replaceChildren(box);
    }
    return true;
  }

  for (const field of [kind, scripture, choice]) field.addEventListener("change", changed);
  for (const field of [from, to]) field.addEventListener("input", changed);
  syncGroups();

  window.ResImperiiScripture = {
    /** Whether the media is scripture rather than a link. */
    active: () => kind.value === "scripture",
    update,
  };
})();
