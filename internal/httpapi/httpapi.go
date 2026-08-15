package httpapi

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/rainhuang0220/whereToken/internal/adapter"
	"github.com/rainhuang0220/whereToken/internal/scan"
)

func NewMux(home adapter.Home) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/summary", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		res := scan.Run(home, scan.AllAdapters())
		if err := scan.EncodeSummary(w, res); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if dir := webDist(); dir != "" {
			serveWeb(w, r, dir)
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		io.WriteString(w, "whereToken")
	})
	return mux
}

func serveWeb(w http.ResponseWriter, r *http.Request, dir string) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	rel := path.Clean("/" + r.URL.Path)
	full := filepath.Join(dir, filepath.FromSlash(strings.TrimPrefix(rel, "/")))
	if !insideDir(dir, full) {
		http.NotFound(w, r)
		return
	}
	if rel == "/" {
		http.ServeFile(w, r, filepath.Join(dir, "index.html"))
		return
	}
	st, err := os.Stat(full)
	if err == nil && !st.IsDir() {
		http.ServeFile(w, r, full)
		return
	}
	if path.Ext(rel) != "" {
		http.NotFound(w, r)
		return
	}
	http.ServeFile(w, r, filepath.Join(dir, "index.html"))
}

func insideDir(dir, target string) bool {
	rel, err := filepath.Rel(dir, target)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator))
}

func Listen(addr string, home adapter.Home) error {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return err
	}
	if host != "127.0.0.1" && host != "localhost" {
		return fmt.Errorf("refusing non-localhost bind %s", addr)
	}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	srv := &http.Server{Addr: addr, Handler: NewMux(home)}
	return srv.Serve(ln)
}

func webDist() string {
	if v := os.Getenv("WHERETOKEN_WEB"); v != "" {
		if st, err := os.Stat(v); err == nil && st.IsDir() {
			return v
		}
	}
	candidates := []string{filepath.Join("web", "dist")}
	if exe, err := os.Executable(); err == nil {
		candidates = append(candidates, filepath.Join(filepath.Dir(exe), "web", "dist"))
	}
	for _, p := range candidates {
		if st, err := os.Stat(p); err == nil && st.IsDir() {
			return p
		}
	}
	return ""
}
