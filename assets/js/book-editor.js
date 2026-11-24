/*
 * Live preview and length check for the Book I/II/III editor.
 * Uses locally vendored markdown-it (html: false) and DOMPurify (strict CSP).
 *
 * The limit is in bytes (the textarea's data-max-bytes, from
 * db.MaxBookBodyBytes). Nothing is shown until the text goes past it; then an
 * error says by how much, and Save is disabled so the server never has to
 * refuse (or trim) the book.
 */
(() => {
  "use strict";

  const textarea = /** @type {HTMLTextAreaElement | null} */ (document.getElementById("book-md-input"));
  const preview = document.getElementById("book-md-preview");
  const error = document.getElementById("book-length-error");
  if (!textarea || !preview) return;

  const maxBytes = Number(textarea.dataset.maxBytes) || 0;
  const save = textarea.form ? textarea.form.querySelector("button[type='submit']") : null;
  // Checked copies for the function declarations below: TypeScript does not
  // carry a null check into a hoisted function.
  const input = textarea;
  const out = preview;
  const encoder = new TextEncoder();
  const md = typeof window.markdownit === "function"
    ? window.markdownit({ html: false, linkify: true, typographer: false })
    : null;

  // Rounded up to 0.1 KB, so text just past the limit never reads as the
  // limit itself ("128.1 KB", not "128.0 KB").
  /** @param {number} bytes */
  function formatKB(bytes) {
    const kb = Math.ceil((bytes / 1024) * 10) / 10;
    return (Number.isInteger(kb) ? kb : kb.toFixed(1)) + " KB";
  }

  /** @param {string} src */
  function checkLength(src) {
    if (!maxBytes || !error) return;
    const bytes = encoder.encode(src).length;
    const over = bytes > maxBytes;
    if (over) {
      error.textContent = `Too long: ${formatKB(bytes)}. The limit is ${formatKB(maxBytes)}.`;
    }
    error.hidden = !over;
    if (save instanceof HTMLButtonElement) {
      save.disabled = over;
      save.setAttribute("aria-disabled", over ? "true" : "false");
    }
  }

  function render() {
    const src = input.value || "";
    checkLength(src);
    if (!md) return;
    const rawHTML = md.render(src);
    const safeHTML = window.DOMPurify
      ? window.DOMPurify.sanitize(rawHTML, {
          ALLOWED_URI_REGEXP: /^(?:https?:|\/|#)/i,
        })
      : rawHTML;
    out.innerHTML = safeHTML;
  }

  textarea.addEventListener("input", render);
  checkLength(textarea.value || "");
})();
