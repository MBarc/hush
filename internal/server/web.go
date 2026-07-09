package server

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"
)

// The Vite build output is copied to web/dist by the Docker build and
// embedded here. A .gitkeep keeps the package compiling before a build.
//
//go:embed all:web/dist
var embeddedUI embed.FS

// uiHandler serves the built SPA: static assets by path, index.html for
// everything else so client-side routes work on refresh. Returns nil when
// no build has been embedded (dev mode), so the placeholder page is used.
func uiHandler() http.Handler {
	dist, err := fs.Sub(embeddedUI, "web/dist")
	if err != nil {
		return nil
	}
	if _, err := fs.Stat(dist, "index.html"); err != nil {
		return nil // no build embedded yet
	}
	fileServer := http.FileServer(http.FS(dist))
	index, _ := fs.ReadFile(dist, "index.html")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := strings.TrimPrefix(r.URL.Path, "/")
		if p == "" {
			serveIndex(w, index)
			return
		}
		if _, err := fs.Stat(dist, p); err != nil {
			serveIndex(w, index) // SPA fallback
			return
		}
		// Vite asset filenames are content-hashed, so they are safe to cache
		// forever. index.html must never be cached (see serveIndex) or a
		// browser could keep pointing at asset hashes from an old build.
		if strings.HasPrefix(p, "assets/") {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		}
		fileServer.ServeHTTP(w, r)
	})
}

func serveIndex(w http.ResponseWriter, index []byte) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Write(index)
}
