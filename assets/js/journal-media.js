/*
 * Shared tactile media renderer for Res Imperii journals.
 * Supports YouTube, Vimeo, X / Twitter posts, images, audio, and video.
 */
(() => {
  function safeUrl(value) {
    if (!value) return null;
    try {
      const url = new URL(value.trim());
      return url.protocol === "https:" || url.protocol === "http:" ? url : null;
    } catch {
      return null;
    }
  }

  function getActiveTheme() {
    const override = document.documentElement.getAttribute("data-theme") || sessionStorage.getItem("theme-override");
    if (override === "dark" || override === "light") return override;
    return window.matchMedia && window.matchMedia("(prefers-color-scheme: dark)").matches ? "dark" : "light";
  }

  // Tweet load sequence on a journal card:
  //   1. Turso done -> card text is shown with a quiet media space where the
  //      card-level water ripple plays (the client is visibly waiting).
  //   2. The tweet renders off-screen (invisible stage). The moment Twitter
  //      reports a real height, the space animates to exactly that height,
  //      pushing the card body down fluidly. If Twitter resizes again (it
  //      lays embeds out in two passes) the same transition simply retargets.
  //   3. Once the push has finished, the tweet eases in and the ripple fades.
  // The tweet itself is never shown half-laid-out, and nothing waits on a
  // fixed "settle" timer.
  // dotLoading builds the shared phone loader: a 3x3 dot matrix and a label
  // (site.css .dot-loading; the Go twin is views/loading.Dots).
  function dotLoading(label) {
    const wrap = document.createElement("span");
    wrap.className = "dot-loading";
    const matrix = document.createElement("span");
    matrix.className = "dot-matrix";
    matrix.setAttribute("aria-hidden", "true");
    for (let i = 0; i < 9; i++) matrix.append(document.createElement("i"));
    const text = document.createElement("span");
    text.className = "dot-loading-label";
    text.textContent = label;
    wrap.append(matrix, text);
    return wrap;
  }

  const isPhoneUI = () =>
    window.ResImperiiWater ? window.ResImperiiWater.isPhoneUI() : !!(window.matchMedia && window.matchMedia("(pointer: coarse)").matches);

  // On phones the waiting cue is one quiet line where the tweet will open, no
  // water. When the tweet arrives, clearPending removes the line inside the
  // FLIP's layout step, so the slot unrolls from the line to the tweet.
  function showLineLoader(container) {
    container.classList.add("is-loading-line");
    container.setAttribute("role", "status");
    container.append(dotLoading("Loading preview"));
    return {
      fadeOut() {},
      cleanup() {
        const line = container.querySelector(":scope > .dot-loading");
        if (line) line.remove();
        container.classList.remove("is-loading-line");
      },
    };
  }

  function showRippleLoader(container) {
    container.style.height = "";
    container.replaceChildren();
    if (isPhoneUI()) return showLineLoader(container);

    // Nothing is rendered or reserved while the tweet loads: the slot is
    // collapsed (is-pending: no height, no gap) and only the card's own content
    // shows, under the card ripple. When the tweet arrives, flipPush opens the
    // slot and pushes the content down to make room.
    container.classList.add("is-pending");
    container.setAttribute("role", "status");
    container.setAttribute("aria-label", "Loading media");

    const overlay = document.createElement("div");
    overlay.className = "card-water-ripple";
    overlay.setAttribute("aria-hidden", "true");

    const wave1 = document.createElement("span");
    wave1.className = "card-water-wave wave-1";
    const wave2 = document.createElement("span");
    wave2.className = "card-water-wave wave-2";
    const wave3 = document.createElement("span");
    wave3.className = "card-water-wave wave-3";

    overlay.append(wave1, wave2, wave3);

    // The overlay and refraction filter belong on the whole enclosing
    // .journal-card, so the wavefront disturbs the meta, tags and text too.
    // renderTweet() runs inside createMedia(), *before* the caller appends the
    // container to the card, so closest() would find nothing and the ripple
    // would be trapped inside the tweet slot. Mount on a microtask instead:
    // by then the synchronous append has happened.
    let hostCard = null;
    let waterHandle = null;
    let cardObserver = null;
    let cleaned = false;

    // The drop lands where the tweet will open: centred on the collapsed slot,
    // at its top edge.
    const alignOrigin = () => {
      if (!hostCard || !container.isConnected || !hostCard.isConnected) return;
      const hostRect = hostCard.getBoundingClientRect();
      const slotRect = container.getBoundingClientRect();
      if (hostRect.width <= 0 || hostRect.height <= 0) return;

      const x = (slotRect.left - hostRect.left) + (slotRect.width / 2);
      const y = Math.min(Math.max(slotRect.top - hostRect.top, 0), hostRect.height);
      const maxDist = Math.max(
        Math.hypot(x, y),
        Math.hypot(hostRect.width - x, y),
        Math.hypot(x, hostRect.height - y),
        Math.hypot(hostRect.width - x, hostRect.height - y)
      );
      overlay.style.setProperty("--ripple-x", `${x.toFixed(1)}px`);
      overlay.style.setProperty("--ripple-y", `${y.toFixed(1)}px`);
      overlay.style.setProperty("--ripple-diameter", `${Math.ceil(maxDist * 2.25)}px`);
    };

    const mount = () => {
      if (cleaned) return;
      hostCard = container.closest(".journal-card") || container;
      const existingOverlay = hostCard.querySelector(":scope > .card-water-ripple");
      if (existingOverlay) existingOverlay.remove();
      hostCard.append(overlay);

      requestAnimationFrame(alignOrigin);
      // Keep origin and reach correct if the card reflows (fonts, neighbours, resize).
      if (typeof ResizeObserver !== "undefined") {
        cardObserver = new ResizeObserver(alignOrigin);
        cardObserver.observe(hostCard);
      }

      waterHandle = window.ResImperiiWater
        ? window.ResImperiiWater.attach(hostCard, overlay, { amplitude: 7 })
        : null;
    };
    queueMicrotask(mount);

    return {
      fadeOut() {
        overlay.classList.add("is-fading-out");
      },
      cleanup() {
        if (cleaned) return;
        cleaned = true;
        if (cardObserver) cardObserver.disconnect();
        if (waterHandle) waterHandle.detach();
        if (overlay.parentNode) overlay.remove();
      },
    };
  }

  // clearPending ends the collapsed loading state of a tweet slot.
  function clearPending(container) {
    const line = container.querySelector(":scope > .dot-loading");
    if (line) line.remove();
    container.classList.remove("is-pending", "is-loading-line");
    container.removeAttribute("role");
    container.removeAttribute("aria-label");
  }

  function showTweetFallback(container, username, tweetId, rippleHandle) {
    if (rippleHandle) rippleHandle.cleanup();
    clearPending(container);
    container.classList.remove("is-revealed");
    container.style.height = "";
    container.replaceChildren();
    const card = document.createElement("div");
    card.className = "tweet-fallback-card";

    const badge = document.createElement("span");
    badge.className = "tweet-fallback-badge";
    badge.textContent = "X / Twitter Post";

    const link = document.createElement("a");
    link.href = `https://x.com/${encodeURIComponent(username)}/status/${encodeURIComponent(tweetId)}`;
    link.target = "_blank";
    link.rel = "noopener noreferrer";
    link.textContent = username !== "post" ? `@${username} on X (status ${tweetId})` : `Post ${tweetId} on X`;

    card.append(badge, link);
    container.append(card);
    return card;
  }

  let twitterScriptInjected = false;
  function ensureTwitterWidgetsScript() {
    if (twitterScriptInjected || (window.twttr && window.twttr.widgets)) return;
    if (document.querySelector('script[src*="platform.twitter.com/widgets.js"]')) {
      twitterScriptInjected = true;
      return;
    }
    twitterScriptInjected = true;
    const s = document.createElement("script");
    s.src = "https://platform.twitter.com/widgets.js";
    s.charset = "utf-8";
    s.async = true;
    document.head.appendChild(s);
  }

  // What Twitter actually does (measured via its postMessage protocol):
  //   initialized -> results -> resize(h) + rendered   <- createTweet() resolves here,
  //                                                       iframe height is already exact
  //   ...later, tweets with photos/video send another resize(h') once media loads.
  // So there is no "settle" to wait for. As soon as a real height exists we push
  // the card body down to it and ease the tweet in; every later resize (Twitter's
  // second phase, or a viewport-width change) is pushed the same way.
  //
  // The push is FLIP (First, Last, Invert, Play), not an animated height: the
  // slot takes its final height in ONE layout, then everything that moved is
  // offset back with transform and animated home, and every card that grew is
  // clipped back to its old height and unrolled. Transforms run on the GPU and
  // the clip only repaints, so no frame re-lays-out the catalog (an animated
  // height did, ~every frame for every revealing tweet: choppy on phones).
  const MIN_REAL_HEIGHT = 40; // below this Twitter hasn't laid the tweet out yet
  const PUSH_MS = 460;
  const PUSH_EASING = "cubic-bezier(0.22, 1, 0.36, 1)";
  const flipAnimations = new WeakMap(); // element -> its running FLIP animation

  // flipPush applies change() (a single layout change at the media slot) and
  // animates everything it moved or grew from where it visually was.
  function flipPush(container, change) {
    const slot = container.closest(".journal-media-slot") || container;
    const host = container.closest(".journal-card") || slot.parentElement;
    const peers = host && host.parentElement ? Array.from(host.parentElement.children) : [];
    const inner = [];
    for (let n = slot.nextElementSibling; n; n = n.nextElementSibling) inner.push(n);
    // The slot itself is measured too: when it opens, it unrolls in step with
    // the content it pushes, so the tweet's visible bottom edge always meets
    // the moving content (no overlap, no empty box).
    const els = peers.concat(inner, [container]).filter((el) => el instanceof HTMLElement);

    // First: where things are now, including any in-flight FLIP offset.
    const first = new Map(els.map((el) => [el, el.getBoundingClientRect()]));

    change();

    const reduce = window.matchMedia && window.matchMedia("(prefers-reduced-motion: reduce)").matches;
    if (reduce || typeof Element.prototype.animate !== "function") return;

    // Last: the settled layout (cancel running offsets so we measure layout).
    els.forEach((el) => {
      const running = flipAnimations.get(el);
      if (running) running.cancel();
    });
    els.forEach((el) => {
      const a = first.get(el);
      const b = el.getBoundingClientRect();
      const isSlot = el === container;
      const dy = isSlot ? 0 : a.top - b.top; // the slot's own top does not move
      const grew = isSlot || peers.includes(el) ? b.height - a.height : 0;
      if (Math.abs(dy) < 0.5 && grew < 0.5) return;
      // Clip only the bottom edge while a card or the slot unrolls; keep side
      // and top shadows.
      const from = isSlot ? {} : { transform: `translateY(${dy.toFixed(1)}px)` };
      const to = isSlot ? {} : { transform: "translateY(0)" };
      if (grew >= 0.5) {
        from.clipPath = `inset(-60px -60px ${grew.toFixed(1)}px -60px)`;
        to.clipPath = "inset(-60px -60px 0px -60px)";
      }
      const anim = el.animate([from, to], { duration: PUSH_MS, easing: PUSH_EASING });
      flipAnimations.set(el, anim);
      anim.onfinish = anim.oncancel = () => {
        if (flipAnimations.get(el) === anim) flipAnimations.delete(el);
      };
    });
  }
  const TWEET_GIVE_UP_MS = 12000; // embed blocked/never rendered -> show the link card

  function expandAndRevealTweet(container, rippleHandle, stage) {
    let target = 0;
    let revealed = false;
    let stopped = false;
    let observer = null;

    // Measure Twitter's iframe itself, never the stage/container: once the stage
    // is in flow it takes the container's height, which would feed back into
    // itself. offsetHeight ignores the stage's scale-in transform.
    let observedFrame = null;
    const measure = () => {
      const frame = stage.querySelector("iframe");
      if (!frame) return 0;
      if (observer && frame !== observedFrame) {
        observer.observe(frame);
        observedFrame = frame;
      }
      const cs = window.getComputedStyle(stage);
      const padY = (parseFloat(cs.paddingTop) || 0) + (parseFloat(cs.paddingBottom) || 0);
      return Math.ceil(frame.offsetHeight + padY);
    };

    const stop = () => {
      stopped = true;
      if (observer) observer.disconnect();
    };

    const reveal = () => {
      if (revealed) return;
      revealed = true;
      stage.classList.remove("is-staging");
      if (rippleHandle) {
        rippleHandle.fadeOut();
        setTimeout(() => rippleHandle.cleanup(), 420);
      }
    };

    const onHeight = () => {
      if (stopped) return;
      if (!container.isConnected) {
        stop();
        if (rippleHandle) rippleHandle.cleanup();
        return;
      }
      const h = measure();
      if (h < MIN_REAL_HEIGHT || h === target) return;

      const firstTime = target === 0;
      target = h;
      // One layout change at the slot; flipPush animates the consequences.
      flipPush(container, () => {
        if (firstTime) {
          // Open the collapsed slot for the tweet; from here the slot height is
          // driven explicitly (overflow clipped) and follows Twitter's resizes.
          clearPending(container);
          container.classList.add("is-revealed");
        }
        container.style.height = `${h}px`;
      });
      reveal();
    };

    if (typeof ResizeObserver !== "undefined") {
      observer = new ResizeObserver(onHeight);
      observer.observe(stage);
    }

    return {
      // createTweet() resolved: Twitter has already applied the exact height.
      rendered() {
        onHeight();
        if (target === 0 && !stopped) {
          // Nothing measurable (no ResizeObserver, or an odd embed):
          // fall back to plain layout rather than keep the reader waiting.
          stop();
          flipPush(container, () => {
            clearPending(container);
            container.style.height = "";
          });
          reveal();
        }
      },
      abort: stop,
    };
  }

  function renderTweet(container, username, tweetId) {
    ensureTwitterWidgetsScript();
    const theme = getActiveTheme();

    // Card text is visible; the media space shows the ripple while we wait.
    const rippleHandle = showRippleLoader(container);

    const doCreate = (twttr) => {
      const stage = document.createElement("div");
      stage.className = "tweet-embed-stage is-staging";
      container.append(stage);

      // Start tracking before Twitter renders so the very first height is caught.
      const tracker = expandAndRevealTweet(container, rippleHandle, stage);

      let settled = false;
      const fail = () => {
        if (settled) return;
        settled = true;
        clearTimeout(giveUp);
        // Already on screen (the height tracker revealed it): never replace it.
        if (!stage.classList.contains("is-staging")) return;
        tracker.abort();
        stage.remove();
        showTweetFallback(container, username, tweetId, rippleHandle);
      };

      // Failure net, not a wait: if a browser's tracking protection (e.g. Edge's
      // Tracking Prevention) blocks the embed iframe, createTweet() may never
      // settle. Rather than ripple forever, fall back to the plain X link card.
      const giveUp = setTimeout(fail, TWEET_GIVE_UP_MS);

      twttr.widgets.createTweet(tweetId, stage, {
        theme: theme,
        dnt: true,
        align: "center",
        conversation: "none",
      }).then((el) => {
        if (settled) return;
        if (!el) {
          fail();
          return;
        }
        settled = true;
        clearTimeout(giveUp);
        tracker.rendered();
      }).catch(fail);
    };

    if (window.twttr && window.twttr.widgets && typeof window.twttr.widgets.createTweet === "function") {
      doCreate(window.twttr);
      return;
    }

    if (window.twttr && window.twttr.ready) {
      window.twttr.ready(doCreate);
    } else {
      let attempts = 0;
      const interval = setInterval(() => {
        attempts++;
        if (window.twttr && window.twttr.widgets && typeof window.twttr.widgets.createTweet === "function") {
          clearInterval(interval);
          doCreate(window.twttr);
        } else if (attempts > 50) {
          clearInterval(interval);
          showTweetFallback(container, username, tweetId, rippleHandle);
        }
      }, 100);
    }
  }

  function bindMediaFadeIn(el) {
    el.classList.add("media-fade-in");
    const reveal = () => el.classList.add("is-loaded");
    if (el.tagName === "IMG" && el.complete && el.naturalWidth > 0) {
      requestAnimationFrame(reveal);
      return;
    }
    el.addEventListener("load", reveal, { once: true });
    el.addEventListener("error", reveal, { once: true });
  }

  function createMedia(url) {
    const host = url.hostname.toLowerCase().replace(/^www\./, "");
    const path = url.pathname.toLowerCase();

    const frame = (src, title) => {
      const iframe = document.createElement("iframe");
      iframe.src = src;
      iframe.title = title;
      iframe.loading = "lazy";
      iframe.allow = "accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture; web-share";
      iframe.allowFullscreen = true;
      bindMediaFadeIn(iframe);
      return iframe;
    };

    if (host === "youtu.be" || host === "youtube.com" || host === "m.youtube.com") {
      const id = host === "youtu.be"
        ? url.pathname.slice(1)
        : (url.pathname === "/watch" ? url.searchParams.get("v") : url.pathname.split("/").filter(Boolean).at(-1));
      if (id && /^[\w-]{6,}$/.test(id)) {
        return frame(`https://www.youtube-nocookie.com/embed/${encodeURIComponent(id)}`, "YouTube video");
      }
    }

    if (host === "vimeo.com" || host === "player.vimeo.com") {
      const id = url.pathname.split("/").filter(Boolean).at(-1);
      if (id && /^\d+$/.test(id)) {
        return frame(`https://player.vimeo.com/video/${id}`, "Vimeo video");
      }
    }

    if (host === "twitter.com" || host === "x.com" || host === "mobile.twitter.com" || host === "mobile.x.com") {
      const statusMatch = url.pathname.match(/(?:^|\/)(?:status|statuses)\/(\d+)/i);
      if (statusMatch && /^\d+$/.test(statusMatch[1])) {
        const tweetId = statusMatch[1];
        const userMatch = url.pathname.match(/^\/([^/]+)\/(?:status|statuses)/i);
        const username = (userMatch && userMatch[1] !== "i") ? userMatch[1] : "post";

        const container = document.createElement("div");
        container.className = "tweet-embed-container";
        renderTweet(container, username, tweetId);
        return container;
      }
    }

    if (/\.(mp4|webm|ogg|mov)(?:$)/i.test(path)) {
      const video = document.createElement("video");
      video.src = url.href;
      video.controls = true;
      video.preload = "metadata";
      return video;
    }

    if (/\.(mp3|wav|m4a|aac|flac)(?:$)/i.test(path)) {
      const audio = document.createElement("audio");
      audio.src = url.href;
      audio.controls = true;
      audio.preload = "metadata";
      return audio;
    }

    if (/\.(avif|gif|jpe?g|png|webp|svg)(?:$)/i.test(path)) {
      const image = document.createElement("img");
      image.src = url.href;
      image.alt = "Journal media";
      image.loading = "lazy";
      bindMediaFadeIn(image);
      return image;
    }

    return null;
  }

  function createLinkFallback(url) {
    const card = document.createElement("div");
    card.className = "tweet-fallback-card";
    const badge = document.createElement("span");
    badge.className = "tweet-fallback-badge";
    badge.textContent = "External Link";
    const link = document.createElement("a");
    link.href = url.href;
    link.target = "_blank";
    link.rel = "noopener noreferrer";
    link.textContent = url.href;
    card.append(badge, link);
    return card;
  }

  function mountCatalogSlots() {
    const slots = document.querySelectorAll("[data-media-url]");
    slots.forEach((slot) => {
      const raw = slot.getAttribute("data-media-url") || "";
      if (slot.getAttribute("data-media-mounted") === raw && slot.childElementCount > 0) {
        return;
      }
      const url = safeUrl(raw);
      slot.replaceChildren();
      if (!url) return;
      const media = createMedia(url) || createLinkFallback(url);
      slot.append(media);
      slot.setAttribute("data-media-mounted", raw);
    });
  }

  window.ResImperiiMedia = {
    safeUrl,
    getActiveTheme,
    createMedia,
    createLinkFallback,
    mountCatalogSlots,
  };

  // On the async catalog shell, start downloading Twitter's widgets.js while
  // Turso is still being queried instead of after the cards arrive, so the
  // tweet's own ~1-2s render pipeline overlaps the DB round-trip.
  if (document.querySelector("#journals-catalog-stage[hx-get]")) {
    ensureTwitterWidgetsScript();
  }

  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", mountCatalogSlots);
  } else {
    mountCatalogSlots();
  }
  document.addEventListener("htmx:afterSwap", mountCatalogSlots);
  document.addEventListener("htmx:afterSettle", mountCatalogSlots);

  const themeObserver = new MutationObserver(() => {
    const slots = document.querySelectorAll("[data-media-url]");
    slots.forEach((slot) => {
      if (slot.querySelector(".tweet-embed-container")) {
        const raw = slot.getAttribute("data-media-url") || "";
        const url = safeUrl(raw);
        if (url) {
          slot.replaceChildren();
          const media = createMedia(url);
          if (media) slot.append(media);
        }
      }
    });
  });
  themeObserver.observe(document.documentElement, { attributes: true, attributeFilter: ["data-theme"] });
})();
