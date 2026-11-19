/*
 * Selection and confirmation controls for the authenticated journal catalog.
 */
(() => {
  const selectAll = document.getElementById("select-all-entries");
  const countLabel = document.getElementById("selected-count");
  const batchForm = document.getElementById("batch-form");
  const checkboxes = Array.from(document.querySelectorAll(".entry-select-cb"));
  const selectionButtons = Array.from(document.querySelectorAll("[data-requires-selection='true']"));

  if (!batchForm || checkboxes.length === 0) return;

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
      syncSelectionState();
    });
  }

  checkboxes.forEach((cb) => {
    cb.addEventListener("change", syncSelectionState);
  });

  batchForm.addEventListener("submit", (e) => {
    const submitter = e.submitter;
    if (!submitter) return;
    const action = submitter.value;
    const selectedCount = checkboxes.filter((cb) => cb.checked).length;

    if (action === "delete_selected") {
      if (!confirm(`Delete ${selectedCount} selected entry/entries permanently?`)) {
        e.preventDefault();
      }
    } else if (action === "delete_all") {
      if (!confirm("Delete ALL journal entries permanently? This cannot be undone.")) {
        e.preventDefault();
      }
    } else if (action === "unpublish_all") {
      if (!confirm("Unpublish ALL journal entries so they are hidden from the public catalog?")) {
        e.preventDefault();
      }
    }
  });

  syncSelectionState();
})();
