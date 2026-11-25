/**
 * Field performance (PROFILING.md, HIG §1.6): the Core Web Vitals this page
 * view had in this reader's browser, measured by Google's web-vitals library
 * (web-vitals.min.js), sent once as the page is left (routes/vitals.go).
 *
 * Sent: the path, mobile or desktop, the effective connection type, the
 * device memory class, and per metric its value, rating and the selector of
 * the element the browser attributed it to. Nothing else: no identifier, and
 * the server keeps no address.
 */
(() => {
  if (!window.webVitals || !navigator.sendBeacon) return;

  /** @type {Map<string, {name: string, value: number, rating: string, target: string}>} */
  const samples = new Map();
  let sent = false;

  /** @param {WebVitalsMetric} metric */
  const keep = (metric) => {
    const a = metric.attribution || {};
    const target = a.target || a.interactionTarget || a.largestShiftTarget || "";
    samples.set(metric.name, {
      name: metric.name,
      value: metric.name === "CLS" ? Math.round(metric.value * 10000) / 10000 : Math.round(metric.value),
      rating: metric.rating,
      target: String(target).slice(0, 100),
    });
  };

  const send = () => {
    if (sent || samples.size === 0) return;
    sent = true;
    const conn = /** @type {{effectiveType?: string} | undefined} */ (/** @type {any} */ (navigator).connection);
    const memory = /** @type {number | undefined} */ (/** @type {any} */ (navigator).deviceMemory);
    // Mobile: the browser's own hint (Chromium), else a touch pointer.
    const uaData = /** @type {{mobile?: boolean} | undefined} */ (/** @type {any} */ (navigator).userAgentData);
    const mobile = (uaData && uaData.mobile) || matchMedia("(pointer: coarse)").matches;
    const report = {
      page: location.pathname,
      device: mobile ? "mobile" : "desktop",
      network: (conn && conn.effectiveType) || "",
      memory: typeof memory === "number" ? memory : 0,
      samples: [...samples.values()],
    };
    navigator.sendBeacon("/vitals", new Blob([JSON.stringify(report)], { type: "application/json" }));
  };

  window.webVitals.onLCP(keep);
  window.webVitals.onINP(keep);
  window.webVitals.onCLS(keep);
  window.webVitals.onTTFB(keep);

  // web-vitals finalises LCP, INP and CLS when the page becomes hidden
  // (leaving it, switching tab, locking the phone), in its own listener,
  // which runs before this one. Not on pagehide: when a link is followed,
  // Chrome fires pagehide first, and a report sent then carried only TTFB.
  addEventListener("visibilitychange", () => {
    if (document.visibilityState === "hidden") send();
  });
})();
