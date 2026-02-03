package web

import (
	"embed"
	"errors"
	"io/fs"
	"net/http"
	"os"
	"strings"
	"time"
)

//go:generate pnpm install
//go:generate pnpm build
//go:embed all:dist
var files embed.FS

func AppHandler() (http.Handler, error) {
	fsys, err := fs.Sub(files, "dist")
	if err != nil {
		return nil, err
	}
	httpFS := http.FS(fsys)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			serveIndex(httpFS, w, r)
			return
		}

		f, err := httpFS.Open(r.URL.Path)
		if errors.Is(err, os.ErrNotExist) {
			// If path begins with `/assets`, return 404.
			if strings.HasPrefix(r.URL.Path, "/assets") {
				http.NotFound(w, r)
				return
			}

			// If the file doesn't exist, serve the index.html file as a fallback,
			// and handle routing client side.
			serveIndex(httpFS, w, r)
			return
		}
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		f.Close()
		http.FileServer(httpFS).ServeHTTP(w, r)
	}), nil
}

func serveIndex(httpFS http.FileSystem, w http.ResponseWriter, r *http.Request) {
	indexFile, err := httpFS.Open("index.html")
	if err != nil {
		http.Error(w, "index.html not found", http.StatusInternalServerError)
		return
	}
	defer indexFile.Close()

	http.ServeContent(w, r, "index.html", modTime(indexFile), indexFile)
}

func modTime(f fs.File) time.Time {
	info, err := f.Stat()
	if err != nil {
		return time.Time{}
	}

	return info.ModTime()
}
