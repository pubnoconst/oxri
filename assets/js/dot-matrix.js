/*
 * The site's loading cue: a 3x3 dot matrix (site.css .dot-matrix), in three
 * motions that each mean something (RATIONALE 048):
 *
 *   puzzle   "working on what you did": sign-in, saving an entry or a book,
 *            the Confirming overlay. A gap wanders the grid and dots slide
 *            into it (scramble), then the walk is retraced (solve).
 *   twinkle  "downloading": tweets, the catalog, following a link. Dots glint
 *            in a scattered order.
 *   comet    "waiting on you": passkey prompts. A bright head runs clockwise
 *            round the edge with a fading tail.
 *
 * Puzzle and twinkle are seeded: every instance gets its own walk or glint
 * order (and a slightly different period), so twelve tweet cards loading at
 * once never flash in unison. A seed string makes an instance reproducible
 * (e.g. a tweet ID); without one a random seed is used.
 *
 * Only opacity and transform are animated (Web Animations API), which the
 * compositor keeps running while the main thread is busy. Under reduced
 * motion nothing animates; the dots stay still (site.css).
 *
 * Markup comes from views/loading (Go) or create() here; a matrix declares its
 * motion with data-motion and is started once (data-started).
 */
(() => {
  "use strict";

  // One cell = 4 px dot + 3 px gap (site.css .dot-matrix). Slides move whole cells.
  const STEP_PX = 7;
  const REST = 0.14;
  const RING = [0, 1, 2, 5, 8, 7, 6, 3]; // perimeter, clockwise from top-left

  const reducedMotion = () =>
    !!(window.matchMedia && window.matchMedia("(prefers-reduced-motion: reduce)").matches);

  // ---- seeded randomness (FNV-1a hash -> mulberry32) -----------------------
  function hash(str) {
    let h = 0x811c9dc5;
    for (let i = 0; i < str.length; i++) {
      h ^= str.charCodeAt(i);
      h = Math.imul(h, 0x01000193);
    }
    return h >>> 0;
  }

  function rng(seed) {
    let a = typeof seed === "string" ? hash(seed) : seed >>> 0;
    return () => {
      a = (a + 0x6d2b79f5) >>> 0;
      let t = a;
      t = Math.imul(t ^ (t >>> 15), t | 1);
      t ^= t + Math.imul(t ^ (t >>> 7), t | 61);
      return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
    };
  }

  const randomSeed = () => Math.floor(Math.random() * 4294967296);

  // ---- motions ---------------------------------------------------------------
  const neighbours = (k) => {
    const r = Math.floor(k / 3), c = k % 3, out = [];
    if (r > 0) out.push(k - 3);
    if (r < 2) out.push(k + 3);
    if (c > 0) out.push(k - 1);
    if (c < 2) out.push(k + 1);
    return out;
  };

  // puzzle: the gap starts on a seeded cell and takes a seeded walk of 4-6
  // moves (never straight back), then retraces it, so every dot ends where it
  // began and the loop is seamless. Each move slides one dot into the gap;
  // the moving dot brightens.
  function puzzle(dots, rand) {
    const start = Math.floor(rand() * 9);
    const walk = [start];
    const steps = 4 + Math.floor(rand() * 3);
    for (let i = 0; i < steps; i++) {
      const here = walk[walk.length - 1];
      const back = walk.length > 1 ? walk[walk.length - 2] : -1;
      const options = neighbours(here).filter((n) => n !== back);
      walk.push(options[Math.floor(rand() * options.length)]);
    }
    const path = walk.concat(walk.slice(0, -1).reverse()); // scramble, then solve
    const moves = path.length - 1;
    const duration = moves * 280;

    const at = dots.map((_, i) => i); // element i -> current cell
    at[start] = -1; // the element on the gap's cell is the hole: it never moves
    const frames = dots.map(() => [{ offset: 0, x: 0, y: 0, o: 0.45 }]);
    for (let m = 0; m < moves; m++) {
      const gap = path[m], from = path[m + 1];
      const el = at.indexOf(from);
      const home = el, t0 = m / moves, t1 = (m + 0.55) / moves;
      const pos = (cell) => ({ x: (cell % 3) - (home % 3), y: Math.floor(cell / 3) - Math.floor(home / 3) });
      const a = pos(from), b = pos(gap);
      frames[el].push({ offset: t0, x: a.x, y: a.y, o: 0.45 });
      frames[el].push({ offset: (t0 + t1) / 2, x: (a.x + b.x) / 2, y: (a.y + b.y) / 2, o: 0.95 });
      frames[el].push({ offset: t1, x: b.x, y: b.y, o: 0.45 });
      at[el] = gap;
    }

    dots.forEach((dot, i) => {
      if (i === start) {
        dot.style.opacity = "0"; // the gap
        return;
      }
      const f = frames[i];
      if (f.length === 1) {
        dot.style.opacity = "0.45"; // never moves this lap
        return;
      }
      f.push({ offset: 1, x: 0, y: 0, o: 0.45 });
      dot.animate(
        f.map((k) => ({
          offset: k.offset,
          opacity: k.o,
          transform: `translate(${k.x * STEP_PX}px, ${k.y * STEP_PX}px)`,
          easing: "ease-in-out",
        })),
        { duration, iterations: Infinity }
      );
    });
  }

  // twinkle: one glint shape for every dot, shifted by a seeded shuffle of
  // nine evenly spread phases (with a little jitter), so glints never clump.
  function twinkle(dots, rand) {
    const duration = 1600 + Math.floor(rand() * 500);
    const phases = dots.map((_, i) => (i + rand() * 0.6) / 9);
    for (let i = phases.length - 1; i > 0; i--) {
      const j = Math.floor(rand() * (i + 1));
      [phases[i], phases[j]] = [phases[j], phases[i]];
    }
    const glint = [
      { offset: 0, opacity: REST },
      { offset: 0.06, opacity: 0.9, easing: "ease-out" },
      { offset: 0.16, opacity: 0.35, easing: "ease-in" },
      { offset: 0.34, opacity: REST },
      { offset: 1, opacity: REST },
    ];
    dots.forEach((dot, i) => {
      dot.animate(glint, { duration, iterations: Infinity, delay: -phases[i] * duration });
    });
  }

  // comet: a head runs clockwise round the edge with a fading tail; the centre
  // glows faintly. Not seeded: it means "your turn", so it should look the same
  // every time.
  function comet(dots) {
    const duration = 1200;
    const tail = [
      { offset: 0, opacity: 0.92 },
      { offset: 1 / 8, opacity: 0.55 },
      { offset: 2 / 8, opacity: 0.32 },
      { offset: 3 / 8, opacity: 0.2 },
      { offset: 4 / 8, opacity: 0.1 },
      { offset: 7 / 8, opacity: 0.1 },
      { offset: 1, opacity: 0.92 },
    ];
    dots.forEach((dot, i) => {
      if (i === 4) {
        dot.style.opacity = "0.26";
        return;
      }
      const idx = RING.indexOf(i);
      dot.animate(tail, { duration, iterations: Infinity, delay: -(1 - idx / 8) * duration });
    });
  }

  const MOTIONS = { puzzle, twinkle, comet };

  // ---- public API ------------------------------------------------------------
  function start(matrix) {
    if (!matrix || matrix.dataset.started === "1") return;
    matrix.dataset.started = "1";
    if (reducedMotion()) return;
    const motion = MOTIONS[matrix.dataset.motion] || twinkle;
    const seed = matrix.dataset.seed ? matrix.dataset.seed : randomSeed();
    motion(Array.from(matrix.children), rng(seed));
  }

  function matrix(motion, seed) {
    const m = document.createElement("span");
    m.className = "dot-matrix";
    m.dataset.motion = motion;
    if (seed !== undefined && seed !== null) m.dataset.seed = String(seed);
    m.setAttribute("aria-hidden", "true");
    for (let i = 0; i < 9; i++) m.append(document.createElement("i"));
    start(m);
    return m;
  }

  // create builds the full loader: matrix + label (the Go twin is loading.Dots).
  function create(label, motion, seed) {
    const wrap = document.createElement("span");
    wrap.className = "dot-loading";
    const text = document.createElement("span");
    text.className = "dot-loading-label";
    text.textContent = label;
    wrap.append(matrix(motion, seed), text);
    return wrap;
  }

  // Server-rendered matrices (views/loading) start on load, and any that
  // arrive later (HTMX swaps) start as they are added.
  function startAll(root) {
    if (root.matches && root.matches(".dot-matrix[data-motion]")) start(root);
    if (root.querySelectorAll) root.querySelectorAll(".dot-matrix[data-motion]").forEach(start);
  }

  function boot() {
    startAll(document);
    if (typeof MutationObserver === "undefined") return;
    new MutationObserver((records) => {
      for (const r of records) r.addedNodes.forEach((n) => n.nodeType === 1 && startAll(n));
    }).observe(document.body, { childList: true, subtree: true });
  }

  if (document.readyState === "loading") document.addEventListener("DOMContentLoaded", boot);
  else boot();

  window.ResImperiiDots = { create, matrix, start };
})();
