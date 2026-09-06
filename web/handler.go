package webassets

import (
	"io/fs"
	"net/http"
	"path"
	"strings"
)

func fileHandler(root fs.FS) http.Handler {
	files := http.FileServer(http.FS(root))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		name := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		if name == "" {
			name = "."
		}
		if _, err := fs.Stat(root, name); err == nil {
			w.Header().Set("Cache-Control", "no-cache")
			files.ServeHTTP(w, r)
			return
		}
		// Only document routes get the SPA shell; missing assets remain 404.
		if r.URL.Path == "/admin" || strings.HasPrefix(r.URL.Path, "/docs/") || strings.HasPrefix(r.URL.Path, "/admin/") {
			index, err := fs.ReadFile(root, "index.html")
			if err != nil {
				http.Error(w, "Frontend unavailable", http.StatusServiceUnavailable)
				return
			}
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Header().Set("Cache-Control", "no-cache")
			w.Write(index)
			return
		}
		http.NotFound(w, r)
	})
}
