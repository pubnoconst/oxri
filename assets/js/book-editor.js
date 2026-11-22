/*
 * Live Markdown preview & byte counter for the Book I/II/III editor.
 * Uses locally vendored markdown-it (html: false) and DOMPurify (strict CSP).
 */
(() => {
  "use strict";

  const textarea = document.getElementById("book-md-input");
  const preview = document.getElementById("book-md-preview");
  const counter = document.getElementById("book-byte-count");
  if (!textarea || !preview) return;

  const MAX_BYTES = 128 * 1024;
  const encoder = new TextEncoder();
  const md = typeof window.markdownit === "function"
    ? window.markdownit({ html: false, linkify: true, typographer: false })
    : null;

  function render() {
    const src = textarea.value || "";
    if (counter) {
      const byteLen = encoder.encode(src).length;
      counter.textContent = `${byteLen} / ${MAX_BYTES} bytes`;
    }
    if (!md) return;
    const rawHTML = md.render(src);
    const safeHTML = window.DOMPurify
      ? window.DOMPurify.sanitize(rawHTML, {
          ALLOWED_URI_REGEXP: /^(?:https?:|\/|#)/i,
        })
      : rawHTML;
    preview.innerHTML = safeHTML;
  }

  textarea.addEventListener("input", render);
})();
