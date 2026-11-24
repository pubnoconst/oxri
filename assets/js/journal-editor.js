/*
 * ALL INPUTS ARE EVIL.
 * Journal editor preview, Title Case tag auto-capitalization, autocomplete,
 * strict character-set whitelisting (English alphanumeric, Latin, Greek; no emojis),
 * and schema constraint validation.
 * Disallowed characters are NEVER rendered in the preview pane or echoed in error messages.
 */
(() => {
  const contentInput = document.getElementById("journal-content");
  const mediaInput = document.getElementById("journal-media-link");
  const tagsInput = document.getElementById("journal-tags");
  const renderedContent = document.getElementById("journal-rendered-content");
  const mediaPreview = document.getElementById("journal-preview-media");
  const tagsPreview = document.getElementById("journal-preview-tags");
  const preview = document.getElementById("journal-preview");

  const mediaCount = document.getElementById("media-char-count");
  const mediaError = document.getElementById("media-error");
  const tagsCount = document.getElementById("tags-count");
  const tagsError = document.getElementById("tags-error");
  const tagSuggestions = document.getElementById("tag-suggestions");
  const bodyError = document.getElementById("body-error");
  const bodyCount = document.getElementById("body-char-count");
  // The body limit comes from the textarea's maxlength (set from the Go domain
  // constant), so it is defined in exactly one place.
  const MAX_BODY_LEN = contentInput && contentInput.maxLength > 0 ? contentInput.maxLength : Infinity;
  const submitBtn = document.getElementById("publish-submit-btn");
  const form = document.getElementById("journal-editor-form");

  if (!contentInput || !mediaInput || !tagsInput || !renderedContent || !mediaPreview || !tagsPreview || !preview) return;
  if (!window.markdownit || !window.DOMPurify || !window.ResImperiiMedia) {
    preview.textContent = "The live preview libraries did not load. Check your connection and reload.";
    return;
  }

  const MAX_MEDIA_LEN = 128;
  const MAX_TAG_COUNT = 3;
  const MAX_TAG_LEN = 8;

  // Strict ASCII-only tags and Latin/Greek prose matching server-side internal/domain/journal.go
  const TAG_CHAR_RE = /^[A-Za-z0-9]$/;
  const PROSE_LETTER_RE = /^[\p{Script=Latin}\p{Script=Greek}\u0300-\u036F]$/u;
  const ALLOWED_TYPO_SET = new Set([
    0x00A1, 0x00A7, 0x00AB, 0x00B0, 0x00B7, 0x00BB, 0x00BF,
    0x2013, 0x2014, 0x2018, 0x2019, 0x201C, 0x201D, 0x2022, 0x2026,
    0x037E, 0x0384, 0x0385, 0x0387, 0x1FFD, 0x1FFE,
  ]);

  function formatCodePoint(ch) {
    const cp = ch.codePointAt(0) || 0;
    return "U+" + cp.toString(16).toUpperCase().padStart(4, "0");
  }

  function findInvalidMediaChar(str) {
    for (const ch of Array.from(str)) {
      const cp = ch.codePointAt(0);
      if (cp < 0x21 || cp > 0x7E) return ch;
    }
    return null;
  }

  function findInvalidTagChar(str) {
    for (const ch of Array.from(str)) {
      if (!TAG_CHAR_RE.test(ch)) return ch;
    }
    return null;
  }

  function isAllowedProseChar(ch) {
    if (ch === " " || ch === "\n" || ch === "\r" || ch === "\t") return true;
    const cp = ch.codePointAt(0);
    if (cp >= 0x21 && cp <= 0x7E) return true;
    if (PROSE_LETTER_RE.test(ch)) return true;
    if (ALLOWED_TYPO_SET.has(cp)) return true;
    if ((cp >= 0x1FBD && cp <= 0x1FC1) || (cp >= 0x1FCD && cp <= 0x1FCF) ||
        (cp >= 0x1FDD && cp <= 0x1FDF) || (cp >= 0x1FED && cp <= 0x1FEF)) {
      return true;
    }
    return false;
  }

  function findInvalidProseChar(str) {
    for (const ch of Array.from(str)) {
      if (!isAllowedProseChar(ch)) return ch;
    }
    return null;
  }

  const { safeUrl, createMedia } = window.ResImperiiMedia;
  const markdown = window.markdownit({ html: false, linkify: false, breaks: false });

  const existingTags = (tagsInput.getAttribute("data-existing-tags") || "")
    .split(",")
    .map((t) => toTitleCase(t.trim()))
    .filter(Boolean);

  let isMediaValid = false;
  let isTagsValid = true;
  let isBodyValid = false;

  function toTitleCase(str) {
    const trimmed = str.trim();
    if (!trimmed) return "";
    const chars = Array.from(trimmed);
    return chars[0].toUpperCase() + chars.slice(1).join("").toLowerCase();
  }

  function autoCapitalizeInputValue(rawVal) {
    return rawVal
      .split(",")
      .map((seg) => {
        const match = seg.match(/^(\s*)(\S+)([\s\S]*)$/);
        if (!match) return seg;
        const [, lead, word, tail] = match;
        const chars = Array.from(word);
        const titled = chars[0].toUpperCase() + chars.slice(1).join("").toLowerCase();
        return lead + titled + tail;
      })
      .join(",");
  }

  function syncSubmitButton() {
    if (!submitBtn) return;
    const isAuth = submitBtn.getAttribute("data-is-auth") === "true";
    const allValid = isMediaValid && isTagsValid && isBodyValid;
    submitBtn.disabled = !(isAuth && allValid);
    submitBtn.setAttribute("aria-disabled", submitBtn.disabled ? "true" : "false");
  }

  function updateMedia() {
    mediaPreview.replaceChildren();
    const raw = mediaInput.value.trim();
    const len = Array.from(raw).length;

    if (mediaCount) {
      mediaCount.textContent = `${len} / ${MAX_MEDIA_LEN}`;
      mediaCount.classList.toggle("is-invalid", len > MAX_MEDIA_LEN);
    }

    if (!raw) {
      mediaPreview.textContent = "Your media will appear here";
      mediaPreview.classList.add("is-empty");
      isMediaValid = false;
      if (mediaError) mediaError.textContent = "Required: enter a media URL (1–128 characters).";
      syncSubmitButton();
      return;
    }

    if (len > MAX_MEDIA_LEN) {
      mediaPreview.classList.add("is-empty");
      mediaPreview.textContent = "Media link exceeds 128 characters.";
      isMediaValid = false;
      if (mediaError) mediaError.textContent = `Exceeds maximum length of ${MAX_MEDIA_LEN} characters (${len}).`;
      syncSubmitButton();
      return;
    }

    const badChar = findInvalidMediaChar(raw);
    if (badChar !== null) {
      mediaPreview.classList.add("is-empty");
      mediaPreview.textContent = "Media link contains disallowed characters.";
      isMediaValid = false;
      if (mediaError) mediaError.textContent = `Disallowed character (${formatCodePoint(badChar)}): only printable ASCII URL characters are permitted (no emojis).`;
      syncSubmitButton();
      return;
    }

    const url = safeUrl(raw);
    if (!url) {
      mediaPreview.classList.add("is-empty");
      mediaPreview.textContent = "Enter a valid http or https link.";
      isMediaValid = false;
      if (mediaError) mediaError.textContent = "Must be a valid http:// or https:// URL.";
      syncSubmitButton();
      return;
    }

    isMediaValid = true;
    if (mediaError) mediaError.textContent = "";

    const media = createMedia(url);
    if (media) {
      mediaPreview.classList.remove("is-empty");
      mediaPreview.append(media);
    } else {
      mediaPreview.classList.add("is-empty");
      mediaPreview.textContent = "Link accepted (no inline preview for this URL type).";
    }
    syncSubmitButton();
  }

  function applySuggestion(selectedTag) {
    const segments = tagsInput.value.split(",");
    segments[segments.length - 1] = (segments.length > 1 ? " " : "") + selectedTag;
    const filledCount = segments.map((s) => s.trim()).filter(Boolean).length;
    tagsInput.value = filledCount < MAX_TAG_COUNT
      ? segments.join(",") + ", "
      : segments.join(",");
    tagsInput.focus();
    updateTags();
  }

  function renderSuggestions() {
    if (!tagSuggestions) return;
    tagSuggestions.replaceChildren();
    if (existingTags.length === 0) return;

    const segments = tagsInput.value.split(",");
    const activeToken = toTitleCase(segments[segments.length - 1] || "");
    const committedSet = new Set(
      segments
        .slice(0, -1)
        .map((s) => toTitleCase(s))
        .filter(Boolean)
    );

    if (committedSet.size >= MAX_TAG_COUNT) return;

    const matches = existingTags.filter((tag) => {
      if (committedSet.has(tag)) return false;
      if (!activeToken) return true;
      return tag.startsWith(activeToken) && tag !== activeToken;
    });

    if (matches.length === 0) return;

    const label = document.createElement("span");
    label.className = "tag-suggestion-label";
    label.textContent = "Suggestions:";
    tagSuggestions.append(label);

    matches.slice(0, 8).forEach((tag) => {
      const btn = document.createElement("button");
      btn.type = "button";
      btn.className = "tag-suggestion-pill";
      btn.setAttribute("role", "option");
      btn.textContent = tag;
      btn.addEventListener("click", () => applySuggestion(tag));
      tagSuggestions.append(btn);
    });
  }

  function updateTags() {
    const start = tagsInput.selectionStart;
    const end = tagsInput.selectionEnd;
    const capitalizedVal = autoCapitalizeInputValue(tagsInput.value);
    if (tagsInput.value !== capitalizedVal) {
      tagsInput.value = capitalizedVal;
      if (typeof start === "number" && typeof end === "number") {
        tagsInput.setSelectionRange(start, end);
      }
    }

    const chips = tagsInput.value
      .split(",")
      .map((tag) => toTitleCase(tag))
      .filter(Boolean);

    if (tagsCount) {
      tagsCount.textContent = `${chips.length} / ${MAX_TAG_COUNT}`;
      tagsCount.classList.toggle("is-invalid", chips.length > MAX_TAG_COUNT);
    }

    const seen = new Set();
    let firstDuplicate = "";
    let badTagChar = null;
    chips.forEach((t) => {
      if (badTagChar === null) {
        const badCh = findInvalidTagChar(t);
        if (badCh !== null) {
          badTagChar = badCh;
        }
      }
      if (seen.has(t)) {
        if (!firstDuplicate) firstDuplicate = t;
      }
      seen.add(t);
    });

    let errText = "";
    if (chips.length > MAX_TAG_COUNT) {
      errText = `Maximum ${MAX_TAG_COUNT} tags allowed (currently ${chips.length}).`;
    } else if (badTagChar !== null) {
      errText = `Tag contains disallowed character (${formatCodePoint(badTagChar)}): only ASCII letters and digits permitted.`;
    } else {
      const tooLong = chips.find((t) => Array.from(t).length > MAX_TAG_LEN);
      if (tooLong) {
        errText = `Tag "${tooLong}" exceeds ${MAX_TAG_LEN} characters.`;
      } else if (firstDuplicate) {
        errText = `Duplicate tag "${firstDuplicate}" (tags must be unique).`;
      }
    }

    isTagsValid = errText === "";
    if (tagsError) tagsError.textContent = errText;

    // Never preview illegal characters in tag chips
    const seenInRender = new Set();
    const previewChipNodes = [];
    chips.forEach((tag) => {
      const cleanTag = toTitleCase(Array.from(tag).filter((ch) => TAG_CHAR_RE.test(ch)).join(""));
      if (!cleanTag) return;
      const chip = document.createElement("span");
      const over = Array.from(cleanTag).length > MAX_TAG_LEN;
      const isDup = seenInRender.has(cleanTag);
      seenInRender.add(cleanTag);
      chip.className = (over || isDup) ? "journal-tag-chip is-invalid" : "journal-tag-chip";
      chip.textContent = cleanTag;
      previewChipNodes.push(chip);
    });
    tagsPreview.replaceChildren(...previewChipNodes);

    renderSuggestions();
    syncSubmitButton();
  }

  function updateContent() {
    const raw = contentInput.value;
    const bodyLen = Array.from(raw).length; // characters (code points), as the server counts
    if (bodyCount && Number.isFinite(MAX_BODY_LEN)) {
      bodyCount.textContent = `${bodyLen} / ${MAX_BODY_LEN}`;
      bodyCount.classList.toggle("is-invalid", bodyLen > MAX_BODY_LEN);
    }
    if (bodyLen > MAX_BODY_LEN) {
      isBodyValid = false;
      if (bodyError) bodyError.textContent = `Too long: ${bodyLen} characters. The limit is ${MAX_BODY_LEN}.`;
    } else if (raw.trim().length === 0) {
      isBodyValid = false;
      if (bodyError) bodyError.textContent = "Required: journal entry cannot be empty.";
    } else {
      const badChar = findInvalidProseChar(raw);
      if (badChar !== null) {
        isBodyValid = false;
        if (bodyError) {
          bodyError.textContent = `Disallowed character (${formatCodePoint(badChar)}): only English, Latin, Greek, numbers, and standard punctuation are permitted (no emojis).`;
        }
      } else {
        isBodyValid = true;
        if (bodyError) bodyError.textContent = "";
      }
    }
    // Strip any disallowed characters before rendering into the Preview pane
    const cleanForPreview = Array.from(raw).filter(isAllowedProseChar).join("");
    renderedContent.innerHTML = window.DOMPurify.sanitize(markdown.render(cleanForPreview));
    syncSubmitButton();
  }

  tagsInput.addEventListener("keydown", (e) => {
    if (e.key === "Tab" && tagSuggestions) {
      const segments = tagsInput.value.split(",");
      const activeToken = (segments[segments.length - 1] || "").trim();
      const firstPill = tagSuggestions.querySelector(".tag-suggestion-pill");
      if (activeToken && firstPill) {
        e.preventDefault();
        applySuggestion(firstPill.textContent);
      }
    }
  });

  contentInput.addEventListener("input", updateContent);
  mediaInput.addEventListener("input", updateMedia);
  tagsInput.addEventListener("input", updateTags);

  if (form) {
    form.addEventListener("submit", (e) => {
      updateMedia();
      updateTags();
      updateContent();
      if (!(isMediaValid && isTagsValid && isBodyValid)) {
        e.preventDefault();
      }
    });
  }

  updateContent();
  updateMedia();
  updateTags();

  const themeObserver = new MutationObserver(() => {
    if (mediaInput.value.trim() && mediaPreview.querySelector(".tweet-embed-container")) {
      updateMedia();
    }
  });
  themeObserver.observe(document.documentElement, { attributes: true, attributeFilter: ["data-theme"] });
})();
