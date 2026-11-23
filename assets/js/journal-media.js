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
    const override = document.documentElement.getAttribute("data-theme") || sessionStorage.getItem("res-imperii-theme") || sessionStorage.getItem("theme-override");
    if (override === "dark" || override === "light") return override;
    return window.matchMedia && window.matchMedia("(prefers-color-scheme: dark)").matches ? "dark" : "light";
  }

  function cleanupContainer(container) {
    if (container && typeof container._oxriCleanup === "function") {
      try {
        container._oxriCleanup();
      } catch (_) {
        // Ignore cleanup errors on already-detached elements
      }
      container._oxriCleanup = null;
    }
  }

  function cleanupSlot(slot) {
    if (!slot) return;
    slot.querySelectorAll(".tweet-embed-container").forEach(cleanupContainer);
    const hostCard = slot.closest(".journal-card");
    if (hostCard) {
      const strayOverlay = hostCard.querySelector(":scope > .card-water-ripple");
      if (strayOverlay) strayOverlay.remove();
    }
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

    const waterHandle = window.ResImperiiWater
      ? window.ResImperiiWater.attach(hostCard, overlay, { amplitude: 7 })
      : null;

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
        if (waterHandle) waterHandle.detach();
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

  // Wait until Twitter's internal iframe resize and media layout fully settle before revealing,
  // preventing half-height / 2-phase partial reveals.
  function waitForStableHeight(el, onSettled) {
    let done = false;
    let settleTimer = null;
    let maxTimer = null;
    let resizeObserver = null;
    let mutObserver = null;
    const iframe = el.querySelector("iframe");
    const iframeId = iframe ? iframe.id : "";
    let lastHeight = -1;

    const measure = () => {
      const elH = el.getBoundingClientRect().height;
      const frameH = iframe ? iframe.getBoundingClientRect().height : 0;
      return Math.max(elH, frameH);
    };

    const finish = (shouldReveal) => {
      if (done) return;
      done = true;
      if (settleTimer) clearTimeout(settleTimer);
      if (maxTimer) clearTimeout(maxTimer);
      if (resizeObserver) resizeObserver.disconnect();
      if (mutObserver) mutObserver.disconnect();
      if (iframe) iframe.removeEventListener("load", scheduleSettle);
      window.removeEventListener("message", onTwitterMessage);
      if (shouldReveal && el.isConnected) {
        onSettled();
      }
    };

    const armQuietTimer = () => {
      if (settleTimer) clearTimeout(settleTimer);
      settleTimer = setTimeout(() => finish(true), 750);
    };

    const scheduleSettle = () => {
      if (done) return;
      if (!el.isConnected) {
        finish(false);
        return;
      }
      const h = Math.round(measure());
      if (h <= 80) return;

      if (Math.abs(h - lastHeight) > 1 || settleTimer === null) {
        lastHeight = h;
        armQuietTimer();
      }
    };

    const onTwitterMessage = (ev) => {
      if (done) return;
      if (!el.isConnected) {
        finish(false);
        return;
      }
      if (ev.origin !== "https://platform.twitter.com" && ev.origin !== "https://syndication.twitter.com") {
        return;
      }
      let raw = "";
      if (typeof ev.data === "string") {
        raw = ev.data;
      } else if (ev.data && typeof ev.data === "object") {
        try {
          raw = JSON.stringify(ev.data);
        } catch (_) {
          raw = "";
        }
      }
      if (!raw) return;
      // If Twitter posts a resize/render message for this widget, reset the quiet settle window
      if ((iframeId && raw.includes(iframeId)) || raw.includes("twttr.private.resize")) {
        const h = Math.round(measure());
        if (h > 80) {
          lastHeight = h;
          armQuietTimer();
        }
      }
    };

    if (typeof ResizeObserver !== "undefined") {
      resizeObserver = new ResizeObserver(scheduleSettle);
      resizeObserver.observe(el);
      if (iframe) resizeObserver.observe(iframe);
    }
    if (typeof MutationObserver !== "undefined" && iframe) {
      mutObserver = new MutationObserver(scheduleSettle);
      mutObserver.observe(iframe, { attributes: true, attributeFilter: ["style", "height"] });
    }
    if (iframe) {
      iframe.addEventListener("load", scheduleSettle);
    }
    window.addEventListener("message", onTwitterMessage);

    scheduleSettle();
    maxTimer = setTimeout(() => finish(true), 5500);

    return () => finish(false);
  }

  function revealTweetFluidly(container, rippleHandle, stage, onComplete) {
    if (!container.isConnected || !stage.isConnected) {
      if (rippleHandle) rippleHandle.cleanup();
      return () => {};
    }

    const prefersReducedMotion = window.matchMedia && window.matchMedia("(prefers-reduced-motion: reduce)").matches;
    if (prefersReducedMotion || !rippleHandle || !rippleHandle.spacer || !rippleHandle.spacer.isConnected) {
      if (rippleHandle) rippleHandle.cleanup();
      stage.classList.remove("is-staging");
      stage.classList.add("is-ready");
      if (onComplete) onComplete();
      return () => {};
    }

    const spacer = rippleHandle.spacer;
    const startHeight = Math.round(spacer.getBoundingClientRect().height);
    const targetHeight = Math.ceil(stage.getBoundingClientRect().height);

    let revealed = false;
    let fallbackTimer = null;

    const finishReveal = (shouldReveal) => {
      if (revealed) return;
      revealed = true;
      if (fallbackTimer) clearTimeout(fallbackTimer);
      spacer.removeEventListener("transitionend", onSpacerEnd);

      if (!shouldReveal || !container.isConnected || !stage.isConnected) {
        if (rippleHandle) rippleHandle.cleanup();
        return;
      }

      // Step 2: Swap settled tweet into flow at exact height and fade in all at once
      if (rippleHandle) rippleHandle.cleanup();
      stage.classList.remove("is-staging");
      requestAnimationFrame(() => {
        if (stage.isConnected) {
          stage.classList.add("is-ready");
          if (onComplete) onComplete();
        }
      });
    };

    const onSpacerEnd = (e) => {
      if (e.target === spacer && e.propertyName === "height") {
        finishReveal(true);
      }
    };

    if (Math.abs(targetHeight - startHeight) <= 2) {
      finishReveal(true);
      return () => finishReveal(false);
    }

    // Step 1: Expand ONLY the empty spacer while the tweet remains completely invisible
    // and unclipped at opacity: 0, so zero partial/half-height tweet is ever displayed.
    rippleHandle.fadeOut();
    spacer.style.height = `${startHeight}px`;
    void spacer.offsetHeight;
    spacer.addEventListener("transitionend", onSpacerEnd);
    requestAnimationFrame(() => {
      if (spacer.isConnected) {
        spacer.style.height = `${targetHeight}px`;
      }
    });
    fallbackTimer = setTimeout(() => finishReveal(true), 340);

    return () => finishReveal(false);
  }

  function renderTweet(container, username, tweetId) {
    cleanupContainer(container);
    if (!container.isConnected) return;

    ensureTwitterWidgetsScript();
    const theme = getActiveTheme();

    let cancelled = false;
    let waitInterval = null;
    let cancelWait = null;
    let cancelReveal = null;
    let widthObserver = null;
    let widthDebounce = null;

    // Full-card pure-CSS water-drop ripple originating from the quiet media space
    const rippleHandle = showRippleLoader(container);

    const teardown = () => {
      if (cancelled) return;
      cancelled = true;
      if (waitInterval) {
        clearInterval(waitInterval);
        waitInterval = null;
      }
      if (cancelWait) {
        cancelWait();
        cancelWait = null;
      }
      if (cancelReveal) {
        cancelReveal();
        cancelReveal = null;
      }
      if (widthDebounce) {
        clearTimeout(widthDebounce);
        widthDebounce = null;
      }
      if (widthObserver) {
        widthObserver.disconnect();
        widthObserver = null;
      }
      if (rippleHandle) rippleHandle.cleanup();
    };
    container._oxriCleanup = teardown;

    const armWidthWatcher = (renderedWidth) => {
      if (cancelled || !container.isConnected || typeof ResizeObserver === "undefined" || renderedWidth < 250) {
        return;
      }
      widthObserver = new ResizeObserver(() => {
        if (cancelled || !container.isConnected) {
          teardown();
          return;
        }
        const rawW = Math.floor(container.getBoundingClientRect().width);
        if (rawW < 250) return;
        const clampedW = Math.min(550, rawW);
        if (Math.abs(clampedW - renderedWidth) <= 24) return;

        if (widthDebounce) clearTimeout(widthDebounce);
        widthDebounce = setTimeout(() => {
          if (cancelled || !container.isConnected) return;
          renderTweet(container, username, tweetId);
        }, 350);
      });
      widthObserver.observe(container);
    };

    const doCreate = (twttr) => {
      if (cancelled || !container.isConnected) {
        teardown();
        return;
      }

      const stage = document.createElement("div");
      stage.className = "tweet-embed-stage is-staging";
      container.append(stage);

      const slotWidth = Math.floor(container.getBoundingClientRect().width);
      const renderedWidth = slotWidth >= 250 ? Math.min(550, slotWidth) : 0;
      const tweetOptions = {
        theme: theme,
        dnt: true,
        align: "center",
        conversation: "none",
      };
      if (renderedWidth >= 250) {
        tweetOptions.width = renderedWidth;
      }

      twttr.widgets.createTweet(tweetId, stage, tweetOptions).then((el) => {
        if (cancelled || !container.isConnected) {
          stage.remove();
          teardown();
          return;
        }
        if (!el) {
          stage.remove();
          showTweetFallback(container, username, tweetId, rippleHandle);
          return;
        }
        cancelWait = waitForStableHeight(el, () => {
          if (cancelled || !container.isConnected) return;
          cancelReveal = revealTweetFluidly(container, rippleHandle, stage, () => {
            armWidthWatcher(renderedWidth);
          });
        });
      }).catch(() => {
        if (cancelled || !container.isConnected) return;
        stage.remove();
        showTweetFallback(container, username, tweetId, rippleHandle);
      });
    };

    if (window.twttr && window.twttr.widgets && typeof window.twttr.widgets.createTweet === "function") {
      doCreate(window.twttr);
      return;
    }

    if (window.twttr && window.twttr.ready) {
      window.twttr.ready((twttr) => {
        if (!cancelled && container.isConnected) {
          doCreate(twttr);
        }
      });
    } else {
      let attempts = 0;
      waitInterval = setInterval(() => {
        if (cancelled || !container.isConnected) {
          clearInterval(waitInterval);
          waitInterval = null;
          return;
        }
        attempts++;
        if (window.twttr && window.twttr.widgets && typeof window.twttr.widgets.createTweet === "function") {
          clearInterval(waitInterval);
          waitInterval = null;
          doCreate(window.twttr);
        } else if (attempts > 40) {
          clearInterval(waitInterval);
          waitInterval = null;
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

        // Reserve space synchronously, then mount ripple + Twitter widget on the next frame
        // AFTER the caller appends `container` to the live DOM (prevents detached bailout on
        // double-click theme toggles and ensures .journal-card & slot width are measurable).
        const initialSpacer = document.createElement("div");
        initialSpacer.className = "media-ripple-spacer";
        initialSpacer.setAttribute("role", "status");
        initialSpacer.setAttribute("aria-label", "Loading media");
        container.append(initialSpacer);

        requestAnimationFrame(() => {
          if (!container.isConnected) return;
          renderTweet(container, username, tweetId);
        });
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
      cleanupSlot(slot);
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
    cleanupSlot,
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
          cleanupSlot(slot);
          slot.replaceChildren();
          const media = createMedia(url);
          if (media) slot.append(media);
        }
      }
    });
  });
  themeObserver.observe(document.documentElement, { attributes: true, attributeFilter: ["data-theme"] });
})();
