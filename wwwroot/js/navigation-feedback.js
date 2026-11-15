/* Acknowledge same-origin link navigation if the destination takes a moment. */
(() => {
  const pendingClass = "navigation-pending";
  const delayMs = 180;
  // The app's endpoint timeout is 30 seconds; release the dim after that plus a buffer.
  const resetAfterMs = 35000;
  let pendingTimer;
  let resetTimer;

  function clearPending() {
    window.clearTimeout(pendingTimer);
    window.clearTimeout(resetTimer);
    document.documentElement.classList.remove(pendingClass);
  }

  document.addEventListener("click", (event) => {
    if (!(event instanceof MouseEvent) || event.defaultPrevented) return;
    if (event.button !== 0 || event.metaKey || event.ctrlKey || event.shiftKey || event.altKey) return;

    const link = event.target instanceof Element ? event.target.closest("a[href]") : null;
    if (!link || link.hasAttribute("download")) return;
    if (link.target && link.target.toLowerCase() !== "_self") return;
    if (link.relList.contains("external")) return;

    let destination;
    try {
      destination = new URL(link.href, window.location.href);
    } catch {
      return;
    }

    if (destination.origin !== window.location.origin) return;
    if (destination.pathname === window.location.pathname
      && destination.search === window.location.search
      && destination.hash) return;

    clearPending();
    pendingTimer = window.setTimeout(() => {
      if (event.defaultPrevented) return;
      document.documentElement.classList.add(pendingClass);
      resetTimer = window.setTimeout(clearPending, resetAfterMs);
    }, delayMs);
  });

  // A Back/Forward Cache restore may bring this document back with its class.
  window.addEventListener("pageshow", clearPending);
})();
