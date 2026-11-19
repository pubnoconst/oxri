/*
 * Journal editor preview, Title Case tag auto-capitalization, autocomplete,
 * and schema constraint validation.
 * Reuses window.ResImperiiMedia for shared media preview rendering.
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
  const submitBtn = document.getElementById("publish-submit-btn");
  const form = document.getElementById("journal-editor-form");

  if (!contentInput || !mediaInput || !tagsInput || !renderedContent || !mediaPreview || !tagsPreview || !preview) return;
  if (!window.markdownit || !window.DOMPurify || !window.ResImperiiMedia) {
    preview.textContent = "The live preview libraries did not load. Check your connection and reload.";
    return;
  }

  const MAX_MEDIA_LEN = 128;
  const MAX_TAG_COUNT = 3;
  const MAX_TAG_LEN = 4;

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

  // Capitalize each comma-separated token in place while preserving leading/trailing
  // spaces so typing commas and spaces feels completely natural.
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

  function renderSuggestions(chips) {
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
    // Auto-capitalize while preserving cursor position
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
    const duplicateSet = new Set();
    let firstDuplicate = "";
    chips.forEach((t) => {
      if (seen.has(t)) {
        duplicateSet.add(t);
        if (!firstDuplicate) firstDuplicate = t;
      }
      seen.add(t);
    });

    let errText = "";
    if (chips.length > MAX_TAG_COUNT) {
      errText = `Maximum ${MAX_TAG_COUNT} tags allowed (currently ${chips.length}).`;
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

    const seenInRender = new Set();
    tagsPreview.replaceChildren(...chips.map((tag) => {
      const chip = document.createElement("span");
      const over = Array.from(tag).length > MAX_TAG_LEN;
      const isDup = seenInRender.has(tag);
      seenInRender.add(tag);
      chip.className = (over || isDup) ? "journal-tag-chip is-invalid" : "journal-tag-chip";
      chip.textContent = tag;
      return chip;
    }));

    renderSuggestions(chips);
    syncSubmitButton();
  }

  function updateContent() {
    const raw = contentInput.value;
    isBodyValid = raw.trim().length > 0;
    if (bodyError) {
      bodyError.textContent = isBodyValid ? "" : "Required: journal entry cannot be empty.";
    }
    renderedContent.innerHTML = window.DOMPurify.sanitize(markdown.render(raw));
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
