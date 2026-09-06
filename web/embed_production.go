//go:build production

package webassets

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed all:dist
var assets embed.FS

func Handler() http.Handler {
	root, err := fs.Sub(assets, "dist")
	if err != nil {
		panic(err)
	}
	return fileHandler(root)
}
