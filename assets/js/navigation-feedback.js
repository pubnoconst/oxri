/* Smooth same-origin page transition ease-out and pending navigation feedback. */
(() => {
  const pendingClass = "navigation-pending";
  const leavingClass = "navigation-leaving";
  const delayMs = 180;
  const resetAfterMs = 35000;
  let pendingTimer;
  let resetTimer;

  function clearNavigationState() {
    window.clearTimeout(pendingTimer);
    window.clearTimeout(resetTimer);
    document.documentElement.classList.remove(pendingClass, leavingClass);
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
    if (
      destination.pathname === window.location.pathname &&
      destination.search === window.location.search &&
      destination.hash
    ) {
      return;
    }

    clearNavigationState();
    document.documentElement.classList.add(leavingClass);
    pendingTimer = window.setTimeout(() => {
      if (event.defaultPrevented) return;
      document.documentElement.classList.add(pendingClass);
      resetTimer = window.setTimeout(clearNavigationState, resetAfterMs);
    }, delayMs);
  });

  // Handle declarative confirmation prompts without inline onsubmit attributes (strict CSP).
  document.addEventListener("submit", (event) => {
    const form = event.target instanceof HTMLFormElement ? event.target : null;
    if (!form) return;
    const msg = form.getAttribute("data-confirm");
    if (msg && !window.confirm(msg)) {
      event.preventDefault();
    }
  });

  // A submit that will navigate is a navigation in flight too: mark it so the
  // touch ripple (touch-ripple.js) keeps pulsing if the response is slow.
  // Registered after the data-confirm handler, so a cancelled confirm is seen
  // as defaultPrevented. Forms with the "Confirming" overlay have their own UI.
  document.addEventListener("submit", (event) => {
    const form = event.target instanceof HTMLFormElement ? event.target : null;
    if (!form || event.defaultPrevented) return;
    if (form.getAttribute("data-confirm-loading") === "true") return;
    if (form.target && form.target.toLowerCase() !== "_self") return;
    clearNavigationState();
    document.documentElement.classList.add(leavingClass);
    resetTimer = window.setTimeout(clearNavigationState, resetAfterMs);
  });

  // Ensure Back/Forward Cache restore clears any leaving/pending state.
  window.addEventListener("pageshow", clearNavigationState);
})();
