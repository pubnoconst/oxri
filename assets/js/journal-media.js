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

  function showRippleLoader(container) {
    container.style.height = "";
    container.replaceChildren();

    // Quiet empty space inside the media slot where the water drop originates
    const spacer = document.createElement("div");
    spacer.className = "media-ripple-spacer";
    spacer.setAttribute("role", "status");
    spacer.setAttribute("aria-label", "Loading media");
    container.append(spacer);

    // Mount the optical water-drop ripple over the entire enclosing .journal-card
    // so the expanding wavefront disturbs the preview text as well.
    const hostCard = container.closest(".journal-card") || container;
    const existingOverlay = hostCard.querySelector(":scope > .card-water-ripple");
    if (existingOverlay) existingOverlay.remove();

    const overlay = document.createElement("div");
    overlay.className = "card-water-ripple";
    overlay.setAttribute("aria-hidden", "true");

    const drop = document.createElement("span");
    drop.className = "card-water-drop";

    const wave1 = document.createElement("span");
    wave1.className = "card-water-wave wave-1";
    const wave2 = document.createElement("span");
    wave2.className = "card-water-wave wave-2";
    const wave3 = document.createElement("span");
    wave3.className = "card-water-wave wave-3";

    overlay.append(drop, wave1, wave2, wave3);
    hostCard.append(overlay);

    const alignOrigin = () => {
      if (!spacer.isConnected || !hostCard.isConnected) return;
      const hostRect = hostCard.getBoundingClientRect();
      const spacerRect = spacer.getBoundingClientRect();
      if (hostRect.width <= 0 || hostRect.height <= 0) return;

      const x = (spacerRect.left - hostRect.left) + (spacerRect.width / 2);
      const y = (spacerRect.top - hostRect.top) + (spacerRect.height / 2);
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

    requestAnimationFrame(alignOrigin);

    return {
      spacer,
      overlay,
      fadeOut() {
        overlay.classList.add("is-fading-out");
      },
      removeSpacer() {
        if (spacer.parentNode === container) spacer.remove();
      },
      cleanup() {
        if (spacer.parentNode === container) spacer.remove();
        if (overlay.parentNode) overlay.remove();
      },
    };
  }

  function showTweetFallback(container, username, tweetId, rippleHandle) {
    if (rippleHandle) rippleHandle.cleanup();
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

  // Wait until Twitter's internal iframe resize settles before revealing, preventing double-jumps.
  function waitForStableHeight(el, onSettled) {
    let done = false;
    let settleTimer = null;
    let maxTimer = null;
    let observer = null;

    const finish = () => {
      if (done) return;
      done = true;
      if (settleTimer) clearTimeout(settleTimer);
      if (maxTimer) clearTimeout(maxTimer);
      if (observer) observer.disconnect();
      onSettled();
    };

    const scheduleSettle = () => {
      if (settleTimer) clearTimeout(settleTimer);
      const h = el.getBoundingClientRect().height;
      // Wait 140ms of stability once the tweet has a real rendered height
      if (h > 40) {
        settleTimer = setTimeout(finish, 140);
      }
    };

    if (typeof ResizeObserver !== "undefined") {
      observer = new ResizeObserver(scheduleSettle);
      observer.observe(el);
    }

    scheduleSettle();
    maxTimer = setTimeout(finish, 950);
  }

  function revealTweetFluidly(container, rippleHandle, stage) {
    const prefersReducedMotion = window.matchMedia && window.matchMedia("(prefers-reduced-motion: reduce)").matches;
    if (prefersReducedMotion) {
      if (rippleHandle) rippleHandle.cleanup();
      stage.classList.remove("is-staging");
      stage.classList.add("is-ready");
      container.style.height = "";
      return;
    }

    const startHeight = container.getBoundingClientRect().height;
    const stageHeight = stage.getBoundingClientRect().height;
    const style = window.getComputedStyle(container);
    const padY = (parseFloat(style.paddingTop) || 0) + (parseFloat(style.paddingBottom) || 0);
    const computedMax = parseFloat(style.maxHeight);

    let targetHeight = Math.max(startHeight, stageHeight + padY);
    if (Number.isFinite(computedMax) && computedMax > 0) {
      targetHeight = Math.min(targetHeight, computedMax);
    }

    container.style.height = `${startHeight}px`;

    if (rippleHandle) {
      rippleHandle.fadeOut();
      rippleHandle.removeSpacer();
    }
    stage.classList.remove("is-staging");

    // Force layout flush so the height transition starts cleanly from startHeight
    void container.offsetHeight;

    requestAnimationFrame(() => {
      container.style.height = `${targetHeight}px`;
      stage.classList.add("is-ready");
    });

    let settled = false;
    const cleanup = () => {
      if (settled) return;
      settled = true;
      if (rippleHandle) rippleHandle.cleanup();
      container.style.height = "";
    };
    container.addEventListener("transitionend", (e) => {
      if (e.target === container && e.propertyName === "height") {
        cleanup();
      }
    }, { once: true });
    setTimeout(cleanup, 520);
  }

  function renderTweet(container, username, tweetId) {
    ensureTwitterWidgetsScript();
    const theme = getActiveTheme();

    // Full-card pure-CSS water-drop ripple originating from the quiet media space
    const rippleHandle = showRippleLoader(container);

    const doCreate = (twttr) => {
      const stage = document.createElement("div");
      stage.className = "tweet-embed-stage is-staging";
      container.append(stage);

      twttr.widgets.createTweet(tweetId, stage, {
        theme: theme,
        dnt: true,
        align: "center",
        conversation: "none",
      }).then((el) => {
        if (!el) {
          stage.remove();
          showTweetFallback(container, username, tweetId, rippleHandle);
          return;
        }
        waitForStableHeight(el, () => {
          revealTweetFluidly(container, rippleHandle, stage);
        });
      }).catch(() => {
        stage.remove();
        showTweetFallback(container, username, tweetId, rippleHandle);
      });
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
        } else if (attempts > 30) {
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
