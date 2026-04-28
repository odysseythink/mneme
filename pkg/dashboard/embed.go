// Package dashboard hosts the M10 web UI as embedded static assets.
//
// At go-build time, //go:embed snapshots the contents of web/dist/*. The repo
// commits a placeholder web/dist/index.html so go build always succeeds even
// on a fresh clone without running pnpm. After `make web-build`, web/dist/
// contains the real React bundle.
package dashboard

import (
	"embed"
	"io/fs"
	"os"
)

//go:embed dist/*
var distFS embed.FS

// FS returns the filesystem the daemon should serve from.
//
// If MNEME_DASHBOARD_DEV_DIR is set to a non-empty path, that filesystem is
// returned instead of the embed FS — useful with `pnpm build --watch`.
func FS() fs.FS {
	if dir := os.Getenv("MNEME_DASHBOARD_DEV_DIR"); dir != "" {
		return os.DirFS(dir)
	}
	sub, err := fs.Sub(distFS, "dist")
	if err != nil {
		panic(err)
	}
	return sub
}
