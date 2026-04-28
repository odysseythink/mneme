package dashboard

import (
	"io/fs"
	"net/http"
	"path"
	"strings"
)

// Deps is reserved for future composition; intentionally empty in M10a.
type Deps struct{}

// Mount registers dashboard routes on mux: /, /assets/*, /static/*.
// The bootstrap query-token path is enforced inside daemon.AuthMW (M8).
//
// Asset routes intentionally do NOT use http.StripPrefix: the embedded FS
// keeps files under their original prefix (e.g. dist/assets/foo.js), so the
// handler resolves r.URL.Path verbatim against FS().
func Mount(mux *http.ServeMux, _ Deps) {
	mux.Handle("/", spaHandler())
	mux.Handle("/assets/", assetHandler())
	mux.Handle("/static/", assetHandler())
}

func spaHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		f := FS()
		clean := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		if clean != "" {
			if file, err := f.Open(clean); err == nil {
				file.Close()
				http.ServeFileFS(w, r, f, clean)
				return
			}
		}
		http.ServeFileFS(w, r, f, "index.html")
	})
}

func assetHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f := FS()
		clean := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		if clean == "" {
			http.NotFound(w, r)
			return
		}
		if _, err := fs.Stat(f, clean); err != nil {
			http.NotFound(w, r)
			return
		}
		http.ServeFileFS(w, r, f, clean)
	})
}
