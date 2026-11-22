(function () {
  "use strict";

  var overlay = document.getElementById("confirm-loading-overlay");

  function showOverlay() {
    if (!overlay) return;
    overlay.removeAttribute("hidden");
    void overlay.offsetWidth;
    overlay.classList.add("is-visible");
  }

  function hideOverlay() {
    if (!overlay) return;
    overlay.classList.remove("is-visible");
    overlay.setAttribute("hidden", "");
  }

  window.ResImperiiLoading = {
    show: showOverlay,
    hide: hideOverlay,
  };

  if (!overlay) return;

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
