/*
 * Offline support, the small version:
 *  - registers the service worker (/sw.js), which shows "You're offline" when
 *    a page can't be reached;
 *  - when an HTMX request can't reach the server (the journal catalog loads
 *    that way), its loader is replaced with the same advice.
 */
(() => {
  if ("serviceWorker" in navigator) {
    window.addEventListener("load", () => {
      navigator.serviceWorker.register("/sw.js").catch(() => {});
    });
  }

  document.addEventListener("htmx:sendError", (event) => {
    // HTMX's CustomEvent: detail.target is the element whose request failed.
    const detail = /** @type {CustomEvent<{ target?: Element }>} */ (event).detail;
    const target = detail && detail.target;
    if (!target) return;
    const loader = target.querySelector(".page-dot-loading");
    if (!loader) return;
    loader.textContent = navigator.onLine === false
      ? "You're offline. Reconnect, then reload the page."
      : "Couldn't reach the server. Reload the page to try again.";
  });
})();
