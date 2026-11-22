/*
 * Res Imperii — Minimal zero-dependency WebAuthn / Passkey client helper.
 * Handles credential registration (/pilot/user/settings) and sign-in (/login).
 */
(() => {
  "use strict";

  function b64urlToBytes(b64url) {
    const pad = "=".repeat((4 - (b64url.length % 4)) % 4);
    const b64 = (b64url + pad).replace(/-/g, "+").replace(/_/g, "/");
    const raw = atob(b64);
    const out = new Uint8Array(raw.length);
    for (let i = 0; i < raw.length; i++) out[i] = raw.charCodeAt(i);
    return out;
  }

  function bytesToB64url(buf) {
    const bytes = new Uint8Array(buf);
    let str = "";
    for (let i = 0; i < bytes.length; i++) str += String.fromCharCode(bytes[i]);
    return btoa(str).replace(/\+/g, "-").replace(/\//g, "_").replace(/=+$/, "");
  }

  function showStatus(el, msg, isError) {
    if (!el) return;
    el.textContent = msg;
    el.hidden = !msg;
    el.classList.toggle("is-error", Boolean(isError));
  }

  async function registerPasskey(btn, statusEl) {
    if (!window.PublicKeyCredential || !navigator.credentials) {
      showStatus(statusEl, "This browser does not support Passkeys (WebAuthn).", true);
      return;
    }

    const labelInput = document.getElementById("passkey-label-input");
    const label = (labelInput ? labelInput.value : "").trim() || "Passkey";

    btn.disabled = true;
    showStatus(statusEl, "Waiting for your device or passkey manager…", false);

    try {
      const beginResp = await fetch("/pilot/passkeys/begin", {
        method: "POST",
        credentials: "same-origin",
      });
      if (!beginResp.ok) {
        const errText = await beginResp.text();
        throw new Error(errText || "Could not start passkey registration.");
      }

      const opts = await beginResp.json();
      const pubKey = opts.publicKey;
      pubKey.challenge = b64urlToBytes(pubKey.challenge);
      pubKey.user.id = b64urlToBytes(pubKey.user.id);
      if (Array.isArray(pubKey.excludeCredentials)) {
        pubKey.excludeCredentials = pubKey.excludeCredentials.map((c) => ({
          ...c,
          id: b64urlToBytes(c.id),
        }));
      }

      const cred = await navigator.credentials.create({ publicKey: pubKey });
      if (!cred) {
        throw new Error("Passkey registration was cancelled.");
      }

      const payload = {
        id: cred.id,
        rawId: bytesToB64url(cred.rawId),
        type: cred.type,
        response: {
          attestationObject: bytesToB64url(cred.response.attestationObject),
          clientDataJSON: bytesToB64url(cred.response.clientDataJSON),
          transports: typeof cred.response.getTransports === "function" ? cred.response.getTransports() : [],
        },
      };

      const finishResp = await fetch("/pilot/passkeys/finish?name=" + encodeURIComponent(label), {
        method: "POST",
        credentials: "same-origin",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(payload),
      });
      if (!finishResp.ok) {
        const errText = await finishResp.text();
        throw new Error(errText || "Passkey verification failed.");
      }

      showStatus(statusEl, "Passkey added. Reloading…", false);
      window.location.reload();
    } catch (err) {
      const msg = err && err.name === "NotAllowedError"
        ? "Passkey prompt was cancelled or timed out."
        : (err && err.message ? err.message : "Passkey registration failed.");
      showStatus(statusEl, msg, true);
      btn.disabled = false;
    }
  }

  async function loginWithPasskey(btn, statusEl) {
    if (!window.PublicKeyCredential || !navigator.credentials) {
      showStatus(statusEl, "This browser does not support Passkeys (WebAuthn).", true);
      return;
    }

    const nextURL = btn.getAttribute("data-next") || "/";
    btn.disabled = true;
    showStatus(statusEl, "Waiting for your Passkey…", false);

    try {
      const beginResp = await fetch("/auth/passkey/begin", {
        method: "POST",
        credentials: "same-origin",
      });
      if (!beginResp.ok) {
        const errText = await beginResp.text();
        throw new Error(errText || "Could not start Passkey sign-in.");
      }

      const opts = await beginResp.json();
      const pubKey = opts.publicKey;
      pubKey.challenge = b64urlToBytes(pubKey.challenge);
      if (Array.isArray(pubKey.allowCredentials)) {
        pubKey.allowCredentials = pubKey.allowCredentials.map((c) => ({
          ...c,
          id: b64urlToBytes(c.id),
        }));
      }

      const assertion = await navigator.credentials.get({ publicKey: pubKey });
      if (!assertion) {
        throw new Error("Passkey sign-in was cancelled.");
      }

      const payload = {
        id: assertion.id,
        rawId: bytesToB64url(assertion.rawId),
        type: assertion.type,
        response: {
          authenticatorData: bytesToB64url(assertion.response.authenticatorData),
          clientDataJSON: bytesToB64url(assertion.response.clientDataJSON),
          signature: bytesToB64url(assertion.response.signature),
          userHandle: assertion.response.userHandle ? bytesToB64url(assertion.response.userHandle) : "",
        },
      };

      const finishResp = await fetch("/auth/passkey/finish?next=" + encodeURIComponent(nextURL), {
        method: "POST",
        credentials: "same-origin",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(payload),
      });
      if (!finishResp.ok) {
        const errText = await finishResp.text();
        throw new Error(errText || "Passkey sign-in failed.");
      }

      const data = await finishResp.json();
      window.location.assign(data.redirect || nextURL);
    } catch (err) {
      const msg = err && err.name === "NotAllowedError"
        ? "Passkey prompt was cancelled or timed out."
        : (err && err.message ? err.message : "Passkey sign-in failed.");
      showStatus(statusEl, msg, true);
      btn.disabled = false;
    }
  }

  function init() {
    const addBtn = document.getElementById("add-passkey-btn");
    const addStatus = document.getElementById("passkey-register-status");
    if (addBtn) {
      addBtn.addEventListener("click", () => registerPasskey(addBtn, addStatus));
    }

    const loginBtn = document.getElementById("passkey-login-btn");
    const loginStatus = document.getElementById("passkey-login-status");
    if (loginBtn) {
      loginBtn.addEventListener("click", () => loginWithPasskey(loginBtn, loginStatus));
    }
  }

  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", init);
  } else {
    init();
  }
})();
