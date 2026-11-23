(function () {
  "use strict";

  // "Confirming, please wait." speaks the same water language as /login/confirm
  // and the journals catalog: sunlit rings spread from the phrase's centre while
  // water-ripple.js refracts the phrase along each crest. The per-character
  // centre-out wave itself is pure CSS (views/loading/loading.css).
  var water = null;

  function startWater(root) {
    if (water || !root || !window.ResImperiiWater) return;
    var rings = root.querySelector(".loading-water-ripple");
    if (!rings) return;
    // Refract the whole linen surface (page or overlay), not just the phrase:
    // the bent weave is what makes the water read as water.
    water = window.ResImperiiWater.attach(root, rings, { amplitude: 9 });
  }

  function stopWater() {
    if (water) water.detach();
    water = null;
  }

  // Standalone /loading page: the ripple runs for the page's whole life.
  var page = document.querySelector(".loading-page");
  if (page) startWater(page);

  var overlay = document.getElementById("confirm-loading-overlay");

  function showOverlay() {
    if (!overlay) return;
    overlay.removeAttribute("hidden");
    void overlay.offsetWidth;
    overlay.classList.add("is-visible");
    startWater(overlay);
  }

  function hideOverlay() {
    if (!overlay) return;
    stopWater();
    overlay.classList.remove("is-visible");
    overlay.setAttribute("hidden", "");
  }

  window.ResImperiiLoading = {
    show: showOverlay,
    hide: hideOverlay,
  };

  if (!overlay) return;

  // Rendered visible (e.g. vacate-space auto-resubmit): start immediately.
  if (overlay.classList.contains("is-visible")) startWater(overlay);

  var forms = document.querySelectorAll('form[data-confirm-loading="true"]');
  for (var i = 0; i < forms.length; i++) {
    forms[i].addEventListener("submit", function (event) {
      if (event.defaultPrevented) {
        return;
      }
      showOverlay();
    });
  }

  // Hide overlay cleanly if the user navigates back via browser bfcache.
  window.addEventListener("pageshow", function (event) {
    if (event.persisted) {
      hideOverlay();
    }
  });
})();
