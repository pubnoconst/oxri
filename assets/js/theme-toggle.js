/*
 * Light and dark. The first visit follows the device's setting; the theme
 * switch at the top right of every page (layout.ThemeToggle), or a
 * double-click on any empty part of the background, flips it, and the choice
 * is remembered across visits. assets.ThemeInitJS (inline, in <head>)
 * applies a remembered choice before the first paint, so no page flashes the
 * other theme; this script keeps the browser chrome in step and switches.
 */
(() => {
  const root = document.documentElement;
  const storageKey = "res-imperii-theme";

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
      window.localStorage.setItem(storageKey, theme);
    } catch {
      // Storage blocked: the switch still works for this page.
    }
  }

  function toggle() {
    const systemIsDark = window.matchMedia("(prefers-color-scheme: dark)").matches;
    const current = root.dataset.theme || (systemIsDark ? "dark" : "light");
    applyTheme(current === "dark" ? "light" : "dark");
  }

  // A choice from before this change lasted only for its tab
  // (sessionStorage); carry it over once.
  try {
    const old = window.sessionStorage.getItem(storageKey);
    if ((old === "light" || old === "dark") && !window.localStorage.getItem(storageKey)) {
      window.localStorage.setItem(storageKey, old);
      root.dataset.theme = old;
    }
    window.sessionStorage.removeItem(storageKey);
  } catch {
    // Storage blocked: nothing to carry over.
  }

  const saved = root.dataset.theme;
  if (saved === "light" || saved === "dark") syncThemeColor(saved);

  document.addEventListener("click", (event) => {
    const button = event.target instanceof Element ? event.target.closest("[data-theme-toggle]") : null;
    if (button) toggle();
  });

  // Double-clicking on empty background toggles too. Interactions with text
  // selections, links, buttons, form controls, code, or embedded media do not.
  window.addEventListener("dblclick", (event) => {
    if (!(event.target instanceof Element)) return;
    const interactive = event.target.closest(
      "a, button, input, textarea, select, label, code, pre, iframe, video, audio, [contenteditable='true']"
    );
    if (interactive) return;
    const selection = window.getSelection();
    if (selection && selection.toString().trim().length > 0) return;
    toggle();
  });
})();
