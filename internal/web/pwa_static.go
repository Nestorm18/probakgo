package web

import (
	"io"
	"io/fs"
	"net/http"
	"path"
	"strings"
)

// serveServiceWorker streams /sw.js with the headers the browser expects:
//   - text/javascript content-type so the SW runtime is happy
//   - Service-Worker-Allowed: / so the SW can claim the entire origin
//     even though it is served from a sub-FS
//   - cache-control: no-cache so a deployment never leaves a stale SW
//     running after a server update
func serveServiceWorker(staticFS fs.FS) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
		w.Header().Set("Service-Worker-Allowed", "/")
		w.Header().Set("Cache-Control", "no-cache")
		writeEmbedded(w, staticFS, "sw.js")
	}
}

// serveManifest streams /manifest.webmanifest with the right MIME type. The
// manifest is what tells the OS "this is an installable app", so we keep it
// simple and let the browser cache it for an hour.
func serveManifest(staticFS fs.FS) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/manifest+json; charset=utf-8")
		w.Header().Set("Cache-Control", "public, max-age=3600")
		writeEmbedded(w, staticFS, "manifest.webmanifest")
	}
}

// writeEmbedded copies a small static asset from the embed FS to the response.
// These files are small (the SW is ~2 KB, the manifest is <1 KB) so reading
// them fully into memory is the simplest correct approach.
func writeEmbedded(w http.ResponseWriter, staticFS fs.FS, name string) {
	// Reject path traversal early; everything is hard-coded but defence in
	// depth is cheap.
	clean := strings.TrimPrefix(path.Clean("/"+name), "/")
	if clean == "." || !fs.ValidPath(clean) || strings.Contains(clean, "..") {
		http.NotFound(w, nil)
		return
	}
	f, err := staticFS.Open(clean)
	if err != nil {
		http.NotFound(w, nil)
		return
	}
	defer f.Close()
	if info, err := f.Stat(); err == nil && info.IsDir() {
		http.NotFound(w, nil)
		return
	}
	if _, err := io.Copy(w, f); err != nil {
		// Best-effort: the client may have disconnected mid-stream.
		return
	}
}
