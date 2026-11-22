/*
 * Realistic water-surface refraction for ripple loading screens.
 *
 * The page content sits "under the water": an SVG displacement filter bends it
 * radially along each expanding wavefront, using a precomputed radial refraction
 * map (the slope of a decaying wave packet). The map's position, size and strength
 * are driven every frame from the real CSS ripple rings, so the sunlit crest and the
 * optical distortion stay perfectly in sync.
 */
(() => {
  "use strict";

  const stage = document.querySelector("[data-water-stage]");
  const filter = document.getElementById("water-surface-refract");
  if (!stage || !filter) return;
  if (window.matchMedia && window.matchMedia("(prefers-reduced-motion: reduce)").matches) return;

  const waves = Array.from(
    document.querySelectorAll(".card-water-ripple.is-pagewide .card-water-wave")
  ).slice(0, 3);

  const layers = waves.map((_, i) => ({
    img: filter.querySelector("#wr-img-" + (i + 1)),
    disp: filter.querySelector("#wr-disp-" + (i + 1)),
  }));
  if (!waves.length || layers.some((l) => !l.img || !l.disp)) return;

  // Crest radius as a fraction of the ring element's diameter (matches site.css gradient: 47.4% of farthest-corner).
  const SUN_CREST = 0.335;
  // Crest radius inside the refraction map, in units of the map's half-size.
  const MAP_CREST = 0.67;
  // Peak displacement strength (px) at full wave opacity.
  const AMPLITUDE = 22;

  // Build the radial refraction map once: R/G encode the outward direction scaled by wave slope.
  function buildMapURL() {
    const N = 256;
    const canvas = document.createElement("canvas");
    canvas.width = N;
    canvas.height = N;
    const ctx = canvas.getContext("2d");
    const img = ctx.createImageData(N, N);

    const sigma = 0.12;
    const wavelength = 0.26;
    const k = (2 * Math.PI) / wavelength;
    const height = (r) => Math.exp(-((r - MAP_CREST) ** 2) / (2 * sigma * sigma)) * Math.cos(k * (r - MAP_CREST));
    const slope = (r) => {
      const e = 0.002;
      return (height(r + e) - height(r - e)) / (2 * e);
    };

    let maxSlope = 0;
    for (let r = 0; r <= 1; r += 0.002) maxSlope = Math.max(maxSlope, Math.abs(slope(r)));
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
          // Smooth taper to zero at the map edge so there is no visible seam.
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
    return canvas.toDataURL("image/png");
  }

  const mapURL = buildMapURL();
  layers.forEach(({ img, disp }) => {
    img.setAttribute("href", mapURL);
    img.setAttributeNS("http://www.w3.org/1999/xlink", "href", mapURL);
    disp.setAttribute("scale", "0");
  });

  function fitFilterRegion() {
    const r = stage.getBoundingClientRect();
    filter.setAttribute("x", "-20");
    filter.setAttribute("y", "-20");
    filter.setAttribute("width", String(Math.ceil(r.width) + 40));
    filter.setAttribute("height", String(Math.ceil(r.height) + 40));
  }
  fitFilterRegion();
  window.addEventListener("resize", fitFilterRegion);

  function hideLayer(layer) {
    layer.img.setAttribute("x", "-10");
    layer.img.setAttribute("y", "-10");
    layer.img.setAttribute("width", "1");
    layer.img.setAttribute("height", "1");
    layer.disp.setAttribute("scale", "0");
  }

  function frame() {
    const stageRect = stage.getBoundingClientRect();

    for (let i = 0; i < waves.length; i++) {
      const el = waves[i];
      const layer = layers[i];
      const cs = window.getComputedStyle(el);

      const opacity = parseFloat(cs.opacity) || 0;
      let scale = 0;
      if (cs.transform && cs.transform !== "none") {
        try {
          scale = new DOMMatrixReadOnly(cs.transform).a;
        } catch (_) {
          scale = 0;
        }
      }

      const crestRadius = scale * el.offsetWidth * SUN_CREST;
      if (crestRadius < 3 || opacity < 0.02) {
        hideLayer(layer);
        continue;
      }

      const rect = el.getBoundingClientRect();
      const cx = rect.left + rect.width / 2 - stageRect.left;
      const cy = rect.top + rect.height / 2 - stageRect.top;
      const mapHalf = crestRadius / MAP_CREST;

      layer.img.setAttribute("x", (cx - mapHalf).toFixed(1));
      layer.img.setAttribute("y", (cy - mapHalf).toFixed(1));
      layer.img.setAttribute("width", (mapHalf * 2).toFixed(1));
      layer.img.setAttribute("height", (mapHalf * 2).toFixed(1));
      layer.disp.setAttribute("scale", (AMPLITUDE * opacity).toFixed(2));
    }

    window.requestAnimationFrame(frame);
  }

  waves.forEach((_, i) => hideLayer(layers[i]));
  stage.classList.add("is-submerged");
  window.requestAnimationFrame(frame);
})();
