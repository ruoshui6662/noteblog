//go:build !production

package webassets

import "net/http"

func Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Use the Vite development server on port 5173, or build with -tags production.", http.StatusServiceUnavailable)
	})
}
