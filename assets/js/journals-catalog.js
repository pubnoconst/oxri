/*
 * Selection and confirmation controls for the authenticated journal catalog.
 * Re-binds automatically whenever HTMX swaps in the catalog fragment (/journals/content).
 */
(() => {
  function initCatalogControls() {
    const selectAll = /** @type {HTMLInputElement | null} */ (document.getElementById("select-all-entries"));
    const countLabel = document.getElementById("selected-count");
    const batchForm = document.getElementById("batch-form");
    const checkboxes = /** @type {HTMLInputElement[]} */ (Array.from(document.querySelectorAll(".entry-select-cb")));
    const selectionButtons = /** @type {HTMLButtonElement[]} */ (Array.from(document.querySelectorAll("[data-requires-selection='true']")));

    if (!batchForm || batchForm.dataset.bound === "true" || checkboxes.length === 0) return;
    batchForm.dataset.bound = "true";

    function syncSelectionState() {
      const selectedCount = checkboxes.filter((cb) => cb.checked).length;
      if (countLabel) {
        countLabel.textContent = `${selectedCount} selected`;
      }
      if (selectAll) {
        selectAll.checked = selectedCount > 0 && selectedCount === checkboxes.length;
        selectAll.indeterminate = selectedCount > 0 && selectedCount < checkboxes.length;
      }
      selectionButtons.forEach((btn) => {
        btn.disabled = selectedCount === 0;
        btn.setAttribute("aria-disabled", btn.disabled ? "true" : "false");
      });
    }

    if (selectAll) {
      selectAll.addEventListener("change", () => {
        checkboxes.forEach((cb) => {
          cb.checked = selectAll.checked;
        });
        if (batchPrompt) batchPrompt.replaceChildren();
        syncSelectionState();
      });
    }

    // A batch prompt (views/confirm, fetched by HTMX) names how many entries
    // it acts on, so a change of selection closes it: it would be stale.
    const batchPrompt = document.getElementById("batch-confirm");
    function closeBatchPrompt() {
      if (batchPrompt) batchPrompt.replaceChildren();
    }

    checkboxes.forEach((cb) => {
      cb.addEventListener("change", () => {
        closeBatchPrompt();
        syncSelectionState();
      });
    });

    syncSelectionState();
  }

  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", initCatalogControls);
  } else {
    initCatalogControls();
  }
  document.addEventListener("htmx:afterSwap", initCatalogControls);
})();
