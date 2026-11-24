/*
 * Service worker: one job. When a page navigation fails because the device is
 * offline, show the cached /offline page ("You're offline. Reconnect...")
 * instead of the browser's own error. Nothing else is cached, and requests
 * other than GET navigations are never touched (forms, HTMX, assets, tweets).
 *
 * CACHE carries a hash of the offline page, filled in by the server
 * (routes/serviceworker.go): change the page and this file changes, so the
 * browser installs the new worker, which caches the new page and drops the old.
 */
const CACHE = "offline-__OFFLINE_HASH__";
const OFFLINE_URL = "/offline";

self.addEventListener("install", (event) => {
  event.waitUntil(
    caches.open(CACHE)
      .then((cache) => cache.add(new Request(OFFLINE_URL, { cache: "reload" })))
      .then(() => self.skipWaiting())
  );
});

self.addEventListener("activate", (event) => {
  event.waitUntil((async () => {
    // Navigation preload starts the page request while the worker boots, so
    // having a worker costs budget phones no extra wait on each navigation.
    if (self.registration.navigationPreload) {
      await self.registration.navigationPreload.enable();
    }
    const keys = await caches.keys();
    await Promise.all(keys.filter((k) => k !== CACHE).map((k) => caches.delete(k)));
    await self.clients.claim();
  })());
});

self.addEventListener("fetch", (event) => {
  const req = event.request;
  if (req.mode !== "navigate" || req.method !== "GET") return;
  event.respondWith((async () => {
    try {
      const preloaded = await event.preloadResponse;
      if (preloaded) return preloaded;
      return await fetch(req);
    } catch (_) {
      const cached = await caches.match(OFFLINE_URL, { cacheName: CACHE });
      return cached || Response.error();
    }
  })());
});
