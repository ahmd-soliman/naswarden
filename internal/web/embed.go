// Package web embeds the built Vue frontend (web/dist, produced by `npm
// run build`) into the naswarden binary, so deployment stays a single
// container/binary even though building the frontend requires Node.
package web

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed all:dist
var distFS embed.FS

// Handler returns an http.Handler serving the embedded frontend build.
// If dist wasn't built yet (e.g. running `go run` straight from a fresh
// checkout without `npm run build` first), this returns a handler that
// reports the problem clearly instead of a confusing 404/500 -- worth
// catching explicitly since it's the single most likely first-run mistake
// for a new contributor.
func Handler() (http.Handler, error) {
	sub, err := fs.Sub(distFS, "dist")
	if err != nil {
		return nil, err
	}
	if _, err := sub.Open("index.html"); err != nil {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w,
				"frontend not built: run `npm run build` in web/ before building naswarden",
				http.StatusInternalServerError)
		}), nil
	}
	return http.FileServer(http.FS(sub)), nil
}
