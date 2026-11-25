/*
 * Shared tactile media renderer for Res Imperii journals.
 * Supports YouTube, Vimeo, X / Twitter posts, images, audio, and video.
 */
(() => {
  /**
   * @param {string | null | undefined} value
   * @returns {URL | null}
   */
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
  //   1. Turso done -> card text is shown, with one quiet "Loading preview"
  //      line (the dot matrix) where the tweet will open.
  //   2. The tweet renders off-screen (invisible stage). The moment Twitter
  //      reports a real height, flipPush removes the line and opens the slot to
  //      exactly that height in one layout, then unrolls it while pushing the
  //      card body down. Twitter's second resize simply retargets.
  // The tweet itself is never shown half-laid-out, and nothing waits on a
  // fixed "settle" timer.

  // showLineLoader puts the waiting line in the tweet slot: a twinkle (a
  // download) seeded per slot, so cards loading together glint differently.
  // clearPending removes it inside the FLIP's layout step, so the slot unrolls
  // from the line into the tweet.
  let slotCount = 0;
  /**
   * @param {HTMLElement} container
   * @param {string} seed
   */
  function showLineLoader(container, seed) {
    container.style.height = "";
    container.replaceChildren();
    container.classList.add("is-loading-line");
    container.setAttribute("role", "status");
    container.append(window.ResImperiiDots.create("Loading preview", "twinkle", seed + ":" + slotCount++));
  }

  // clearPending ends the loading state of a tweet slot.
  /** @param {HTMLElement} container */
  function clearPending(container) {
    const line = container.querySelector(":scope > .dot-loading");
    if (line) line.remove();
    container.classList.remove("is-loading-line");
    container.removeAttribute("role");
  }

  /**
   * @param {HTMLElement} container
   * @param {string} username
   * @param {string} tweetId
   */
  function showTweetFallback(container, username, tweetId) {
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
  // visualRect: an element's on-screen top and visible height, accounting for
  // a FLIP animation of ours that is still clipping its bottom edge.
  /** @param {HTMLElement} el */
  function visualRect(el) {
    const r = el.getBoundingClientRect();
    let clipBottom = 0;
    if (flipAnimations.has(el)) {
      const m = /inset\(\s*[-\d.]+px\s+[-\d.]+px\s+([-\d.]+)px/.exec(window.getComputedStyle(el).clipPath || "");
      if (m) clipBottom = Math.max(0, parseFloat(m[1]) || 0);
    }
    return { top: r.top, height: r.height - clipBottom };
  }

  /**
   * @param {HTMLElement} container
   * @param {() => void} change
   */
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

    // First: where things are *visually* now, including any in-flight FLIP.
    // getBoundingClientRect includes a running translate but not a running
    // clip, so a card that is still unrolling would measure at its full height
    // and the next tweet's push would cancel its unroll and snap its edge down.
    // With twelve tweets arriving within a second that happened to nearly
    // every card. Subtract the in-flight bottom clip to get the visible height.
    const first = new Map(els.map((el) => [el, visualRect(el)]));

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
      if (!a) return;
      const b = el.getBoundingClientRect();
      const isSlot = el === container;
      const dy = isSlot ? 0 : a.top - b.top; // the slot's own top does not move
      const grew = isSlot || peers.includes(el) ? b.height - a.height : 0;
      if (Math.abs(dy) < 0.5 && grew < 0.5) return;
      // Clip only the bottom edge while a card or the slot unrolls; keep side
      // and top shadows.
      /** @type {Keyframe} */
      const from = isSlot ? {} : { transform: `translateY(${dy.toFixed(1)}px)` };
      /** @type {Keyframe} */
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

  /**
   * @param {HTMLElement} container
   * @param {HTMLElement} stage
   */
  function expandAndRevealTweet(container, stage) {
    let target = 0;
    let revealed = false;
    let stopped = false;
    /** @type {ResizeObserver | null} */
    let observer = null;

    // Measure Twitter's iframe itself, never the stage/container: once the stage
    // is in flow it takes the container's height, which would feed back into
    // itself. offsetHeight ignores the stage's scale-in transform.
    /** @type {HTMLIFrameElement | null} */
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
    };

    const onHeight = () => {
      if (stopped) return;
      if (!container.isConnected) {
        stop();
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

  /**
   * @param {HTMLElement} container
   * @param {string} username
   * @param {string} tweetId
   */
  function renderTweet(container, username, tweetId) {
    ensureTwitterWidgetsScript();
    const theme = getActiveTheme();

    // Card text is visible; the slot shows "Loading preview" while we wait.
    showLineLoader(container, tweetId);

    const doCreate = (/** @type {TwitterWidgets} */ twttr) => {
      const stage = document.createElement("div");
      stage.className = "tweet-embed-stage is-staging";
      container.append(stage);

      // Start tracking before Twitter renders so the very first height is caught.
      const tracker = expandAndRevealTweet(container, stage);

      let settled = false;
      const fail = () => {
        if (settled) return;
        settled = true;
        clearTimeout(giveUp);
        // Already on screen (the height tracker revealed it): never replace it.
        if (!stage.classList.contains("is-staging")) return;
        tracker.abort();
        stage.remove();
        showTweetFallback(container, username, tweetId);
      };

      // Failure net, not a wait: if a browser's tracking protection (e.g. Edge's
      // Tracking Prevention) blocks the embed iframe, createTweet() may never
      // settle. Rather than wait forever, fall back to the plain X link card.
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
          showTweetFallback(container, username, tweetId);
        }
      }, 100);
    }
  }

  /** @param {HTMLImageElement | HTMLIFrameElement} el */
  function bindMediaFadeIn(el) {
    el.classList.add("media-fade-in");
    const reveal = () => el.classList.add("is-loaded");
    if (el instanceof HTMLImageElement && el.complete && el.naturalWidth > 0) {
      requestAnimationFrame(reveal);
      return;
    }
    el.addEventListener("load", reveal, { once: true });
    el.addEventListener("error", reveal, { once: true });
  }

  /**
   * @param {URL} url
   * @param {string | null} [still] the server's still for a GIF or video
   *   (/thumb/…, internal/thumb), shown on its tap tile; the media's own host
   *   is asked for nothing until the tap.
   * @returns {HTMLElement | null}
   */
  function createMedia(url, still = null) {
    const host = url.hostname.toLowerCase().replace(/^www\./, "");
    const path = url.pathname.toLowerCase();

    const frame = (/** @type {string} */ src, /** @type {string} */ title) => {
      const iframe = document.createElement("iframe");
      iframe.src = src;
      iframe.title = title;
      iframe.loading = "lazy";
      iframe.allow = "accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture; web-share";
      iframe.allowFullscreen = true;
      bindMediaFadeIn(iframe);
      return iframe;
    };

    // Nothing plays, or even loads its player, until it is tapped (owner,
    // 2026-11-26): a video, a GIF or an embed is first a still tile in its
    // reserved space (mediakind.go), and the tap that starts it is the
    // reader's choice. The tap also lets it play at once (autoplay after a
    // gesture).
    /**
     * @param {string} label what it is, e.g. "YouTube video"
     * @param {string | null} thumb a still to show, if one is free to get
     * @param {() => HTMLElement} start builds the real thing
     */
    const tapToPlay = (label, thumb, start) => {
      const button = document.createElement("button");
      button.type = "button";
      button.className = "media-facade";
      button.setAttribute("aria-label", `Play ${label}`);
      if (thumb) {
        const still = document.createElement("img");
        still.src = thumb;
        still.alt = "";
        still.loading = "lazy";
        still.decoding = "async";
        still.referrerPolicy = "no-referrer";
        bindMediaFadeIn(still);
        button.append(still);
      } else {
        const name = document.createElement("span");
        name.className = "media-facade-label";
        name.textContent = label;
        button.append(name);
      }
      const mark = document.createElement("span");
      mark.className = "tweet-play-mark";
      mark.setAttribute("aria-hidden", "true");
      button.append(mark);
      button.addEventListener("click", (event) => {
        event.preventDefault();
        const real = start();
        button.replaceWith(real);
        if (real instanceof HTMLMediaElement) real.play().catch(() => {});
      }, { once: true });
      return button;
    };

    if (host === "youtu.be" || host === "youtube.com" || host === "m.youtube.com") {
      const id = host === "youtu.be"
        ? url.pathname.slice(1)
        : (url.pathname === "/watch" ? url.searchParams.get("v") : url.pathname.split("/").filter(Boolean).at(-1));
      if (id && /^[\w-]{6,}$/.test(id)) {
        const safe = encodeURIComponent(id);
        return tapToPlay("YouTube video", still || `https://i.ytimg.com/vi/${safe}/hqdefault.jpg`,
          () => frame(`https://www.youtube-nocookie.com/embed/${safe}?autoplay=1`, "YouTube video"));
      }
    }

    if (host === "vimeo.com" || host === "player.vimeo.com") {
      const id = url.pathname.split("/").filter(Boolean).at(-1);
      if (id && /^\d+$/.test(id)) {
        return tapToPlay("Vimeo video", still, () => frame(`https://player.vimeo.com/video/${id}?autoplay=1`, "Vimeo video"));
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
        // Most tweets are drawn by the server (views/tweet) and never reach
        // here. One without a snapshot yet uses X's embed, built only when its
        // card comes near the screen: a page of them no longer loads every
        // iframe up front on a budget phone.
        whenNear(container, () => renderTweet(container, username, tweetId));
        return container;
      }
    }

    if (/\.(mp4|webm|ogg|mov)(?:$)/i.test(path)) {
      return tapToPlay("Video", null, () => {
        const video = document.createElement("video");
        video.src = url.href;
        video.controls = true;
        video.preload = "auto";
        return fallBackOnError(video, url, "Video");
      });
    }

    if (/\.(mp3|wav|m4a|aac|flac)(?:$)/i.test(path)) {
      // The player is its own tap-to-play; nothing is fetched until then.
      const audio = document.createElement("audio");
      audio.src = url.href;
      audio.controls = true;
      audio.preload = "none";
      return fallBackOnError(audio, url, "Audio");
    }

    // A GIF animates as soon as it loads, and has no still to show without
    // loading it all: it waits behind a tile.
    if (/\.gif$/i.test(path)) {
      return tapToPlay("GIF", still, () => {
        const gif = document.createElement("img");
        gif.src = url.href;
        gif.alt = "Journal media (GIF)";
        bindMediaFadeIn(gif);
        return fallBackOnError(gif, url, "Image");
      });
    }

    if (/\.(avif|jpe?g|png|webp|svg)(?:$)/i.test(path)) {
      const image = document.createElement("img");
      image.src = url.href;
      image.alt = "Journal media";
      image.loading = "lazy";
      bindMediaFadeIn(image);
      return fallBackOnError(image, url, "Image");
    }

    return null;
  }

  // whenNear runs build once el is within about a screen of the viewport.
  const nearObserver = "IntersectionObserver" in window
    ? new IntersectionObserver((entries, observer) => {
        entries.forEach((entry) => {
          if (!entry.isIntersecting) return;
          observer.unobserve(entry.target);
          const build = pendingBuilds.get(entry.target);
          pendingBuilds.delete(entry.target);
          if (build) build();
        });
      }, { rootMargin: "600px 0px" })
    : null;
  /** @type {WeakMap<Element, () => void>} */
  const pendingBuilds = new WeakMap();
  /**
   * @param {HTMLElement} el
   * @param {() => void} build
   */
  function whenNear(el, build) {
    if (!nearObserver) {
      build();
      return;
    }
    pendingBuilds.set(el, build);
    nearObserver.observe(el);
  }

  // The tweet box's play mark (views/tweet): swap the box for X's own player.
  // This is the only time X's script loads for a post with a snapshot.
  document.addEventListener("click", (event) => {
    const play = event.target instanceof Element ? event.target.closest("[data-tweet-play]") : null;
    if (!play) return;
    const box = play.closest(".tweet-box");
    const tweetId = box && box.getAttribute("data-tweet-id");
    const username = box && box.getAttribute("data-tweet-user");
    if (!tweetId || !/^\d+$/.test(tweetId)) return;
    event.preventDefault();
    const container = document.createElement("div");
    container.className = "tweet-embed-container";
    box.replaceWith(container);
    renderTweet(container, username || "post", tweetId);
  });

  /**
   * A quiet card for a link with no inline media, or media that failed to
   * load. It names the site rather than printing the whole address, which for
   * signed CDN links runs to hundreds of characters.
   * @param {URL} url
   * @param {string} [badgeText]
   */
  function createLinkFallback(url, badgeText = "External Link") {
    const card = document.createElement("div");
    card.className = "tweet-fallback-card";
    const badge = document.createElement("span");
    badge.className = "tweet-fallback-badge";
    badge.textContent = badgeText;
    const link = document.createElement("a");
    link.href = url.href;
    link.target = "_blank";
    link.rel = "noopener noreferrer";
    link.textContent = `Open on ${url.hostname.replace(/^www\./, "")}`;
    link.title = url.href;
    card.append(badge, link);
    return card;
  }

  /**
   * Swaps media that fails to load (an expired or refused link) for the link
   * card, instead of leaving a broken-image icon in a framed box.
   * @template {HTMLElement} T
   * @param {T} el
   * @param {URL} url
   * @param {string} what
   * @returns {T}
   */
  function fallBackOnError(el, url, what) {
    el.addEventListener("error", () => {
      if (el.isConnected) el.replaceWith(createLinkFallback(url, `${what} did not load`));
    }, { once: true });
    return el;
  }

  // A still the server made is served from this site only (/thumb/…).
  /** @param {string | null} raw */
  function safeStill(raw) {
    return raw && /^\/thumb\/[0-9a-f]{24}\.jpg\?v=\d+$/.test(raw) ? raw : null;
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
      const media = createMedia(url, safeStill(slot.getAttribute("data-media-still"))) || createLinkFallback(url);
      slot.append(media);
      slot.setAttribute("data-media-mounted", raw);
    });
  }

  // A tap tile the server drew (views/journals mediaSlot): the tap builds the
  // player as a script-made tile would, in the same place.
  document.addEventListener("click", (event) => {
    const target = event.target instanceof Element ? event.target : null;
    const tile = target && target.closest("[data-media-play]");
    const slot = tile && tile.closest("[data-media-url]");
    const url = slot && safeUrl(slot.getAttribute("data-media-url") || "");
    if (!tile || !url) return;
    event.preventDefault();
    const media = createMedia(url, safeStill(slot.getAttribute("data-media-still")));
    if (!media) return;
    tile.replaceWith(media);
    if (media.classList.contains("media-facade")) media.click();
  });

  // The journal editor's live preview reuses these two.
  window.ResImperiiMedia = { safeUrl, createMedia };

  // X's widgets.js is no longer preloaded: tweets with a snapshot are drawn
  // by the server, and the script loads only for an embed that is needed
  // (a post without a snapshot yet, or a tapped video).

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
