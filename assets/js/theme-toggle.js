/*
 * Restore a theme selected in this tab and let the landing page toggle it with
 * a double-click on its empty background. The system preference is the default.
 */
(() => {
  const root = document.documentElement;
  const landing = document.querySelector(".landing");
  const storageKey = "res-imperii-theme";

  // sessionStorage lasts for this browser tab's session; if storage is blocked,
  // the visual toggle still works for the current page without being saved.
  function readSessionTheme() {
    try {
      const saved = window.sessionStorage.getItem(storageKey);
      return saved === "light" || saved === "dark" ? saved : null;
    } catch {
      return null;
    }
  }

  function applyTheme(theme) {
    root.dataset.theme = theme;
    try {
      window.sessionStorage.setItem(storageKey, theme);
    } catch {
      // Keep the in-memory override even when this browser disallows storage.
    }
  }

  const savedTheme = readSessionTheme();
  if (savedTheme) root.dataset.theme = savedTheme;

  if (!landing) return;

  // Only an empty part of the landing canvas has the main element as its event
  // target. Double-clicking navigation, text, or other content does nothing.
  landing.addEventListener("dblclick", (event) => {
    if (event.target !== landing) return;

    const systemIsDark = window.matchMedia("(prefers-color-scheme: dark)").matches;
    const currentTheme = root.dataset.theme || (systemIsDark ? "dark" : "light");
    applyTheme(currentTheme === "dark" ? "light" : "dark");
  });
})();
