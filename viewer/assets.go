package main

import (
	"embed"
	"io/fs"
	"net/http"
	"path"
	"strings"
)

// distFS holds the built web/ SPA, copied into viewer/dist by the build
// (Taskfile `viewer-embed`). Committed as just `.gitkeep` so a plain
// `go build` still compiles before the first web build.
//
//go:embed all:dist
var distFS embed.FS

// spaRoot is the embedded directory holding the built SPA.
const spaRoot = "dist"

// spaHandler returns the SPA file handler when a built index.html is embedded.
func spaHandler() (http.Handler, bool) {
	sub, err := fs.Sub(distFS, spaRoot)
	if err != nil {
		return nil, false
	}
	if _, err := fs.Stat(sub, "index.html"); err != nil {
		return nil, false
	}
	return spaFileHandler(sub), true
}

// spaFileHandler serves static assets and falls back to index.html for
// extension-less paths (hash routes never reach the server, but deep links do).
func spaFileHandler(sub fs.FS) http.Handler {
	files := http.FileServerFS(sub)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		if p == "" {
			p = "index.html"
		}
		if _, err := fs.Stat(sub, p); err == nil {
			files.ServeHTTP(w, r)
			return
		}
		if path.Ext(p) != "" {
			http.NotFound(w, r)
			return
		}
		http.ServeFileFS(w, r, sub, "index.html")
	})
}
