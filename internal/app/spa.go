package app

import (
	"io/fs"
	"net/http"
	"path"
	"strings"
)

func (s *Server) serveSPA(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.NotFound(w, r)
		return
	}

	name := strings.TrimPrefix(path.Clean("/"+r.URL.Path), "/")

	if file, err := fs.Stat(s.dist, name); err == nil && !file.IsDir() {
		http.ServeFileFS(w, r, s.dist, name)
		return
	}

	if name != "" && strings.HasPrefix(name, "assets/") {
		http.NotFound(w, r)
		return
	}

	if _, err := fs.Stat(s.dist, "index.html"); err != nil {
		http.NotFound(w, r)
		return
	}
	http.ServeFileFS(w, r, s.dist, "index.html")
}
