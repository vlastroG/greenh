// Package web serves the embedded dashboard build.
package web

import (
	"embed"
	"io/fs"
	"net/http"
	"path"
	"strings"
)

//go:embed all:dist
var dist embed.FS

// Handler serves the dashboard with SPA fallback to index.html.
func Handler() http.Handler {
	sub, err := fs.Sub(dist, "dist")
	if err != nil {
		panic(err) // "dist" is a compile-time embed root
	}
	return newHandler(sub)
}

func newHandler(fsys fs.FS) http.Handler {
	files := http.FileServerFS(fsys)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := r.URL.Path
		if strings.HasPrefix(p, "/assets/") {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
			files.ServeHTTP(w, r)
			return
		}
		if strings.HasPrefix(p, "/api/") {
			http.NotFound(w, r)
			return
		}
		if _, err := fs.Stat(fsys, "index.html"); err != nil {
			http.Error(w, "dashboard not built: run npm run build in web/", http.StatusServiceUnavailable)
			return
		}
		if p == "/" || path.Ext(p) == "" {
			// serve index directly; FileServer would redirect /index.html to /
			w.Header().Set("Cache-Control", "no-cache")
			http.ServeFileFS(w, r, fsys, "index.html")
			return
		}
		files.ServeHTTP(w, r)
	})
}
