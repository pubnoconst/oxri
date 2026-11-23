/*
 * Realistic, calm water-surface refraction for Res Imperii loading states.
 *
 * Content sits "under the water": an SVG radial displacement filter bends text
 * and borders gently along each expanding wavefront using a precomputed decaying
 * wave packet. Positions, sizes, and strengths are synced every frame to the CSS
 * sunlit ripple rings, and all active ripples share one rAF loop and one map PNG.
 */
(() => {
  "use strict";

  const SVG_NS = "http://www.w3.org/2000/svg";
  const XLINK_NS = "http://www.w3.org/1999/xlink";

  // Crest radius as a fraction of the ring element's diameter (matches site.css gradient).
  const SUN_CREST = 0.335;
  // Crest radius inside the refraction map, in units of the map's half-size.
  const MAP_CREST = 0.67;
  // Gentle, serene peak displacement strength (px) at full wave opacity.
  const DEFAULT_AMPLITUDE = 8.5;

  const prefersReducedMotion = () =>
    window.matchMedia && window.matchMedia("(prefers-reduced-motion: reduce)").matches;

  let cachedMapURL = null;
  function getMapURL() {
    if (cachedMapURL) return cachedMapURL;

    const N = 256;
    const canvas = document.createElement("canvas");
    canvas.width = N;
    canvas.height = N;
    const ctx = canvas.getContext("2d");
    const img = ctx.createImageData(N, N);

    // Calm, smooth primary wave with a whisper-soft trailing echo
    const sigma = 0.13;
    const wavelength = 0.34;
    const k = (2 * Math.PI) / wavelength;
    const height = (r) =>
      Math.exp(-((r - MAP_CREST) ** 2) / (2 * sigma * sigma)) * Math.cos(k * (r - MAP_CREST));
    const slope = (r) => {
      const e = 0.002;
      return (height(r + e) - height(r - e)) / (2 * e);
    };

    let maxSlope = 0;
    for (let r = 0; r <= 1; r += 0.002) {
      maxSlope = Math.max(maxSlope, Math.abs(slope(r)));
    }
    if (maxSlope === 0) maxSlope = 1;

    const half = N / 2;
    for (let y = 0; y < N; y++) {
      for (let x = 0; x < N; x++) {
        const dx = (x + 0.5 - half) / half;
        const dy = (y + 0.5 - half) / half;
        const r = Math.hypot(dx, dy);
        let vx = 0;
        let vy = 0;
        if (r > 1e-6 && r < 1) {
          const edge = r > 0.88 ? Math.max(0, (1 - r) / 0.12) : 1;
          const g = (slope(r) / maxSlope) * edge;
          vx = (g * dx) / r;
          vy = (g * dy) / r;
        }
        const i = (y * N + x) * 4;
        img.data[i] = Math.round(128 + 127 * vx);
        img.data[i + 1] = Math.round(128 + 127 * vy);
        img.data[i + 2] = 128;
        img.data[i + 3] = 255;
      }
    }
    ctx.putImageData(img, 0, 0);
    cachedMapURL = canvas.toDataURL("image/png");
    return cachedMapURL;
  }

  let sharedDefs = null;
  function getSharedDefs() {
    if (sharedDefs && sharedDefs.isConnected) return sharedDefs;
    const svg = document.createElementNS(SVG_NS, "svg");
    svg.setAttribute("id", "res-imperii-water-defs");
    svg.setAttribute("class", "water-refract-svg");
    svg.setAttribute("aria-hidden", "true");
    svg.setAttribute("width", "0");
    svg.setAttribute("height", "0");
    svg.setAttribute("focusable", "false");
    const defs = document.createElementNS(SVG_NS, "defs");
    svg.appendChild(defs);
    document.body.appendChild(svg);
    sharedDefs = defs;
    return defs;
  }

  let nextFilterID = 1;
  function createDynamicFilter(waveCount) {
    const id = `water-refract-dyn-${nextFilterID++}`;
    const filter = document.createElementNS(SVG_NS, "filter");
    filter.setAttribute("id", id);
    filter.setAttribute("filterUnits", "userSpaceOnUse");
    filter.setAttribute("primitiveUnits", "userSpaceOnUse");
    filter.setAttribute("x", "-20");
    filter.setAttribute("y", "-20");
    filter.setAttribute("width", "2000");
    filter.setAttribute("height", "2000");
    filter.setAttribute("color-interpolation-filters", "sRGB");

    const flood = document.createElementNS(SVG_NS, "feFlood");
    flood.setAttribute("flood-color", "rgb(128,128,128)");
    flood.setAttribute("flood-opacity", "1");
    flood.setAttribute("result", "neutral");
    filter.appendChild(flood);

    const layers = [];
    for (let i = 1; i <= waveCount; i++) {
      const inName = i === 1 ? "SourceGraphic" : `d${i - 1}`;

      const feImg = document.createElementNS(SVG_NS, "feImage");
      feImg.setAttribute("preserveAspectRatio", "none");
      feImg.setAttribute("x", "-10");
      feImg.setAttribute("y", "-10");
      feImg.setAttribute("width", "1");
      feImg.setAttribute("height", "1");
      feImg.setAttribute("result", `m${i}`);
      filter.appendChild(feImg);

      const feMerge = document.createElementNS(SVG_NS, "feMerge");
      feMerge.setAttribute("result", `map${i}`);
      const n1 = document.createElementNS(SVG_NS, "feMergeNode");
      n1.setAttribute("in", "neutral");
      const n2 = document.createElementNS(SVG_NS, "feMergeNode");
      n2.setAttribute("in", `m${i}`);
      feMerge.appendChild(n1);
      feMerge.appendChild(n2);
      filter.appendChild(feMerge);

      const feDisp = document.createElementNS(SVG_NS, "feDisplacementMap");
      feDisp.setAttribute("in", inName);
      feDisp.setAttribute("in2", `map${i}`);
      feDisp.setAttribute("scale", "0");
      feDisp.setAttribute("xChannelSelector", "R");
      feDisp.setAttribute("yChannelSelector", "G");
      feDisp.setAttribute("result", `d${i}`);
      filter.appendChild(feDisp);

      layers.push({ img: feImg, disp: feDisp });
    }

    getSharedDefs().appendChild(filter);
    return { filter, id, layers, owned: true };
  }

  function hideLayer(layer) {
    layer.img.setAttribute("x", "-10");
    layer.img.setAttribute("y", "-10");
    layer.img.setAttribute("width", "1");
    layer.img.setAttribute("height", "1");
    layer.disp.setAttribute("scale", "0");
  }

  const activeInstances = new Set();
  let rafId = null;

  function stepAll() {
    rafId = null;
    if (activeInstances.size === 0) return;

    activeInstances.forEach((inst) => {
      if (!inst.stage.isConnected || !inst.overlay.isConnected) {
        inst.detach();
        return;
      }

      const stageRect = inst.stage.getBoundingClientRect();
      if (stageRect.width <= 0 || stageRect.height <= 0) return;

      if (inst.viewportRegion) {
        // Root-element stage (touch ripple): only the visible viewport is ever
        // refracted, so clip the filter region to it rather than filtering the
        // whole (possibly very long) document every frame.
        const x = Math.floor(-stageRect.left) - 20;
        const y = Math.floor(-stageRect.top) - 20;
        const w = window.innerWidth + 40;
        const h = window.innerHeight + 40;
        if (inst.lastX !== x || inst.lastY !== y || inst.lastW !== w || inst.lastH !== h) {
          inst.filter.setAttribute("x", String(x));
          inst.filter.setAttribute("y", String(y));
          inst.filter.setAttribute("width", String(w));
          inst.filter.setAttribute("height", String(h));
          inst.lastX = x;
          inst.lastY = y;
          inst.lastW = w;
          inst.lastH = h;
        }
      } else {
        const w = Math.ceil(stageRect.width) + 40;
        const h = Math.ceil(stageRect.height) + 40;
        if (inst.lastW !== w || inst.lastH !== h) {
          inst.filter.setAttribute("x", "-20");
          inst.filter.setAttribute("y", "-20");
          inst.filter.setAttribute("width", String(w));
          inst.filter.setAttribute("height", String(h));
          inst.lastW = w;
          inst.lastH = h;
        }
      }

      // Scale displacement smoothly down as the overlay fades out
      const overlayOpacity = parseFloat(window.getComputedStyle(inst.overlay).opacity);
      const fadeFactor = Number.isFinite(overlayOpacity) ? overlayOpacity : 1;

      for (let i = 0; i < inst.waves.length; i++) {
        const el = inst.waves[i];
        const layer = inst.layers[i];
        const cs = window.getComputedStyle(el);

        const opacity = (parseFloat(cs.opacity) || 0) * fadeFactor;
        let scale = 0;
        if (cs.transform && cs.transform !== "none") {
          try {
            scale = new DOMMatrixReadOnly(cs.transform).a;
          } catch (_) {
            scale = 0;
          }
        }

        const crestRadius = scale * el.offsetWidth * SUN_CREST;
        if (crestRadius < 3 || opacity < 0.015) {
          hideLayer(layer);
          continue;
        }

        const rect = el.getBoundingClientRect();
        const cx = rect.left + rect.width / 2 - stageRect.left;
        const cy = rect.top + rect.height / 2 - stageRect.top;
        const mapHalf = crestRadius / MAP_CREST;

        if (inst.viewportRegion) {
          // Skip a layer whose wave band (≈ 0.28–1.0 × mapHalf, see getMapURL)
          // doesn't touch the viewport this frame: the viewport lies wholly
          // inside the band's calm centre, or wholly outside the band.
          const vx0 = -stageRect.left;
          const vy0 = -stageRect.top;
          const vx1 = vx0 + window.innerWidth;
          const vy1 = vy0 + window.innerHeight;
          const nearest = Math.hypot(
            Math.max(vx0 - cx, 0, cx - vx1),
            Math.max(vy0 - cy, 0, cy - vy1)
          );
          const farthest = Math.hypot(
            Math.max(Math.abs(cx - vx0), Math.abs(cx - vx1)),
            Math.max(Math.abs(cy - vy0), Math.abs(cy - vy1))
          );
          if (nearest > mapHalf || farthest < mapHalf * 0.28) {
            hideLayer(layer);
            continue;
          }
        }

        layer.img.setAttribute("x", (cx - mapHalf).toFixed(1));
        layer.img.setAttribute("y", (cy - mapHalf).toFixed(1));
        layer.img.setAttribute("width", (mapHalf * 2).toFixed(1));
        layer.img.setAttribute("height", (mapHalf * 2).toFixed(1));
        layer.disp.setAttribute("scale", (inst.amplitude * opacity).toFixed(2));
      }
    });

    if (activeInstances.size > 0) {
      rafId = window.requestAnimationFrame(stepAll);
    }
  }

  function ensureLoop() {
    if (rafId === null && activeInstances.size > 0) {
      rafId = window.requestAnimationFrame(stepAll);
    }
  }

  // ---- Adaptive quality governor -------------------------------------------
  // Refraction cost scales with screen pixels and with every wave layer, so a
  // fast laptop and a five-year-old phone cannot share one setting. The
  // governor measures real frame times while water is moving and settles each
  // device at the richest level it draws smoothly, remembered per browser:
  //   3  drop + every reachable wall echo      2  drop + nearest echo
  //   1  drop only                             0  off: no refraction anywhere
  //                                               (static "received" cue instead)
  const QUALITY_KEY = "res-imperii.water-quality";
  const SLOW_P75_MS = 24; // slower than ~42 fps: step down at once
  const FAST_P75_MS = 18; // ~55 fps or better: counts towards stepping up
  const PROMOTE_AFTER = 3; // consecutive smooth runs before stepping up
  const THROTTLED_P75_MS = 250; // covered/background window, not a slow device: ignore

  function isAppleWebKit() {
    const ua = navigator.userAgent || "";
    return /AppleWebKit/.test(ua) && !/Chrome|Chromium|CriOS|Edg|OPR|Android/.test(ua);
  }

  function startingLevel() {
    try {
      const stored = Number(window.localStorage.getItem(QUALITY_KEY));
      if (Number.isInteger(stored) && stored >= 0 && stored <= 3 && window.localStorage.getItem(QUALITY_KEY) !== null) {
        return stored;
      }
    } catch (_) {
      // storage unavailable (private mode): fall through to a fresh guess
    }
    let level = 3;
    const constrained =
      (window.matchMedia && window.matchMedia("(pointer: coarse)").matches) ||
      (navigator.deviceMemory && navigator.deviceMemory <= 4) ||
      (navigator.hardwareConcurrency && navigator.hardwareConcurrency <= 4);
    if (constrained) level = 2;
    if (navigator.connection && navigator.connection.saveData) level = 1;
    // Safari's SVG displacement on HTML content has historically been slow:
    // start cautious and let measured smoothness earn more.
    if (isAppleWebKit()) level = Math.min(level, 1);
    return level;
  }

  const quality = {
    level: startingLevel(),
    smoothRuns: 0,

    get() {
      return this.level;
    },

    set(level) {
      this.level = Math.max(0, Math.min(3, level));
      this.smoothRuns = 0;
      document.documentElement.classList.toggle("water-off", this.level === 0);
      try {
        window.localStorage.setItem(QUALITY_KEY, String(this.level));
      } catch (_) {
        // not persisted; still applies for this page
      }
    },

    // observe feeds one run's 75th-percentile frame time into the governor.
    observe(p75) {
      if (!(p75 > 0) || p75 >= THROTTLED_P75_MS) return; // throttled window: not evidence
      if (p75 > SLOW_P75_MS) {
        if (this.level > 0) this.set(this.level - 1);
        return;
      }
      if (p75 < FAST_P75_MS) {
        this.smoothRuns += 1;
        if (this.smoothRuns >= PROMOTE_AFTER && this.level < 3) this.set(this.level + 1);
        return;
      }
      this.smoothRuns = 0; // acceptable but not fast: hold
    },

    // measure samples frame times until the returned end(report) is called;
    // end(false) discards the run (e.g. interrupted by a new press).
    measure() {
      if (this.level === 0) return () => {};
      const frames = [];
      let last = 0;
      let raf = 0;
      const hiddenAtStart = document.hidden;
      const tick = (t) => {
        if (last) frames.push(t - last);
        last = t;
        raf = window.requestAnimationFrame(tick);
      };
      raf = window.requestAnimationFrame(tick);
      let ended = false;
      return (report = true) => {
        if (ended) return;
        ended = true;
        window.cancelAnimationFrame(raf);
        if (!report || hiddenAtStart || document.hidden || frames.length < 8) return;
        const sorted = frames.slice(2).sort((a, b) => a - b); // skip start-up frames
        this.observe(sorted[Math.floor(sorted.length * 0.75)]);
      };
    },
  };
  document.documentElement.classList.toggle("water-off", quality.level === 0);

  function attach(stage, overlay, opts = {}) {
    const noop = { detach() {}, fadeAndDetach() {} };
    if (!stage || !overlay || prefersReducedMotion() || quality.level === 0) return noop;

    // Loading ripples use 3 staggered rings; the touch ripple uses 1 drop plus
    // 4 mirror-image echo sources (opts.maxWaves = 5).
    const maxWaves = typeof opts.maxWaves === "number" ? opts.maxWaves : 3;
    const waves = Array.from(overlay.querySelectorAll(".card-water-wave")).slice(0, maxWaves);
    if (!waves.length) return noop;

    let filter = opts.filter || null;
    let filterId = filter ? filter.id : "";
    let layers = null;
    let owned = false;

    if (filter) {
      layers = waves.map((_, i) => ({
        img: filter.querySelector("#wr-img-" + (i + 1)),
        disp: filter.querySelector("#wr-disp-" + (i + 1)),
      }));
      if (layers.some((l) => !l.img || !l.disp)) {
        filter = null;
      }
    }

    if (!filter) {
      const dyn = createDynamicFilter(waves.length);
      filter = dyn.filter;
      filterId = dyn.id;
      layers = dyn.layers;
      owned = dyn.owned;
    }

    const mapURL = getMapURL();
    layers.forEach((layer) => {
      layer.img.setAttribute("href", mapURL);
      layer.img.setAttributeNS(XLINK_NS, "href", mapURL);
      hideLayer(layer);
    });

    const amplitude = typeof opts.amplitude === "number" ? opts.amplitude : DEFAULT_AMPLITUDE;
    const useClass = Boolean(opts.useClass);
    const prevInlineFilter = stage.style.filter;

    if (useClass) {
      stage.classList.add("is-submerged");
    } else {
      stage.style.filter = `url(#${filterId})`;
    }

    let detached = false;
    const inst = {
      stage,
      overlay,
      waves,
      filter,
      layers,
      amplitude,
      viewportRegion: Boolean(opts.viewportRegion),
      lastX: 0,
      lastY: 0,
      lastW: 0,
      lastH: 0,
      detach() {
        if (detached) return;
        detached = true;
        activeInstances.delete(inst);
        layers.forEach(hideLayer);
        if (useClass) {
          stage.classList.remove("is-submerged");
        } else {
          stage.style.filter = prevInlineFilter;
        }
        if (owned && filter.parentNode) {
          filter.remove();
        }
      },
      fadeAndDetach(removeOverlay) {
        if (detached) return;
        overlay.classList.add("is-fading-out");
        setTimeout(() => {
          inst.detach();
          if (removeOverlay && overlay.parentNode) {
            overlay.remove();
          }
        }, 360);
      },
    };

    activeInstances.add(inst);
    ensureLoop();
    return inst;
  }

  window.ResImperiiWater = { attach, quality };

  function autoInitPagewide() {
    const stage = document.querySelector("[data-water-stage]");
    const overlay = document.querySelector(".card-water-ripple.is-pagewide");
    if (!stage || !overlay) return;

    const existingFilter = document.getElementById("water-surface-refract");
    const inst = attach(stage, overlay, {
      filter: existingFilter,
      useClass: Boolean(existingFilter),
      amplitude: DEFAULT_AMPLITUDE,
    });

    const dismissTargetId = overlay.getAttribute("data-water-dismiss-on-swap");
    if (dismissTargetId) {
      const onSwap = (ev) => {
        const target = ev.detail && ev.detail.target;
        if (target && target.id === dismissTargetId) {
          document.removeEventListener("htmx:afterSwap", onSwap);
          inst.fadeAndDetach(true);
        }
      };
      document.addEventListener("htmx:afterSwap", onSwap);
    }
  }

  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", autoInitPagewide);
  } else {
    autoInitPagewide();
  }
})();
