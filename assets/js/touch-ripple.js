/*
 * Touch ripple: the site's "your press was received" language.
 *
 * Pressing a link or button drops a drop at the touch point. The page (linen
 * under water) refracts as the wave spreads; nothing is painted. The wave
 * reflects off the window edges like a real wave off the walls of a basin.
 *
 * Physics: a wave reflecting off a straight wall is indistinguishable from a
 * wave emitted at the same instant by a mirror-image source behind that wall.
 * So the bounce is four extra sources mirrored across the left/right/top/bottom
 * edges, emitted with the drop at half strength (one bounce, damped). All waves
 * share one constant speed, so echoes arrive exactly when they should.
 *
 * Navigation is never delayed. If the next page is slow to arrive (the page is
 * marked .navigation-leaving by navigation-feedback.js), softer drops keep
 * falling from the same point until it does, so latency reads as "working",
 * never as a dead click. Form submits that raise the "Confirming" overlay hand
 * off to it. Reduced motion: no ripple (navigation-feedback keeps a static dim).
 */
(() => {
  "use strict";

  const TRIGGER = 'a[href], button, input[type="submit"], input[type="button"], input[type="reset"], [role="button"], summary';
  const SPEED_PX_PER_MS = 2.1;  // crest speed; a full-HD width in ~0.9 s
  const REACH_OF_DIAGONAL = 0.8; // how far the wave travels before it is spent
  const MIN_MS = 500;
  const MAX_MS = 1100;
  const ECHO_GAIN = 0.5;        // energy kept per reflection
  const PULSE_GAIN = 0.6;       // "still working" drops are softer than the first
  const PULSE_GAP_MS = 180;     // calm pause between latency pulses
  const PULSE_GIVE_UP_MS = 35000; // matches navigation-feedback's reset
  const AMPLITUDE = 7;
  const CREST = 0.335;          // crest radius / ring diameter (water-ripple.js SUN_CREST)

  const reducedMotion = () =>
    window.matchMedia && window.matchMedia("(prefers-reduced-motion: reduce)").matches;

  let active = null;     // { overlay, water, timer }
  let pulseUntil = 0;
  let lastOrigin = null;

  function stop() {
    if (!active) return;
    clearTimeout(active.timer);
    if (active.endMeasure) active.endMeasure(false); // no-op if already reported
    if (active.water) active.water.detach();
    if (active.overlay.parentNode) active.overlay.remove();
    active = null;
  }

  function drop(x, y, gain) {
    if (reducedMotion() || !window.ResImperiiWater) return;
    // Device quality (water-ripple.js governor): 3 every echo, 2 nearest echo,
    // 1 drop only, 0 off (navigation-feedback's static cue applies instead).
    const level = window.ResImperiiWater.quality.get();
    if (level === 0) return;
    stop();

    const W = window.innerWidth;
    const H = window.innerHeight;

    // Reach is bounded to what is visible: 0.8x the window diagonal. Beyond
    // that a real ripple has spent its energy, and rendering cost grows with
    // the ring's size (the refraction map scales with it), so unbounded rings
    // can stall the renderer on heavy pages.
    const radius = Math.min(Math.hypot(W, H) * REACH_OF_DIAGONAL, MAX_MS * SPEED_PX_PER_MS);
    const duration = Math.max(MIN_MS, radius / SPEED_PX_PER_MS);
    const diameter = Math.ceil(radius / CREST);

    // An echo is born when the drop's crest reaches that wall, i.e. at radius
    // = distance to the wall. Walls the wave can't reach within its run
    // never echo (and cost nothing).
    const walls = [
      { d: x, s: { x: -x, y } },             // left
      { d: W - x, s: { x: 2 * W - x, y } },  // right
      { d: y, s: { x, y: -y } },             // top
      { d: H - y, s: { x, y: 2 * H - y } },  // bottom
    ];
    const reachable = walls.filter((w) => w.d < radius * 0.85).sort((a, b) => a.d - b.d);
    const echoes = level >= 3 ? reachable : level === 2 ? reachable.slice(0, 1) : [];
    const sources = [{ x, y, g: gain }, ...echoes.map((w) => ({ ...w.s, g: gain * ECHO_GAIN }))];

    const overlay = document.createElement("div");
    overlay.className = "touch-ripple";
    overlay.setAttribute("aria-hidden", "true");
    overlay.style.setProperty("--touch-ms", `${Math.round(duration)}ms`);
    overlay.style.setProperty("--touch-diameter", `${diameter}px`);
    for (const s of sources) {
      const wave = document.createElement("span");
      wave.className = "card-water-wave";
      wave.style.left = `${s.x.toFixed(1)}px`;
      wave.style.top = `${s.y.toFixed(1)}px`;
      wave.style.setProperty("--gain", s.g.toFixed(3));
      overlay.append(wave);
    }
    document.body.append(overlay);

    // Refract the page's cloth (a fixed linen layer behind all content), never
    // the content itself: see water-ripple.js getCloth / RATIONALE 039.
    const water = window.ResImperiiWater.attach(window.ResImperiiWater.cloth(), overlay, {
      amplitude: AMPLITUDE,
      maxWaves: sources.length,
    });

    // Report how smoothly this run drew, so the governor can adapt.
    const endMeasure = window.ResImperiiWater.quality.measure();

    active = { overlay, water, timer: 0, endMeasure };
    active.timer = setTimeout(() => {
      endMeasure();
      stop();
      // Latency event: still on this page while a navigation is in flight.
      const leaving = document.documentElement.classList.contains("navigation-leaving");
      if (leaving && Date.now() < pulseUntil && lastOrigin) {
        setTimeout(() => {
          if (document.documentElement.classList.contains("navigation-leaving") && Date.now() < pulseUntil) {
            drop(lastOrigin.x, lastOrigin.y, PULSE_GAIN);
          }
        }, PULSE_GAP_MS);
      }
    }, duration + 40);
  }

  function start(x, y) {
    lastOrigin = { x, y };
    pulseUntil = Date.now() + PULSE_GIVE_UP_MS;
    drop(x, y, 1);
  }

  function triggerFrom(target) {
    const el = target instanceof Element ? target.closest(TRIGGER) : null;
    if (!el) return null;
    if (el.matches(":disabled") || el.getAttribute("aria-disabled") === "true") return null;
    return el;
  }

  // Pointer: acknowledge on press, in the same frame as the touch.
  document.addEventListener("pointerdown", (event) => {
    if (!event.isPrimary || event.button !== 0) return;
    if (!triggerFrom(event.target)) return;
    start(event.clientX, event.clientY);
  }, { passive: true });

  // Keyboard activation: the drop falls from the centre of the control.
  document.addEventListener("keydown", (event) => {
    if (event.repeat || (event.key !== "Enter" && event.key !== " ")) return;
    const el = triggerFrom(event.target);
    if (!el || el !== event.target) return;
    if (event.key === " " && el.matches("a[href]")) return; // Space scrolls, not activates, on links
    const r = el.getBoundingClientRect();
    start(r.left + r.width / 2, r.top + r.height / 2);
  });

  // Hand off to the full-screen "Confirming" overlay; it has its own water.
  document.addEventListener("submit", (event) => {
    const form = event.target instanceof HTMLFormElement ? event.target : null;
    if (!form || event.defaultPrevented) return;
    if (form.getAttribute("data-confirm-loading") === "true") {
      pulseUntil = 0;
      stop();
    }
  });

  // The moment a navigation commits, the water stops: the outgoing page is
  // snapshotted for the view transition (pageswap) and torn down (pagehide)
  // with no filter running.
  const stillWater = () => {
    pulseUntil = 0;
    stop();
  };
  window.addEventListener("pageswap", stillWater);
  window.addEventListener("pagehide", stillWater);

  // Back/forward cache restore: the page must come back still.
  window.addEventListener("pageshow", (event) => {
    if (event.persisted) {
      pulseUntil = 0;
      stop();
    }
  });
})();
