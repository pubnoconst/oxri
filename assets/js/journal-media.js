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

  function showTweetFallback(container, username, tweetId, isLoading) {
    container.replaceChildren();
    const card = document.createElement("div");
    card.className = "tweet-fallback-card";

    const badge = document.createElement("span");
    badge.className = "tweet-fallback-badge";
    badge.textContent = isLoading ? "Loading post…" : "X / Twitter Post";

    const link = document.createElement("a");
    link.href = `https://x.com/${encodeURIComponent(username)}/status/${encodeURIComponent(tweetId)}`;
    link.target = "_blank";
    link.rel = "noopener noreferrer";
    link.textContent = username !== "post" ? `@${username} on X (status ${tweetId})` : `Post ${tweetId} on X`;

    card.append(badge, link);
    container.append(card);
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

  function renderTweet(container, username, tweetId) {
    ensureTwitterWidgetsScript();
    const theme = getActiveTheme();

    const doCreate = (twttr) => {
      container.replaceChildren();
      twttr.widgets.createTweet(tweetId, container, {
        theme: theme,
        dnt: true,
        align: "center",
        conversation: "none",
      }).then((el) => {
        if (!el) {
          showTweetFallback(container, username, tweetId, false);
        }
      }).catch(() => {
        showTweetFallback(container, username, tweetId, false);
      });
    };

    if (window.twttr && window.twttr.widgets && typeof window.twttr.widgets.createTweet === "function") {
      doCreate(window.twttr);
      return;
    }

    showTweetFallback(container, username, tweetId, true);

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
          showTweetFallback(container, username, tweetId, false);
        }
      }, 100);
    }
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
      const url = safeUrl(raw);
      slot.replaceChildren();
      if (!url) return;
      const media = createMedia(url) || createLinkFallback(url);
      slot.append(media);
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
