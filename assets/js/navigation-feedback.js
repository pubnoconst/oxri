/* Smooth same-origin page transition ease-out and pending navigation feedback. */
(() => {
  const pendingClass = "navigation-pending";
  const delayMs = 180;
  const resetAfterMs = 35000;
  /** @type {number | undefined} */
  let pendingTimer;
  /** @type {number | undefined} */
  let resetTimer;

  // A slow navigation shows the site's dot matrix at the bottom of the screen
  // (site.css .nav-dot-loading, visible only while .navigation-pending). The
  // page is not dimmed: the dots are the acknowledgement. Following a link is
  // a download (twinkle, "Loading"); a POST is the site working on what you
  // did (puzzle, "Working"): signing in, saving a book.
  /** @type {HTMLElement | null} */
  let navDots = null;
  /** @param {boolean} working */
  function showNavDots(working) {
    if (navDots) navDots.remove();
    navDots = document.createElement("div");
    navDots.className = "nav-dot-loading";
    navDots.setAttribute("role", "status");
    navDots.append(window.ResImperiiDots.create(working ? "Working" : "Loading", working ? "puzzle" : "twinkle"));
    document.body.append(navDots);
  }

  // Every tap is acknowledged at once by the control itself (HIG §1.7): the
  // link or button shows a pending state (site.css .is-pending) until the
  // next page or the swapped results arrive; the dots follow only if the
  // wait is real.
  /** @type {Element | null} */
  let pendingControl = null;
  /** @param {Element | null} control */
  function markPending(control) {
    if (!control) return;
    pendingControl = control;
    control.classList.add("is-pending");
    control.setAttribute("aria-busy", "true");
  }

  function clearNavigationState() {
    window.clearTimeout(pendingTimer);
    window.clearTimeout(resetTimer);
    document.documentElement.classList.remove(pendingClass);
    if (pendingControl) {
      pendingControl.classList.remove("is-pending");
      pendingControl.removeAttribute("aria-busy");
      pendingControl = null;
    }
  }

  document.addEventListener("click", (event) => {
    if (!(event instanceof MouseEvent) || event.defaultPrevented) return;
    if (event.button !== 0 || event.metaKey || event.ctrlKey || event.shiftKey || event.altKey) return;

    const link = event.target instanceof Element ? event.target.closest("a[href]") : null;
    if (!(link instanceof HTMLAnchorElement)) return;
    if (!link || link.hasAttribute("download")) return;
    // HTMX links (e.g. a delete that fetches its confirmation) stay on the page,
    // and so do the catalog's in-place swaps (see below; acknowledged there).
    if (link.hasAttribute("hx-get")) return;
    if (link.closest('[hx-boost="true"]')) {
      clearNavigationState();
      markPending(link);
      return;
    }
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
    markPending(link);
    pendingTimer = window.setTimeout(() => {
      if (event.defaultPrevented) return;
      showNavDots(false);
      document.documentElement.classList.add(pendingClass);
      resetTimer = window.setTimeout(clearNavigationState, resetAfterMs);
    }, delayMs);
  });

  // Confirmation prompts (views/confirm) arrive by HTMX into a .confirm-slot.
  // "Keep" or Escape closes the prompt here, with no round trip, and gives
  // focus back to the control that asked.
  /** @param {Element} inside */
  function closePrompt(inside) {
    const slot = inside.closest(".confirm-slot");
    if (!slot) return;
    slot.replaceChildren();
    const trigger = document.querySelector(`[aria-controls="${slot.id}"]`);
    if (trigger instanceof HTMLElement) trigger.focus();
  }
  document.addEventListener("click", (event) => {
    const keep = event.target instanceof Element ? event.target.closest("[data-confirm-keep]") : null;
    if (keep) closePrompt(keep);
  });
  document.addEventListener("keydown", (event) => {
    if (event.key !== "Escape" || !(event.target instanceof Element)) return;
    if (event.target.closest(".confirm-inline")) closePrompt(event.target);
  });

  // A submit that will navigate is a navigation in flight too: show the dots
  // if the response is slow.
  // Forms with the "Confirming" overlay have their own UI.
  document.addEventListener("submit", (event) => {
    const form = event.target instanceof HTMLFormElement ? event.target : null;
    if (!form || event.defaultPrevented) return;
    if (form.getAttribute("data-confirm-loading") === "true") return;
    const submitter = /** @type {SubmitEvent} */ (event).submitter || form.querySelector('[type="submit"]');
    if (form.closest('[hx-boost="true"]')) {
      // swapped in place (below)
      clearNavigationState();
      markPending(submitter);
      return;
    }
    if (form.target && form.target.toLowerCase() !== "_self") return;
    clearNavigationState();
    markPending(submitter);
    const working = (form.getAttribute("method") || "get").toLowerCase() === "post";
    pendingTimer = window.setTimeout(() => {
      showNavDots(working);
      document.documentElement.classList.add(pendingClass);
    }, delayMs);
    resetTimer = window.setTimeout(clearNavigationState, resetAfterMs);
  });

  // The catalog's search, tags and pages swap the results in place
  // (views/journals catalogSwap): the current results
  // stay until the new ones replace them, and the dots appear only if the
  // answer is slow, as for a page. If the request fails (offline, a server
  // error), the same URL is loaded as a page instead, so a search always ends
  // in results or the offline page.
  /** @param {Event} event */
  const swapElt = (event) => {
    const detail = /** @type {{elt?: unknown, pathInfo?: {requestPath?: string, finalRequestPath?: string}} | undefined} */ (
      /** @type {CustomEvent} */ (event).detail
    );
    const elt = detail && detail.elt;
    return elt instanceof Element && elt.closest('[hx-boost="true"]') ? detail : null;
  };
  // The swap's later events name the new stage, not the link or form that
  // asked, so whether one is in flight is kept here.
  let swapInFlight = false;
  document.addEventListener("htmx:beforeRequest", (event) => {
    if (!swapElt(event)) return;
    swapInFlight = true;
    window.clearTimeout(pendingTimer);
    window.clearTimeout(resetTimer);
    pendingTimer = window.setTimeout(() => {
      showNavDots(false);
      document.documentElement.classList.add(pendingClass);
    }, delayMs);
    resetTimer = window.setTimeout(clearNavigationState, resetAfterMs);
  });
  for (const done of ["htmx:afterSwap", "htmx:afterRequest"]) {
    document.addEventListener(done, () => {
      if (!swapInFlight) return;
      if (done === "htmx:afterSwap") swapInFlight = false;
      clearNavigationState();
    });
  }
  for (const failure of ["htmx:sendError", "htmx:responseError", "htmx:timeout"]) {
    document.addEventListener(failure, (event) => {
      const detail = swapElt(event);
      if (!detail) return;
      swapInFlight = false;
      clearNavigationState();
      const path = detail.pathInfo && (detail.pathInfo.finalRequestPath || detail.pathInfo.requestPath);
      if (path) window.location.assign(path);
    });
  }

  // Ensure Back/Forward Cache restore clears any pending state.
  window.addEventListener("pageshow", clearNavigationState);
})();
