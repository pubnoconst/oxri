/*
 * Restore a theme selected in this tab and allow toggling it with a double-click
 * on any empty part of the background site-wide. The system preference is default.
 */
(() => {
  const root = document.documentElement;
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

  // The browser chrome (status bar, installed-app title bar) follows the page
  // colour of the chosen theme; layout.go sets the system-theme defaults.
  /** @param {"light" | "dark"} theme */
  function syncThemeColor(theme) {
    const page = theme === "dark" ? "#0f1113" : "#f5f4f0";
    document.querySelectorAll('meta[name="theme-color"]').forEach((m) => {
      m.setAttribute("content", page);
    });
  }

  /** @param {"light" | "dark"} theme */
  function applyTheme(theme) {
    root.dataset.theme = theme;
    syncThemeColor(theme);
    try {
      window.sessionStorage.setItem(storageKey, theme);
    } catch {
      // Keep the in-memory override even when this browser disallows storage.
    }
  }

  const savedTheme = readSessionTheme();
  if (savedTheme) {
    root.dataset.theme = savedTheme;
    syncThemeColor(savedTheme);
  }

  // Double-clicking on empty background toggles theme site-wide.
  // Interactions with text selections, links, buttons, form controls, code,
  // or embedded media do not trigger the toggle.
  window.addEventListener("dblclick", (event) => {
    // Ignore interactive elements, form controls, links, code, and embedded media
    if (!(event.target instanceof Element)) return;
    const interactive = event.target.closest(
      "a, button, input, textarea, select, label, code, pre, iframe, video, audio, [contenteditable='true']"
    );
    if (interactive) return;

    // Ignore intentional text selection gestures
    const selection = window.getSelection();
    if (selection && selection.toString().trim().length > 0) return;

    const systemIsDark = window.matchMedia("(prefers-color-scheme: dark)").matches;
    const currentTheme = root.dataset.theme || (systemIsDark ? "dark" : "light");
    applyTheme(currentTheme === "dark" ? "light" : "dark");
  });
})();
