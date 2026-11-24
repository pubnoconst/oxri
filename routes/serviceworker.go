package routes

import (
	"bytes"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"net/http"

	"github.com/oxri/oxri-go/views/offline"
)

//go:embed sw.js
var swTemplate []byte

// ServiceWorkerPath is served from the root so the worker's scope is the
// whole site (a worker under /assets/ could only control /assets/).
const ServiceWorkerPath = "/sw.js"

const offlineHashMarker = "__OFFLINE_HASH__"

// serviceWorkerJS is sw.js with its cache name tied to the offline page's
// content: a new offline page means a new worker, which re-caches it.
var serviceWorkerJS = func() []byte {
	if !bytes.Contains(swTemplate, []byte(offlineHashMarker)) {
		panic("routes: sw.js lost its " + offlineHashMarker + " marker")
	}
	sum := sha256.Sum256(offline.HTML)
	return bytes.Replace(swTemplate, []byte(offlineHashMarker), []byte(hex.EncodeToString(sum[:])[:12]), 1)
}()

// handleServiceWorker serves the worker. no-cache, so a deploy reaches phones
// on their next visit.
func handleServiceWorker(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	_, _ = w.Write(serviceWorkerJS)
}

// handleOffline serves the offline page (fetched by the worker at install).
func handleOffline(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	_, _ = w.Write(offline.HTML)
}
