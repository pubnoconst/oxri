#!/usr/bin/env node
// The repeatable mobile lab run behind PROFILING.md: it loads pages in
// headless Chrome emulating a budget phone on a slow mobile network, from a
// cold cache, several times each, taps and types as a reader would, and
// reports the Core Web Vitals per page as median, spread and worst run,
// rated by Google's thresholds (HIG §1.6: the spread matters as much as the
// median).
//
// The vitals are measured by the same vendored web-vitals library the site
// sends field data with, injected before the page's own scripts, so the lab
// and the field measure alike.
//
//   node cmd/lab/lab.mjs                                   # local server, 5 runs
//   node cmd/lab/lab.mjs -base https://example.org -runs 7
//   node cmd/lab/lab.mjs -local -budget                    # the seeded site, started here
//   node cmd/lab/lab.mjs -beacon                            # the field report is complete
//   mage lab
//
// Emulation (Lighthouse's mobile defaults): 412×823 CSS px at DPR 1.75, touch,
// an Android Chrome user agent, CPU slowed 4×, 150 ms latency, 1.6 Mb/s down
// and 750 kb/s up. The site's own /vitals beacon is blocked, so lab runs never
// count as field data.
//
// It is a script, not a program that is built: on a Windows host nothing here
// is compiled, so there is no unsigned executable for the antimalware to
// judge (Principia #9, RATIONALE 091). It drives Chrome over the DevTools
// protocol through a pipe: no package, no build step, no open port. It needs
// Node 18 or later (for fetch) and a Chrome to drive. It measures as the Go
// lab it replaced did: the same flags, commands, pauses and report.

import { spawn } from "node:child_process";
import { existsSync, mkdtempSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const repoRoot = dirname(dirname(dirname(fileURLToPath(import.meta.url))));

const userAgent = "Mozilla/5.0 (Linux; Android 11; SM-A025F) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/158.0.0.0 Mobile Safari/537.36";

/**
 * One reader action after load: tap a selector, then type text into it if
 * any. Taps are on elements that do not navigate. reveal is tapped first
 * when the page has it: the link that unfolds a folded-away form.
 * @typedef {{ tap: string, typeText?: string, reveal?: string }} Step
 */

/** The pages measured by default, each with what a reader does.
 * @type {Record<string, Step[]>} */
const pages = {
  "/": [{ tap: ".landing-title" }],
  "/journals": [{ tap: ".journals-search-form input", typeText: "god" }],
  "/book-i": [{ tap: ".book-content p" }],
  // On a site with passkeys the code field is folded away behind that link
  // (the seeded site has no passkeys, and shows the field at once).
  "/login": [{ tap: "input[name=code]", typeText: "123", reveal: "summary.auth-alt-summary" }],
};

// collector records the latest value of each metric (reportAllChanges), so
// INP includes the scripted interactions without hiding the page. It also
// keeps the field report vitals.js builds as the page is left (the beacon
// itself is blocked), for -beacon to read from the next page.
const collector = `
(() => {
  window.__lab = {};
  // Every layout shift and long frame (over 50 ms), for the scroll phase.
  window.__labShifts = []; window.__labFrames = [];
  try {
    window.__labShiftSources = [];
    const describe = (n) => n && n.nodeType === 1 ? n.tagName.toLowerCase() + (n.className && typeof n.className === "string" ? "." + n.className.trim().split(/\\s+/).join(".") : "") + (n.tagName === "IMG" ? " [" + (n.getAttribute("src") || "").slice(0, 60) + " in " + ((n.parentElement && n.parentElement.className) || "") + "]" : "") : String(n && n.nodeName);
    new PerformanceObserver((l) => { for (const e of l.getEntries()) if (!e.hadRecentInput) {
      window.__labShifts.push([e.startTime, e.value]);
      for (const s of e.sources || []) window.__labShiftSources.push(Math.round(e.startTime) + "ms " + e.value.toFixed(4) + " " + describe(s.node) + " y " + Math.round(s.previousRect.y) + "→" + Math.round(s.currentRect.y) + " h " + Math.round(s.previousRect.height) + "→" + Math.round(s.currentRect.height));
    } }).observe({ type: "layout-shift", buffered: true });
    window.__labFrameSources = [];
    new PerformanceObserver((l) => { for (const e of l.getEntries()) {
      window.__labFrames.push([e.startTime, e.duration]);
      const style = e.styleAndLayoutStart ? Math.round(e.startTime + e.duration - e.styleAndLayoutStart) : 0;
      const scripts = (e.scripts || []).map((x) => (x.invoker || x.sourceFunctionName || "?") + " " + Math.round(x.duration) + "ms").join("; ");
      window.__labFrameSources.push(Math.round(e.startTime) + "ms " + Math.round(e.duration) + "ms (render " + (e.renderStart ? Math.round(e.startTime + e.duration - e.renderStart) : 0) + ", style+layout " + style + ", blocking " + Math.round(e.blockingDuration) + ") " + scripts);
    } }).observe({ type: "long-animation-frame", buffered: true });
  } catch (e) {}
  const keep = (m) => {
    window.__lab[m.name] = m.value;
    // LCP in its four parts (web-vitals attribution), and the element.
    if (m.name === "LCP" && m.attribution) {
      const a = m.attribution;
      window.__lab.lcpTTFB = a.timeToFirstByte;
      window.__lab.lcpLoadDelay = a.resourceLoadDelay;
      window.__lab.lcpLoadTime = a.resourceLoadDuration;
      window.__lab.lcpRenderDelay = a.elementRenderDelay;
      window.__labTarget = a.target || "";
    }
  };
  const opts = { reportAllChanges: true };
  webVitals.onLCP(keep, opts); webVitals.onINP(keep, opts);
  webVitals.onCLS(keep, opts); webVitals.onTTFB(keep, opts); webVitals.onFCP(keep, opts);
  const B = window.Blob;
  window.Blob = function (parts, o) {
    if (parts && typeof parts[0] === "string" && parts[0].includes('"samples"')) localStorage.setItem("lab-beacon", parts[0]);
    return new B(parts, o);
  };
})();`;

// timelineJS lists paints, resources and font faces in use (-debug).
const timelineJS = `[
  ...performance.getEntriesByType("paint").map((p) => [Math.round(p.startTime), "paint", p.name]),
  ...performance.getEntriesByType("resource").map((r) => [Math.round(r.startTime), Math.round(r.responseEnd), r.initiatorType + " " + new URL(r.name).pathname]),
].sort((a, b) => a[0] - b[0]).map((e) => e.join("  ")).join(String.fromCharCode(10)) + String.fromCharCode(10) + "fonts: " +
  [...document.fonts].filter((f) => f.status !== "unloaded").map((f) => f.family + " " + f.unicodeRange.slice(0, 8) + " " + f.status).join(", ") +
  String.fromCharCode(10) + "shifts: " + (window.__labShiftSources || []).join(String.fromCharCode(10) + "  ") +
  String.fromCharCode(10) + "long frames: " + (window.__labFrameSources || []).join(String.fromCharCode(10) + "  ")`;

// scrollJS scrolls to the bottom in steps of 80 % of the screen every
// 300 ms (at most 40 steps), waits for the page to settle, and records in
// window.__lab: scrollShift (layout shift during the scroll), scrollFrames
// (frames over 50 ms) and scrollWorstFrame (ms).
const scrollJS = `(async () => {
  const wait = (ms) => new Promise((r) => setTimeout(r, ms));
  const start = performance.now();
  for (let i = 0; i < 40; i++) {
    const before = scrollY;
    scrollBy(0, Math.round(innerHeight * 0.8));
    await wait(300);
    if (scrollY === before) break;
  }
  await wait(1200);
  const shifts = (window.__labShifts || []).filter((e) => e[0] >= start);
  const frames = (window.__labFrames || []).filter((e) => e[0] >= start);
  window.__lab.scrollShift = shifts.reduce((a, e) => a + e[1], 0);
  window.__lab.scrollFrames = frames.length;
  window.__lab.scrollWorstFrame = frames.reduce((a, e) => Math.max(a, e[1]), 0);
  return true;
})()`;

// resourcesJS summarises the page's requests: when the catalog fragment
// (/journals/content) was requested and arrived, when the last font
// arrived, and the bytes the page transferred (kB).
const resourcesJS = `(() => {
  const rs = performance.getEntriesByType("resource");
  const out = {};
  const cat = rs.find((r) => r.name.includes("/journals/content"));
  if (cat) { out.catalogStart = cat.startTime; out.catalogEnd = cat.responseEnd; }
  const fonts = rs.filter((r) => r.name.includes(".woff2"));
  if (fonts.length) out.fontsEnd = Math.max(...fonts.map((r) => r.responseEnd));
  const nav = performance.getEntriesByType("navigation")[0];
  out.kB = (rs.reduce((a, r) => a + (r.transferSize || 0), 0) + (nav ? nav.transferSize : 0)) / 1024;
  out.requests = rs.length + 1;
  // Where the server's time went (its Server-Timing header).
  if (nav && nav.serverTiming) for (const t of nav.serverTiming) out["st_" + t.name] = t.duration;
  return out;
})()`;

// The flags Chrome is launched with. They are chromedp's own
// (DefaultExecAllocatorOptions), the library the lab's Go predecessor used:
// kept in its order so the browser is the same one PROFILING.md's figures
// were taken in (RATIONALE 074–091). The profile and the DevTools pipe
// follow, then about:blank.
const chromeFlags = [
  "no-first-run", "no-default-browser-check", "headless",
  "disable-background-networking", "enable-features=NetworkService,NetworkServiceInProcess",
  "disable-background-timer-throttling", "disable-backgrounding-occluded-windows", "disable-breakpad",
  "disable-client-side-phishing-detection", "disable-default-apps", "disable-dev-shm-usage", "disable-extensions",
  "disable-features=Translate,BlinkGenPropertyTrees", "disable-hang-monitor", "disable-ipc-flooding-protection",
  "disable-popup-blocking", "disable-prompt-on-repost", "disable-renderer-backgrounding", "disable-sync",
  "force-color-profile=srgb", "metrics-recording-only", "safebrowsing-disable-auto-update", "enable-automation",
  "password-store=basic", "use-mock-keychain",
];

/** The domains the Go predecessor enabled on every page (chromedp's doing), so the page is instrumented as it was. */
const pageDomains = ["Runtime.enable", "Log.enable", "Network.enable", "Inspector.enable", "Page.enable", "DOM.enable"];

// Thresholds are Google's Core Web Vitals boundaries: at or under the first
// is good, at or under the second needs improvement, over it poor (LCP, INP
// and TTFB in ms, CLS unitless).
/** @type {Record<string, [number, number]>} */
const thresholds = { LCP: [2500, 4000], INP: [200, 500], CLS: [0.1, 0.25], TTFB: [800, 1800] };

/** @typedef {Record<string, number>} Sample one run's metrics */

/** @typedef {Record<string, string | boolean>} Flags a flag's value, or true when it stands alone */

/**
 * Reads the command line as Go's flag package does (-name value, -name=value,
 * --name).
 * @param {string[]} argv
 * @returns {Flags}
 */
function parseFlags(argv) {
  /** @type {Flags} */
  const out = {};
  for (let i = 0; i < argv.length; i++) {
    const arg = argv[i];
    if (!arg.startsWith("-")) continue;
    const body = arg.replace(/^--?/, "");
    const eq = body.indexOf("=");
    if (eq >= 0) {
      out[body.slice(0, eq)] = body.slice(eq + 1);
    } else if (i + 1 < argv.length && !argv[i + 1].startsWith("-")) {
      out[body] = argv[++i];
    } else {
      out[body] = true;
    }
  }
  return out;
}

const flags = parseFlags(process.argv.slice(2));

/** @param {string} name @param {string} fallback */
const text = (name, fallback) => (typeof flags[name] === "string" ? /** @type {string} */ (flags[name]) : fallback);
/** @param {string} name @param {number} fallback */
const number = (name, fallback) => {
  const v = Number(text(name, String(fallback)));
  return Number.isFinite(v) ? v : fallback;
};
/** @param {string} name */
const yes = (name) => flags[name] === true || (typeof flags[name] === "string" && !["0", "f", "false"].includes(/** @type {string} */ (flags[name]).toLowerCase()));

// The run's settings, as the Go lab's flags.
const baseFlag = text("base", "http://127.0.0.1:8080");
const runs = Math.trunc(number("runs", 5));
const parallel = Math.max(1, Math.trunc(number("parallel", 1)));
const cpu = number("cpu", 4);
const breakdown = yes("breakdown");
const budget = yes("budget");
const timingGate = yes("timing-gate");
const beacon = yes("beacon");
const debug = yes("debug");
const warm = flags.warm === undefined || yes("warm");
const seed = Math.trunc(number("seed", 14));
const pageSize = Math.trunc(number("page-size", 0));
const local = yes("local");
const allPages = sortedKeys(pages);
const paths = text("pages", "") ? text("pages", "").split(",") : allPages;

/** @typedef {{ base: string, vitalsJS: string, cpu: number, debug: boolean, seeded: boolean }} Config */
/** @type {Config} */
const cfg = { base: baseFlag, vitalsJS: "", cpu, debug, seeded: local };

/** @param {string} line */
function stderr(line) {
  process.stderr.write(line + "\n");
}

/** @param {number} ms */
function sleep(ms) {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

/**
 * Rejects with message if promise has not settled in ms.
 * @template T
 * @param {Promise<T>} promise
 * @param {number} ms
 * @param {string} message
 * @returns {Promise<T>}
 */
function within(promise, ms, message) {
  return new Promise((resolve, reject) => {
    const timer = setTimeout(() => reject(new Error(message + " (" + ms + " ms)")), ms);
    promise.then((v) => { clearTimeout(timer); resolve(v); }, (e) => { clearTimeout(timer); reject(e); });
  });
}

// ---- Numbers as the Go lab printed them -----------------------------------

// Splits a double into its exact mantissa and power of two (value = m × 2^e).
const float64 = new DataView(new ArrayBuffer(8));
/** @param {number} x a non-negative finite number @returns {[bigint, number]} */
function parts(x) {
  float64.setFloat64(0, x);
  const hi = float64.getUint32(0);
  const exponent = (hi >>> 20) & 0x7ff;
  const m = (BigInt(hi & 0xfffff) << 32n) | BigInt(float64.getUint32(4));
  return exponent === 0 ? [m, -1074] : [m | (1n << 52n), exponent - 1075];
}

/**
 * Formats v as the Go predecessor's "%.<prec>f" did, so a report reads on
 * from the ones before it: fixed decimals, rounded to the even digit when
 * the value lies exactly between two.
 * @param {number} v
 * @param {number} prec
 */
function fixed(v, prec) {
  if (!Number.isFinite(v)) return v !== v ? "NaN" : v > 0 ? "+Inf" : "-Inf";
  const sign = v < 0 ? "-" : "";
  const [m, e] = parts(Math.abs(v));
  let whole = "0";
  let frac = "";
  if (e >= 0) {
    whole = (m << BigInt(e)).toString();
  } else {
    const k = -e;
    const digits = (m * 5n ** BigInt(k)).toString().padStart(k + 1, "0");
    whole = digits.slice(0, digits.length - k);
    frac = digits.slice(digits.length - k);
  }
  const kept = frac.slice(0, prec).padEnd(prec, "0");
  const rest = frac.slice(prec);
  let keptNumber = BigInt(whole + kept);
  if (rest.length) {
    const first = rest.charCodeAt(0) - 48;
    const more = /[1-9]/.test(rest.slice(1));
    // Exactly between two: Go rounds to the even one.
    const tieToEven = first === 5 && !more && keptNumber % 2n === 1n;
    if (first > 5 || (first === 5 && more) || tieToEven) keptNumber += 1n;
  }
  const digits = keptNumber.toString().padStart(whole.length + prec, "0");
  const cut = digits.length - prec;
  return sign + (prec ? digits.slice(0, cut) + "." + digits.slice(cut) : digits);
}

/**
 * Formats v as Go's "%g": the shortest digits that tell the number apart.
 * @param {number} v
 */
function g(v) {
  const s = String(v);
  if (!s.includes("e")) return s;
  const [mantissa, exponent] = s.split("e");
  const n = Number(exponent);
  return mantissa + "e" + (n < 0 ? "-" : "+") + String(Math.abs(n)).padStart(2, "0");
}

/** @param {number[]} sorted @returns {number} */
function median(sorted) {
  const n = sorted.length;
  if (n % 2 === 1) return sorted[Math.floor(n / 2)];
  return Math.round(((sorted[n / 2 - 1] + sorted[n / 2]) / 2) * 1000) / 1000;
}

/** @param {string} metric @param {number} v */
function rateVital(metric, v) {
  const t = thresholds[metric];
  if (!t) return "";
  return v <= t[0] ? "good" : v <= t[1] ? "needs-improvement" : "poor";
}

/** @param {Sample[]} results @param {string} metric @returns {number[]} */
function values(results, metric) {
  const vs = [];
  for (const r of results) if (r[metric] !== undefined) vs.push(r[metric]);
  return vs.sort((a, b) => a - b);
}

/** A rated cell: median ± spread (worst) rating.
 * @param {Sample[]} results @param {string} metric */
function cell(results, metric) {
  const vs = values(results, metric);
  if (vs.length === 0) return "–";
  const f = (/** @type {number} */ v) => (metric === "CLS" ? fixed(v, 3) : fixed(v, 0));
  const unit = metric === "CLS" ? "" : " ms";
  const med = median(vs);
  return `${f(med)}${unit} ± ${f(vs[vs.length - 1] - vs[0])} (${f(vs[vs.length - 1])}) ${rateVital(metric, med) || "-"}`;
}

/** A plain cell: median ± spread (worst), with no rating.
 * @param {Sample[]} results @param {string} metric @param {number} prec @param {string} unit */
function plain(results, metric, prec, unit) {
  const vs = values(results, metric);
  if (vs.length === 0) return "–";
  const f = (/** @type {number} */ v) => fixed(v, prec) + unit;
  const fSpread = (/** @type {number} */ v) => fixed(v, prec) + unit.replace(" ms", "");
  return `${f(median(vs))} ± ${fSpread(vs[vs.length - 1] - vs[0])} (${f(vs[vs.length - 1])})`;
}

/** A page held to Google's good lines: strictly CLS on every run and that every run
 * produced its vitals; LCP and INP medians, which on a shared CI runner swing by
 * hundreds of milliseconds between runs, as timing (they fail only with
 * -timing-gate).
 * @param {string} page @param {Sample[]} results @returns {[string[], string[]]} */
function checkBudget(page, results) {
  /** @type {string[]} */
  const misses = [];
  /** @type {string[]} */
  const timing = [];
  if (results.length < runs) misses.push(`\`${page}\`: ${runs - results.length} of ${runs} runs failed`);
  const lcp = values(results, "LCP");
  if (lcp.length === 0) misses.push(`\`${page}\`: no LCP recorded`);
  else if (median(lcp) > thresholds.LCP[0]) timing.push(`\`${page}\`: median LCP ${fixed(median(lcp), 0)} ms > 2500 ms`);
  const cls = values(results, "CLS");
  if (cls.length > 0 && cls[cls.length - 1] > thresholds.CLS[0]) misses.push(`\`${page}\`: a run's CLS ${fixed(cls[cls.length - 1], 3)} > 0.1`);
  const inp = values(results, "INP");
  if (inp.length > 0 && median(inp) > thresholds.INP[0]) timing.push(`\`${page}\`: median INP ${fixed(median(inp), 0)} ms > 200 ms`);
  return [misses, timing];
}

// ---- DevTools ---------------------------------------------------------------

/**
 * A byte stream as the lab uses it (the ends of Chrome's DevTools pipe).
 * @typedef {LabNodeStream} Stream
 */

/** A message of the DevTools protocol.
 * @typedef {{ id?: number, method?: string, sessionId?: string, params?: any, result?: any, error?: { code: number, message: string } }} Message
 */

/** Speaks the DevTools protocol over Chrome's pipe: JSON, each message ended by NUL. */
class Cdp {
  /** @param {Stream} toChrome @param {Stream} fromChrome */
  constructor(toChrome, fromChrome) {
    this.toChrome = toChrome;
    this.next = 1;
    /** @type {Map<number, { resolve: (v: any) => void, reject: (e: Error) => void }>} */
    this.pending = new Map();
    /** @type {{ (message: Message): void }[]} */
    this.listeners = [];
    this.buffer = "";
    fromChrome.setEncoding("utf8");
    fromChrome.on("data", (chunk) => {
      this.buffer += chunk;
      for (let i = this.buffer.indexOf("\0"); i >= 0; i = this.buffer.indexOf("\0")) {
        const raw = this.buffer.slice(0, i);
        this.buffer = this.buffer.slice(i + 1);
        if (raw.trim()) this.dispatch(/** @type {Message} */ (JSON.parse(raw)));
      }
    });
  }

  /** @param {Message} message */
  dispatch(message) {
    if (message.id !== undefined) {
      const waiter = this.pending.get(message.id);
      if (!waiter) return;
      this.pending.delete(message.id);
      if (message.error) waiter.reject(new Error(message.error.message));
      else waiter.resolve(message.result);
      return;
    }
    for (const listener of this.listeners) listener(message);
  }

  /** @param {(message: Message) => void} listener @returns {() => void} */
  on(listener) {
    this.listeners.push(listener);
    return () => {
      const i = this.listeners.indexOf(listener);
      if (i >= 0) this.listeners.splice(i, 1);
    };
  }

  /**
   * Sends a command and resolves with its result.
   * @param {string} method
   * @param {object} [params]
   * @param {string} [sessionId]
   * @returns {Promise<any>}
   */
  send(method, params, sessionId) {
    const id = this.next++;
    return new Promise((resolve, reject) => {
      this.pending.set(id, { resolve, reject });
      /** @type {Message} */
      const message = { id, method };
      if (params) message.params = params;
      if (sessionId) message.sessionId = sessionId;
      this.toChrome.write(JSON.stringify(message) + "\0");
    });
  }
}

/**
 * A page in a fresh headless Chrome, with its own empty profile (an empty
 * cache as on a first visit; DevTools' "disable cache" is not used: it also
 * stops a preloaded resource being reused, which is not a first visit).
 * @typedef {{ send: (method: string, params?: object) => Promise<any>, on: (method: string, listener: (params: any) => void) => () => void, close(): Promise<void> }} Page
 */

/**
 * Opens a fresh Chrome and attaches to its first page.
 * @param {string} chrome
 * @returns {Promise<Page>}
 */
async function openPage(chrome) {
  const profile = mkdtempSync(join(tmpdir(), "lab-chrome-"));
  const child = spawn(chrome, [...chromeFlags.map((f) => "--" + f), "--user-data-dir=" + profile, "--remote-debugging-pipe", ...(process.env.CI ? ["--no-sandbox"] : []), "about:blank"], {
    stdio: ["ignore", "ignore", "pipe", "pipe", "pipe"],
  });
  // Chrome's own complaints, kept for the error if it does not start.
  let complaints = "";
  const errStream = /** @type {Stream} */ (child.stdio[2]);
  errStream.setEncoding("utf8");
  errStream.on("data", (chunk) => { complaints = (complaints + chunk).slice(-2000); });
  const exited = new Promise((resolve) => { child.on("exit", () => resolve(undefined)); });
  const cdp = new Cdp(/** @type {Stream} */ (child.stdio[3]), /** @type {Stream} */ (child.stdio[4]));

  /**
   * Removes Chrome and its profile, whatever state it is in. A failure here
   * must not fail a measurement that is already taken.
   * @returns {Promise<void>}
   */
  const close = async () => {
    try { child.kill(); } catch { /* already gone */ }
    // Chrome's own processes (its renderer, its GPU process) outlive it for a
    // moment and hold files in its profile; on Windows they are ended with it.
    if (process.platform === "win32" && child.pid) spawn("taskkill", ["/pid", String(child.pid), "/T", "/F"], { stdio: "ignore" });
    await within(exited, 3000, "Chrome did not exit").catch(() => { /* ended below regardless */ });
    for (let i = 0; i < 50; i++) {
      try {
        rmSync(profile, { recursive: true, force: true });
        return;
      } catch {
        await sleep(100);
      }
    }
  };
  try {
    // The first command doubles as the wait for Chrome to be up.
    const { targetInfos } = await within(Promise.race([
      cdp.send("Target.getTargets"),
      exited.then(() => { throw new Error("Chrome exited at start: " + complaints.trim()); }),
    ]), 30000, "Chrome did not start");
    const first = targetInfos.find((/** @type {{ type: string, targetId: string }} */ t) => t.type === "page");
    const targetId = first ? first.targetId : (await cdp.send("Target.createTarget", { url: "about:blank" })).targetId;
    const { sessionId } = await cdp.send("Target.attachToTarget", { targetId, flatten: true });
    /** Each command's error names the command: a DevTools error alone is only "Invalid parameters".
     * @type {Page["send"]} */
    const send = async (method, params = {}) => {
      try {
        return await cdp.send(method, params, sessionId);
      } catch (e) {
        throw new Error(`${method}: ${/** @type {Error} */ (e).message}`);
      }
    };
    for (const domain of pageDomains) await send(domain, {});
    await send("Page.setLifecycleEventsEnabled", { enabled: true });
    return {
      send,
      on: (method, listener) => cdp.on((message) => {
        if (message.sessionId === sessionId && message.method === method) listener(message.params);
      }),
      close,
    };
  } catch (e) {
    close();
    throw e;
  }
}

/** Evaluates expression in the page and returns its value as JSON brings it.
 * @param {Page} page @param {string} expression @param {boolean} [awaitPromise]
 * @returns {Promise<any>} */
async function evaluate(page, expression, awaitPromise = false) {
  const res = await page.send("Runtime.evaluate", { expression, returnByValue: true, awaitPromise });
  const details = res.exceptionDetails;
  if (details) {
    const err = details.exception && details.exception.description;
    throw new Error("evaluate: " + (err || details.text));
  }
  return res.result.value;
}

/** Resolves with the next event of this method on the page.
 * @template T
 * @param {Page} page @param {string} method
 * @returns {Promise<T>} */
function nextEvent(page, method) {
  return new Promise((resolve) => {
    const off = page.on(method, (/** @type {T} */ params) => { off(); resolve(params); });
  });
}

// Sets up the budget phone and the collector in the page, as the Go lab's
// emulate did, in its order.
/** @param {Page} page */
async function emulate(page) {
  await page.send("Network.enable", {});
  await page.send("Network.setBlockedURLs", { urlPatterns: [{ urlPattern: "*://*:*/vitals", block: true }] });
  // Every field of a condition, as the Go protocol code sent them (an empty
  // urlPattern matches every request): Chrome rejects the command without.
  await page.send("Network.emulateNetworkConditionsByRule", { matchedNetworkConditions: [{ urlPattern: "", latency: 150, downloadThroughput: 1.6e6 / 8, uploadThroughput: 750e3 / 8, packetReordering: false, offline: false }] });
  await page.send("Emulation.setDeviceMetricsOverride", { width: 412, height: 823, deviceScaleFactor: 1.75, mobile: true });
  await page.send("Emulation.setTouchEmulationEnabled", { enabled: true, maxTouchPoints: 5 });
  await page.send("Emulation.setUserAgentOverride", { userAgent });
  await page.send("Emulation.setCPUThrottlingRate", { rate: cfg.cpu });
  if (cfg.seeded) {
    // X's images go to the seeded site instead, still over the emulated
    // network (the request is redirected, not answered here). One URL per
    // image (its X path), so each is fetched as on X: a shared URL would let
    // the cache serve them all after the first.
    page.on("Fetch.requestPaused", (/** @type {{ requestId: string, request: { url: string } }} */ e) => {
      const name = e.request.url.includes("name=small") ? "small" : "large";
      const dest = cfg.base + "/__lab/image?name=" + name + "&src=" + encodeURIComponent(e.request.url);
      page.send("Fetch.continueRequest", { requestId: e.requestId, url: dest }).catch(() => { /* the page may be gone */ });
    });
    await page.send("Fetch.enable", { patterns: [{ urlPattern: "https://pbs.twimg.com/*" }] });
  }
  await page.send("Page.addScriptToEvaluateOnNewDocument", { source: cfg.vitalsJS + "\n" + collector });
}

/** Reports whether an element matching selector is in the page within ms.
 * @param {Page} page @param {string} selector @param {number} ms */
async function present(page, selector, ms) {
  const { root } = await page.send("DOM.getDocument", { depth: 0 });
  for (const end = Date.now() + ms; ; await sleep(20)) {
    const { nodeId } = await page.send("DOM.querySelector", { nodeId: root.nodeId, selector });
    if (nodeId || Date.now() >= end) return Boolean(nodeId);
  }
}

// Waits, polling every 5 ms as chromedp did, until the first element matching
// selector is visible (it has a box and a size), and returns its node id.
/** @param {Page} page @param {string} selector @returns {Promise<number>} */
async function waitVisible(page, selector) {
  const { root } = await page.send("DOM.getDocument", { depth: 0 });
  for (;;) {
    const { nodeId } = await page.send("DOM.querySelector", { nodeId: root.nodeId, selector });
    if (nodeId && (await visible(page, nodeId))) return nodeId;
    await sleep(5);
  }
}

/** @param {Page} page @param {number} nodeId */
async function visible(page, nodeId) {
  try {
    await page.send("DOM.getBoxModel", { nodeId });
  } catch (e) {
    if (String(/** @type {Error} */ (e).message).includes("Could not compute box model")) return false;
    throw e;
  }
  const { object } = await page.send("DOM.resolveNode", { nodeId });
  const res = await page.send("Runtime.callFunctionOn", {
    objectId: object.objectId,
    functionDeclaration: "function() { return !!(this.offsetWidth || this.offsetHeight || this.getClientRects().length); }",
    returnByValue: true,
  });
  return res.result.value === true;
}

/** Clicks the centre of the element, as a mouse click.
 * @param {Page} page @param {number} nodeId */
async function click(page, nodeId) {
  await page.send("DOM.scrollIntoViewIfNeeded", { nodeId });
  const { quads } = await page.send("DOM.getContentQuads", { nodeId });
  if (!quads || quads.length === 0) throw new Error("the element has no box on the page");
  const q = quads[0];
  const n = q.length / 2;
  const x = q.filter((/** @type {number} */ _, /** @type {number} */ i) => i % 2 === 0).reduce((/** @type {number} */ a, /** @type {number} */ v) => a + v, 0) / n;
  const y = q.filter((/** @type {number} */ _, /** @type {number} */ i) => i % 2 === 1).reduce((/** @type {number} */ a, /** @type {number} */ v) => a + v, 0) / n;
  for (const type of ["mousePressed", "mouseReleased"]) await page.send("Input.dispatchMouseEvent", { type, x, y, button: "left", clickCount: 1 });
}

/** Types text into the element, a key down, a character and a key up per character.
 * @param {Page} page @param {number} nodeId @param {string} textToType */
async function typeInto(page, nodeId, textToType) {
  await page.send("DOM.focus", { nodeId });
  for (const ch of textToType) {
    const upper = ch.toUpperCase();
    const code = /[a-z]/i.test(ch) ? "Key" + upper : /[0-9]/.test(ch) ? "Digit" + ch : "";
    const vk = code ? upper.charCodeAt(0) : 0;
    // A key the keyboard knows carries its code; any other character is "Unidentified".
    const common = { key: code ? ch : "Unidentified" };
    if (code) Object.assign(common, { code, windowsVirtualKeyCode: vk, nativeVirtualKeyCode: vk });
    await page.send("Input.dispatchKeyEvent", { type: "keyDown", ...common });
    await page.send("Input.dispatchKeyEvent", { type: "char", ...common, text: ch, unmodifiedText: ch });
    await page.send("Input.dispatchKeyEvent", { type: "keyUp", ...common });
  }
}

/** Does what a reader does on the page: taps, and types where there is a field.
 * @param {Page} page @param {Step} step */
async function act(page, step) {
  // A folded-away form is opened first, when the page has it.
  if (step.reveal && (await present(page, step.reveal, 2000))) {
    await click(page, await waitVisible(page, step.reveal));
    await sleep(500);
  }
  const node = await waitVisible(page, step.tap);
  await click(page, node);
  await sleep(500);
  if (step.typeText) {
    await typeInto(page, node, step.typeText);
    await sleep(500);
  }
}

/**
 * Loads path in an emulated page, settles, and does what a reader does there.
 * @param {Page} page @param {string} path
 */
async function visit(page, path) {
  const steps = pages[path] || [{ tap: "h1" }];
  await emulate(page);
  const loaded = nextEvent(page, "Page.loadEventFired");
  const nav = await page.send("Page.navigate", { url: cfg.base + path });
  if (nav.errorText) throw new Error("navigate " + path + ": " + nav.errorText);
  await loaded;
  await sleep(3000);
  for (const step of steps) {
    try {
      await within(act(page, step), 20000, "the element did not appear");
    } catch (e) {
      throw new Error(`tap ${JSON.stringify(step.tap)}: ${/** @type {Error} */ (e).message} (is the element on the page? an empty database has no catalog)`);
    }
  }
  await sleep(1000);
  // Then read to the end as a reader would: a screen at a time, with a pause
  // to look, so lazy media loads and lays out as it comes into view. What
  // moves and which frames run long during this is the stability a load-time
  // metric misses.
  await evaluate(page, scrollJS, true);
}

/** The LCP elements seen per page, for the breakdown. @type {Record<string, string[]>} */
const lcpTargets = {};

/** One cold run: the page visited, then its vitals.
 * @param {string} path @returns {Promise<Sample>} */
async function measure(path) {
  const page = await openPage(chromePath);
  try {
    return await within(visitAndRead(page, path), 90000, path + " did not finish in 90 s");
  } finally {
    await page.close();
  }
}

/** @param {Page} page @param {string} path @returns {Promise<Sample>} */
async function visitAndRead(page, path) {
  await visit(page, path);
  if (cfg.debug) {
    try {
      stderr(await evaluate(page, timelineJS));
    } catch (e) {
      stderr("timeline: " + /** @type {Error} */ (e).message);
    }
  }
  const raw = await evaluate(page, `JSON.stringify(Object.assign({}, window.__lab || {}, ${resourcesJS}))`);
  /** @type {Sample} */
  const s = JSON.parse(raw);
  if (Object.keys(s).length === 0) throw new Error("no vitals recorded");
  try {
    (lcpTargets[path] ||= []).push(await evaluate(page, `String(window.__labTarget || "")`));
  } catch { /* the element is for the breakdown only */ }
  return s;
}

/**
 * Visits path, leaves it for another page, and checks the field report
 * vitals.js built as the page was left holds all four metrics. (A report
 * sent on pagehide once carried only TTFB: Chrome fires pagehide before
 * web-vitals finalises the rest on visibilitychange.)
 * @param {string} path
 */
async function checkBeacon(path) {
  const page = await openPage(chromePath);
  try {
    return await within((async () => {
      await visitAndLeave(page, path);
      return readBeacon(page);
    })(), 90000, path + " did not finish in 90 s");
  } finally {
    await page.close();
  }
}

/** @param {Page} page @param {string} path */
async function visitAndLeave(page, path) {
  await visit(page, path);
  const leave = path === "/journals" ? "/" : "/journals";
  const loaded = nextEvent(page, "Page.loadEventFired");
  const nav = await page.send("Page.navigate", { url: cfg.base + leave });
  if (nav.errorText) throw new Error("navigate " + leave + ": " + nav.errorText);
  await loaded;
  await sleep(1000);
}

/** @param {Page} page */
async function readBeacon(page) {
  const raw = await evaluate(page, `localStorage.getItem("lab-beacon") || ""`);
  if (!raw) throw new Error("no field report was built on leaving the page");
  /** @type {{ samples?: { name: string, value: number, target?: string }[] }} */
  let report;
  try {
    report = JSON.parse(raw);
  } catch {
    throw new Error("no field report was built on leaving the page");
  }
  const have = new Set();
  /** @type {string[]} */
  const parts = [];
  for (const s of report.samples || []) {
    have.add(s.name);
    parts.push(`${s.name} ${g(s.value)}` + (s.target && s.value > 0 ? ` (${s.target})` : ""));
  }
  for (const m of ["LCP", "INP", "CLS", "TTFB"]) {
    if (!have.has(m)) throw new Error(`the report lacks ${m} (it has ${parts.join(", ")})`);
  }
  return "report has " + parts.join(", ");
}

/** A page's LCP split into its parts and the page's network shape, as medians (-breakdown).
 * @param {string} page @param {Sample[]} results */
function breakdownRow(page, results) {
  const med = (/** @type {string} */ k) => {
    const vs = values(results, k);
    return vs.length === 0 ? "–" : fixed(median(vs), 0);
  };
  const fine = (/** @type {string} */ k) => {
    const vs = values(results, k);
    return vs.length === 0 ? "–" : fixed(median(vs), 1);
  };
  /** @type {Record<string, number>} */
  const counts = {};
  let top = "";
  for (const t of lcpTargets[page] || []) {
    counts[t] = (counts[t] || 0) + 1;
    if (counts[t] > (counts[top] || 0)) top = t;
  }
  if (top === "") {
    top = "–";
  } else {
    const i = top.lastIndexOf(">");
    if (i >= 0) top = "…" + top.slice(i);
  }
  let catalog = "–";
  const s = med("catalogStart");
  const e = med("catalogEnd");
  if (s !== "–") catalog = s + " → " + e;
  return `| \`${page}\` | ${med("LCP")} | \`${top}\` | ${med("lcpTTFB")} | ${fine("st_app")} / ${fine("st_db")} / ${fine("st_tweets")} | ${med("lcpLoadDelay")} | ${med("lcpLoadTime")} | ${med("lcpRenderDelay")} | ${catalog} | ${med("fontsEnd")} | ${med("kB")} / ${med("requests")} |\n`;
}

/** Runs do for every item, at most width at a time, and returns when all are done.
 * @template T
 * @param {T[]} items @param {number} width @param {(item: T, index: number) => Promise<void>} doOne */
async function each(items, width, doOne) {
  let next = 0;
  const worker = async () => {
    while (next < items.length) {
      const index = next++;
      await doOne(items[index], index);
    }
  };
  await Promise.all(Array.from({ length: Math.min(width, items.length) }, worker));
}

/** @param {Record<string, Step[]>} m */
function sortedKeys(m) {
  return Object.keys(m).sort();
}

// ---- The seeded site (-local) ----------------------------------------------

/**
 * The seeded site's address and how many photos it served in place of X.
 * @typedef {{ url: string, images: () => Promise<{ count: number, bytes: number }>, stop(): Promise<void> }} SeededSite
 */

/**
 * Starts the seeded site for -local (cmd/labserver; what it holds is in
 * internal/labsite) and resolves to its address. On Windows the server runs
 * in WSL, where the Go toolchain is: nothing is built on the host (Principia
 * #9). It stops when the lab closes its input.
 * @returns {Promise<SeededSite>}
 */
async function startSeededSite() {
  const goArgs = `-seed ${seed} -page-size ${pageSize}`;
  const server = text("server", "") || (process.platform === "win32" ? `wsl.exe --cd "${wslPath(repoRoot)}" -- go run ./cmd/labserver ${goArgs}` : `go run ./cmd/labserver ${goArgs}`);
  const child = spawn(server, { shell: true, stdio: ["pipe", "pipe", "pipe"] });
  // The server's own complaints, kept for the error if it does not start.
  let complaints = "";
  const errStream = /** @type {Stream} */ (child.stdio[2]);
  errStream.setEncoding("utf8");
  errStream.on("data", (chunk) => { complaints = (complaints + chunk).slice(-2000); });
  const outStream = /** @type {Stream} */ (child.stdio[1]);
  outStream.setEncoding("utf8");
  let started = false;
  const exited = new Promise((resolve) => { child.on("exit", () => resolve(undefined)); });
  /**
   * Ends the server. It is told with a request, which arrives wherever the
   * server is (through WSL from Windows), and its input is closed as well, in
   * case the request did not. Whatever is left of it (the shell that started
   * it) is ended after a moment.
   * @returns {Promise<void>}
   */
  const stop = async () => {
    await fetch(url + "/__lab/stop", { method: "POST" }).catch(() => { /* it may already be gone */ });
    try { /** @type {Stream} */ (child.stdio[0]).end(); } catch { /* already closed */ }
    const timer = setTimeout(() => {
      if (child.exitCode === null) {
        if (process.platform === "win32") spawn("taskkill", ["/pid", String(child.pid), "/T", "/F"], { stdio: "ignore" });
        else child.kill("SIGKILL");
      }
    }, 5000);
    child.on("exit", () => clearTimeout(timer));
    await within(exited, 8000, "the server did not stop").catch(() => { /* nothing more to do */ });
  };
  // The first line of its output is the address.
  const url = await new Promise((resolve, reject) => {
    let out = "";
    outStream.on("data", (chunk) => {
      out += chunk;
      const i = out.indexOf("\n");
      if (i >= 0 && !started) {
        started = true;
        clearTimeout(timer);
        resolve(out.slice(0, i).trim());
      }
    });
    const timer = setTimeout(() => {
      started = true;
      reject(new Error("the seeded site's server did not start in 4 minutes: " + complaints.trim()));
      if (process.platform === "win32") spawn("taskkill", ["/pid", String(child.pid), "/T", "/F"], { stdio: "ignore" });
      else child.kill("SIGKILL");
    }, 240000);
    child.on("exit", () => {
      clearTimeout(timer);
      if (!started) {
        started = true;
        reject(new Error("the seeded site's server exited: " + complaints.trim()));
      }
    });
  });
  // The address is printed the moment the server listens. Between Windows and
  // WSL a moment passes before a new port is reachable from the other side,
  // so wait until it answers before anything is measured.
  for (const begin = Date.now(); ; await sleep(100)) {
    try {
      const res = await fetch(url + "/healthz");
      await res.arrayBuffer();
      if (res.ok) break;
    } catch { /* not answering yet */ }
    if (Date.now() - begin > 60000) throw new Error("the seeded site's server at " + url + " did not answer: " + complaints.trim());
  }
  return {
    url,
    images: async () => {
      const res = await fetch(url + "/__lab/stats");
      return await res.json();
    },
    stop,
  };
}

/** A Windows path as WSL names it: C:\x is /mnt/c/x, and a path behind \\wsl$\Distro or \\wsl.localhost\Distro is the Linux one.
 * @param {string} p */
function wslPath(p) {
  const unc = /^\\\\wsl(?:\$|\.localhost)\\([^\\]+)(\\.*)?$/.exec(p);
  if (unc) return (unc[2] || "\\").replace(/\\/g, "/");
  const drive = /^([A-Za-z]):\\(.*)$/.exec(p);
  if (drive) return "/mnt/" + drive[1].toLowerCase() + "/" + drive[2].replace(/\\/g, "/");
  return p.replace(/\\/g, "/");
}

// ---- Chrome ----------------------------------------------------------------

/**
 * Finds the Chrome to drive: -chrome, then CHROME_PATH, then where it is
 * installed on this system.
 * @returns {string}
 */
function findChrome() {
  const given = text("chrome", "") || process.env.CHROME_PATH || "";
  if (given) return given;
  /** @type {string[]} */
  let candidates = [];
  if (process.platform === "win32") {
    const roots = [process.env.ProgramFiles || "C:\\Program Files", process.env["ProgramFiles(x86)"] || "C:\\Program Files (x86)", process.env.LOCALAPPDATA || ""].filter(Boolean);
    candidates = roots.map((root) => join(root, "Google", "Chrome", "Application", "chrome.exe"));
  } else if (process.platform === "darwin") {
    candidates = ["/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"];
  } else {
    for (const name of ["google-chrome", "google-chrome-stable", "chromium", "chromium-browser"]) {
      for (const dir of (process.env.PATH || "").split(":")) {
        const candidate = join(dir || ".", name);
        if (existsSync(candidate)) candidates.push(candidate);
      }
    }
  }
  const found = candidates.find((c) => existsSync(c));
  if (!found) throw new Error("no Chrome found; pass -chrome <path to chrome>");
  return found;
}

/** @type {string} */
let chromePath = "";

// ---- Main ------------------------------------------------------------------

/** @param {string} message */
function fail(message) {
  stderr("lab: " + message);
  process.exit(1);
}

/** @returns {string} now, as "2026-11-28 06:50" */
function stamp() {
  const d = new Date();
  const pad = (/** @type {number} */ n) => String(n).padStart(2, "0");
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}`;
}

async function main() {
  cfg.vitalsJS = readFileSync(text("lib", join(repoRoot, "assets", "js", "web-vitals.min.js")), "utf8");
  chromePath = findChrome();
  /** @type {SeededSite | undefined} */
  let site;
  if (local) {
    site = await startSeededSite();
    cfg.base = site.url;
    stderr("lab: serving the seeded site at " + site.url);
  }
  try {
    if (beacon) {
      /** @type {(string | Error)[]} */
      const results = Array.from({ length: paths.length });
      await each(paths, parallel, async (p, i) => {
        try {
          results[i] = await checkBeacon(p);
        } catch (e) {
          results[i] = e instanceof Error ? e : new Error(String(e));
        }
      });
      let failed = false;
      paths.forEach((p, i) => {
        const r = results[i];
        if (r instanceof Error) {
          failed = true;
          process.stdout.write(`${p}: ${r.message}\n`);
        } else {
          process.stdout.write(`${p}: ${r}\n`);
        }
      });
      if (failed) process.exitCode = 1;
      return;
    }

    if (warm) {
      // Request each page once first, so the server's own cold start is not
      // counted against the page (field data covers the server).
      for (const p of paths) await fetch(cfg.base + p).then((r) => r.arrayBuffer()).catch(() => { /* warming is best effort */ });
    }

    let report = `# Lab run ${stamp()}\n\n`;
    report += `\`${cfg.base}\`, ${runs} cold-cache runs per page; budget phone: 412×823 @1.75, CPU ${g(cpu)}× slower, 150 ms RTT, 1.6 Mb/s down. Median / spread (max − min) / worst. Good: LCP ≤ 2.5 s, INP ≤ 200 ms, CLS ≤ 0.1.\n\n`;
    report += "| Page | LCP | INP | CLS | TTFB | FCP | Shift while scrolling | Long frames while scrolling | Worst frame |\n|---|---|---|---|---|---|---|---|---|---|\n";
    /** @type {string[]} */
    const misses = [];
    /** @type {string[]} */
    const advisories = [];
    /** @type {Sample[][]} */
    const perPage = paths.map(() => []);
    await each(paths, parallel, async (p, n) => {
      for (let i = 0; i < runs; i++) {
        let s;
        try {
          s = await measure(p);
        } catch (e) {
          stderr(`${p} run ${i + 1}: ${/** @type {Error} */ (e).message}`);
          continue;
        }
        perPage[n].push(s);
        stderr(`${p} run ${i + 1}: LCP ${fixed(s.LCP ?? 0, 0)}  INP ${fixed(s.INP ?? 0, 0)}  CLS ${fixed(s.CLS ?? 0, 3)}  TTFB ${fixed(s.TTFB ?? 0, 0)} (server ${fixed(s.st_app ?? 0, 1)})  scroll: shift ${fixed(s.scrollShift ?? 0, 3)}, ${fixed(s.scrollFrames ?? 0, 0)} long frames, worst ${fixed(s.scrollWorstFrame ?? 0, 0)} ms`);
      }
    });
    paths.forEach((p, n) => {
      const results = perPage[n];
      report += `| \`${p}\` | ${cell(results, "LCP")} | ${cell(results, "INP")} | ${cell(results, "CLS")} | ${cell(results, "TTFB")} | ${cell(results, "FCP")} | ${plain(results, "scrollShift", 3, "")} | ${plain(results, "scrollFrames", 0, "")} | ${plain(results, "scrollWorstFrame", 0, " ms")} |\n`;
      const [strict, timing] = checkBudget(p, results);
      misses.push(...strict);
      (timingGate ? misses : advisories).push(...timing);
    });
    report += "\nA cell reads *median ± spread (worst)* and the median's rating. `–` is a metric the page never produced (no interaction, or no shift).\n";

    if (breakdown) {
      report += "\n### Where the LCP time goes (medians, ms)\n\n";
      report += "LCP = server (TTFB) + wait before the LCP element's resource is requested + its download + render delay (web-vitals attribution). For a text element there is no resource: the wait for the text to be drawn (its data, fonts, scripts) is all render delay.\n\n";
      report += "| Page | LCP | LCP element | TTFB | Server (app / db / tweets) | Resource wait | Resource load | Render delay | Catalog request (start → end) | Last font | kB / requests |\n|---|---|---|---|---|---|---|---|---|---|---|\n";
      paths.forEach((p, n) => { report += breakdownRow(p, perPage[n]); });
    }

    if (budget) {
      if (misses.length === 0) {
        report += "\n**Budget: met**: no page shifts (every run's CLS ≤ 0.1) and every run produced its vitals.\n";
      } else {
        report += "\n**Budget: missed**\n\n";
        for (const m of misses) report += "- " + m + "\n";
      }
      if (advisories.length > 0) {
        report += "\n**Over Google's good line on this machine** (reported, not failed: timing varies with the runner):\n\n";
        for (const m of advisories) report += "- " + m + "\n";
      }
    }

    if (site) {
      const { count, bytes } = await site.images();
      if (count > 0) report += `\nImages served by the lab (as from X): ${count}, ${fixed(bytes / 1024, 0)} kB in all, ${fixed(bytes / 1024 / count, 0)} kB each on average (over all runs).\n`;
    }

    process.stdout.write(report);
    if (out) {
      try {
        writeFileSync(out, report);
      } catch (e) {
        fail(`write ${out}: ${/** @type {Error} */ (e).message}`);
      }
    }
    if (budget && misses.length > 0) process.exitCode = 1;
  } finally {
    if (site) await site.stop();
  }
}

/** The path to also write the report to. */
const out = text("out", "");

main().catch((e) => fail(e instanceof Error ? e.message : String(e)));
