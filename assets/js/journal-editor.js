/*
 * Browser-only journal editor prototype. Draft content stays in this page and
 * no input is submitted or persisted. Keep preview rendering isolated here so
 * the server view remains a small semantic shell around the editor controls.
 */
(() => {
  // Resolve the view's stable IDs once; if the editor is not on this page, exit
  // without affecting other routes that share the site's script environment.
  const contentInput = document.getElementById("journal-content");
  const mediaInput = document.getElementById("journal-media-link");
  const tagsInput = document.getElementById("journal-tags");
  const renderedContent = document.getElementById("journal-rendered-content");
  const mediaPreview = document.getElementById("journal-preview-media");
  const tagsPreview = document.getElementById("journal-preview-tags");
  const preview = document.getElementById("journal-preview");

  if (!contentInput || !mediaInput || !tagsInput || !renderedContent || !mediaPreview || !tagsPreview || !preview) return;
  if (!window.markdownit || !window.DOMPurify) {
    preview.textContent = "The live preview libraries did not load. Check your connection and reload.";
    return;
  }

  // Raw HTML stays disabled. Sanitization is an additional boundary before the
  // generated markup is inserted into the preview DOM.
  const markdown = window.markdownit({ html: false, linkify: false, breaks: false });

  // Accept only web URLs before assigning user-provided media URLs to elements.
  // Provider embed URLs below are built from validated IDs rather than copied
  // from an arbitrary iframe URL.
  function safeUrl(value) {
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

  function renderTweet(container, username, tweetId) {
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
    // Give provider frames a descriptive title and defer loading until visible.
    const frame = (src, title) => {
      const iframe = document.createElement("iframe");
      iframe.src = src;
      iframe.title = title;
      iframe.loading = "lazy";
      iframe.allow = "accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture; web-share";
      iframe.allowFullscreen = true;
      return iframe;
    };

    // Convert common YouTube share/watch/short links to the privacy-enhanced
    // embed host; only accept a video ID with the provider's expected shape.
    if (host === "youtu.be" || host === "youtube.com" || host === "m.youtube.com") {
      const id = host === "youtu.be" ? url.pathname.slice(1) : (url.pathname === "/watch" ? url.searchParams.get("v") : url.pathname.split("/").filter(Boolean).at(-1));
      if (id && /^[\w-]{6,}$/.test(id)) return frame(`https://www.youtube-nocookie.com/embed/${encodeURIComponent(id)}`, "YouTube video preview");
    }
    // Vimeo's player expects a numeric video ID in its path.
    if (host === "vimeo.com" || host === "player.vimeo.com") {
      const id = url.pathname.split("/").filter(Boolean).at(-1);
      if (id && /^\d+$/.test(id)) return frame(`https://player.vimeo.com/video/${id}`, "Vimeo video preview");
    }
    // Convert Twitter and X post links to official embeds using widgets.js
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
    // Direct files use native browser controls. Metadata preload avoids
    // downloading the full media file just to build the preview.
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
      image.alt = "Linked media preview";
      image.loading = "lazy";
      return image;
    }
    return null;
  }

  // Rebuild the media region on each edit so replacing or clearing a URL also
  // removes the previous embedded element and its ongoing network request.
  function updateMedia() {
    mediaPreview.replaceChildren();
    const url = safeUrl(mediaInput.value);
    if (!mediaInput.value.trim()) {
      mediaPreview.textContent = "Your media will appear here";
      mediaPreview.classList.add("is-empty");
      return;
    }

    const media = url && createMedia(url);
    if (media) {
      mediaPreview.classList.remove("is-empty");
      mediaPreview.append(media);
      return;
    }
    mediaPreview.classList.add("is-empty");
    mediaPreview.textContent = url ? "This link type can’t be previewed here." : "Enter a valid http or https link.";
  }

  // Tags are plain text nodes, never HTML, and comma-separated input keeps the
  // prototype field simple while still previewing individual chips.
  function updateTags() {
    const chips = tagsInput.value.split(",").map((tag) => tag.trim()).filter(Boolean);
    tagsPreview.replaceChildren(...chips.map((tag) => {
      const chip = document.createElement("span");
      chip.className = "journal-tag-chip";
      chip.textContent = tag;
      return chip;
    }));
  }

  // Markdown rendering is live; sanitize the output at the DOM insertion point.
  function updateContent() {
    renderedContent.innerHTML = window.DOMPurify.sanitize(markdown.render(contentInput.value));
  }

  contentInput.addEventListener("input", updateContent);
  mediaInput.addEventListener("input", updateMedia);
  tagsInput.addEventListener("input", updateTags);
  updateContent();
  updateMedia();
  updateTags();

  // If the page theme changes, re-render any active tweet widget to match the canvas
  const themeObserver = new MutationObserver(() => {
    if (mediaInput.value.trim() && mediaPreview.querySelector(".tweet-embed-container")) {
      updateMedia();
    }
  });
  themeObserver.observe(document.documentElement, { attributes: true, attributeFilter: ["data-theme"] });
})();
