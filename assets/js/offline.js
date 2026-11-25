/*
 * Offline support, the small version:
 *  - registers the service worker (/sw.js), which shows "You're offline" when
 *    a page can't be reached. (A catalog search that can't reach the server
 *    falls back to loading the page, so it lands here too:
 *    navigation-feedback.js.)
 */
(() => {
  if ("serviceWorker" in navigator) {
    window.addEventListener("load", () => {
      navigator.serviceWorker.register("/sw.js").catch(() => {});
    });
  }
})();
