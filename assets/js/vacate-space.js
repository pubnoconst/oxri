(function () {
  "use strict";

  var section = document.getElementById("vacate-progress-section");
  if (!section) {
    return;
  }

  var requiredBytes = parseInt(section.getAttribute("data-required-bytes") || "0", 10);
  if (isNaN(requiredBytes) || requiredBytes < 0) {
    requiredBytes = 0;
  }

  var progressBar = document.getElementById("vacate-progress-bar");
  var progressFill = document.getElementById("vacate-progress-fill");
  var progressTrack = section.querySelector(".vacate-progress-track");
  var progressReadout = document.getElementById("vacate-progress-readout");
  var deleteSelectedBtn = document.getElementById("vacate-selected-btn");
  var resubmitForm = document.getElementById("resubmit-journal-form");
  var checkboxes = Array.prototype.slice.call(document.querySelectorAll(".vacate-entry-cb"));

  function formatBytes(b) {
    if (b < 0) b = 0;
    var kb = 1024;
    var mb = 1024 * kb;
    var gb = 1024 * mb;
    if (b >= gb) return (b / gb).toFixed(2) + " GB";
    if (b >= mb) return (b / mb).toFixed(2) + " MB";
    if (b >= kb) return (b / kb).toFixed(1) + " KB";
    return b + " B";
  }

  function updateProgress() {
    var selectedBytes = 0;
    var selectedCount = 0;
    for (var i = 0; i < checkboxes.length; i++) {
      if (checkboxes[i].checked) {
        selectedCount++;
        var b = parseInt(checkboxes[i].getAttribute("data-bytes") || "0", 10);
        if (!isNaN(b) && b > 0) {
          selectedBytes += b;
        }
      }
    }

    var pct = 100;
    if (requiredBytes > 0) {
      pct = Math.min(100, Math.floor((selectedBytes * 100) / requiredBytes));
    }
    var sufficient = selectedBytes >= requiredBytes && (requiredBytes === 0 || selectedCount > 0);

    if (progressBar) {
      progressBar.value = requiredBytes > 0 ? Math.min(selectedBytes, requiredBytes) : 1;
    }
    if (progressTrack) {
      progressTrack.setAttribute(
        "aria-valuenow",
        String(requiredBytes > 0 ? Math.min(selectedBytes, requiredBytes) : 1)
      );
    }
    if (progressFill) {
      progressFill.style.width = pct + "%";
      if (selectedBytes >= requiredBytes) {
        progressFill.classList.add("is-complete");
      } else {
        progressFill.classList.remove("is-complete");
      }
    }
    if (progressReadout) {
      if (requiredBytes <= 0 && selectedCount === 0) {
        progressReadout.textContent = "Sufficient space vacated (100%)";
      } else {
        progressReadout.textContent =
          formatBytes(selectedBytes) +
          " (" +
          selectedBytes +
          " B) / " +
          formatBytes(requiredBytes) +
          " (" +
          pct +
          "%)";
      }
    }
    if (deleteSelectedBtn) {
      deleteSelectedBtn.disabled = !(selectedCount > 0 && sufficient);
    }
  }

  for (var i = 0; i < checkboxes.length; i++) {
    checkboxes[i].addEventListener("change", updateProgress);
  }

  updateProgress();

  // Once sufficient space has been vacated after a vacate action, automatically resubmit the pending form.
  if (
    resubmitForm &&
    resubmitForm.getAttribute("data-auto-resubmit") === "true" &&
    requiredBytes <= 0
  ) {
    resubmitForm.submit();
  }
})();
