/*
 * Journal editor preview and schema constraint validation.
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

  let isMediaValid = false;
  let isTagsValid = true;
  let isBodyValid = false;

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

  function updateTags() {
    const chips = tagsInput.value
      .split(",")
      .map((tag) => tag.trim())
      .filter(Boolean);

    if (tagsCount) {
      tagsCount.textContent = `${chips.length} / ${MAX_TAG_COUNT}`;
      tagsCount.classList.toggle("is-invalid", chips.length > MAX_TAG_COUNT);
    }

    let errText = "";
    if (chips.length > MAX_TAG_COUNT) {
      errText = `Maximum ${MAX_TAG_COUNT} tags allowed (currently ${chips.length}).`;
    } else {
      const tooLong = chips.find((t) => Array.from(t).length > MAX_TAG_LEN);
      if (tooLong) {
        errText = `Tag "${tooLong}" exceeds ${MAX_TAG_LEN} characters.`;
      }
    }

    isTagsValid = errText === "";
    if (tagsError) tagsError.textContent = errText;

    tagsPreview.replaceChildren(...chips.map((tag) => {
      const chip = document.createElement("span");
      const over = Array.from(tag).length > MAX_TAG_LEN;
      chip.className = over ? "journal-tag-chip is-invalid" : "journal-tag-chip";
      chip.textContent = tag;
      return chip;
    }));
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
