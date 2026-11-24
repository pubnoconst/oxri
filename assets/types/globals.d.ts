// Types for the globals the site's scripts share or receive from vendored
// libraries. Only `tsc` reads this file (tsconfig.json); it is not embedded
// or served. The scripts themselves stay plain JavaScript with JSDoc types.
//
// Our own globals are each script's whole public surface; the Go test
// TestScriptGlobalsMatchTheirCallers keeps them as small as their callers.

/** A dot-matrix motion (views/loading, HIG §6). */
type DotMotion = "puzzle" | "twinkle" | "comet";

interface ResImperiiDotsAPI {
  /** The full loader: a 3x3 matrix in `motion` beside `label` (dot-matrix.js). */
  create(label: string, motion: DotMotion, seed?: string): HTMLElement;
}

interface ResImperiiMediaAPI {
  /** An http(s) URL, or null for anything else (journal-media.js). */
  safeUrl(value: string | null | undefined): URL | null;
  /** The element that shows a media link, or null if there is no preview for it. */
  createMedia(url: URL): HTMLElement | null;
}

interface ResImperiiLoadingAPI {
  /** Show the "Confirming, please wait." overlay (loading-transition.js). */
  show(): void;
}

/** The part of X's widgets.js the site calls (journal-media.js). */
interface TwitterWidgets {
  widgets: {
    createTweet(id: string, target: HTMLElement, options?: Record<string, string | boolean>): Promise<HTMLElement | undefined>;
  };
  ready?(callback: (twttr: TwitterWidgets) => void): void;
}

/** Vendored markdown-it (assets/js/markdown-it.min.js). */
interface MarkdownIt {
  render(source: string): string;
}

/** Vendored DOMPurify (assets/js/purify.min.js). */
interface DOMPurifyAPI {
  sanitize(dirty: string, config?: Record<string, unknown>): string;
}

interface Window {
  ResImperiiDots: ResImperiiDotsAPI;
  ResImperiiMedia: ResImperiiMediaAPI;
  ResImperiiLoading: ResImperiiLoadingAPI;
  twttr?: TwitterWidgets;
  markdownit(options?: Record<string, boolean>): MarkdownIt;
  DOMPurify: DOMPurifyAPI;
}
