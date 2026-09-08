// Package web embeds the built frontend (web/dist) so a single Go binary serves
// both the REST API and the React SPA in production — one origin, no CORS, one
// thing to deploy. The production Docker build runs `npm run build` and bakes
// the resulting web/dist into the binary via the directive below.
//
// For local development you don't need this: run the Vite dev server
// (`npm --prefix web run dev`), which proxies /api to the Go backend. A fresh
// checkout keeps only web/dist/.gitkeep, so the embed still compiles even before
// the SPA is built (index.html is simply absent until you build).
package web

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var distFS embed.FS

// Dist returns the built SPA rooted so that index.html sits at the root.
func Dist() (fs.FS, error) {
	return fs.Sub(distFS, "dist")
}
