package httpapi

import (
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/rainhuang0220/whereToken/internal/adapter"
	"github.com/rainhuang0220/whereToken/internal/scan"
	"github.com/rainhuang0220/whereToken/internal/webembed"
)

type server struct {
	home     adapter.Home
	adapters []adapter.Adapter
	mu       sync.Mutex
	last     *scan.Result
	scanning bool
}

func NewHTTPServer(addr string, home adapter.Home, offline bool) *http.Server {
	return &http.Server{
		Addr:              addr,
		Handler:           NewMuxWith(home, scan.Adapters(offline)),
		ReadHeaderTimeout: 5 * time.Second,
	}
}

func NewMux(home adapter.Home) http.Handler {
	return NewMuxWith(home, scan.AllAdapters())
}

func NewMuxWith(home adapter.Home, adapters []adapter.Adapter) http.Handler {
	s := &server{home: home, adapters: adapters}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/summary", s.getSummary)
	mux.HandleFunc("/api/scan", s.postScan)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if dir := webDist(); dir != "" {
			serveWeb(w, r, dir)
			return
		}
		if fsys, ok := webembed.FS(); ok {
			serveFS(w, r, fsys)
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		io.WriteString(w, "whereToken")
	})
	return withSafeHeaders(mux)
}

func withSafeHeaders(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		h.ServeHTTP(w, r)
	})
}

func (s *server) getSummary(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	s.mu.Lock()
	last := s.last
	s.mu.Unlock()
	if last == nil {
		if err := scan.EncodeSummary(w, scan.Result{Errors: []string{}}); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}
	if err := scan.EncodeSummary(w, *last); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *server) postScan(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !s.beginScan() {
		http.Error(w, "煅烧进行中", http.StatusConflict)
		return
	}
	defer s.endScan()

	stream := strings.Contains(r.Header.Get("Accept"), "text/event-stream")
	var flush http.Flusher
	if stream {
		fl, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "stream unsupported", http.StatusInternalServerError)
			return
		}
		flush = fl
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Connection", "keep-alive")
		w.WriteHeader(http.StatusOK)
		flush.Flush()
	}

	var report func(scan.Progress)
	if stream {
		report = func(p scan.Progress) {
			raw, err := json.Marshal(p)
			if err != nil {
				return
			}
			writeSSE(w, flush, "progress", string(raw))
		}
	}
	res := scan.RunWithProgress(s.home, s.adapters, report)
	s.mu.Lock()
	cp := res
	s.last = &cp
	s.mu.Unlock()

	if stream {
		raw, err := scan.MarshalSummary(res)
		if err != nil {
			writeSSE(w, flush, "error", `{"error":"encode"}`)
			return
		}
		writeSSE(w, flush, "complete", string(raw))
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if err := scan.EncodeSummary(w, res); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func writeSSE(w http.ResponseWriter, flush http.Flusher, event, data string) {
	fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, data)
	flush.Flush()
}

func (s *server) beginScan() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.scanning {
		return false
	}
	s.scanning = true
	return true
}

func (s *server) endScan() {
	s.mu.Lock()
	s.scanning = false
	s.mu.Unlock()
}

func serveFS(w http.ResponseWriter, r *http.Request, fsys fs.FS) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	rel := path.Clean("/" + r.URL.Path)
	name := strings.TrimPrefix(rel, "/")
	if name == "" || rel == "/" {
		name = "index.html"
	}
	f, err := fsys.Open(name)
	if err != nil {
		if path.Ext(rel) != "" {
			http.NotFound(w, r)
			return
		}
		name = "index.html"
		f, err = fsys.Open(name)
		if err != nil {
			http.NotFound(w, r)
			return
		}
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil || st.IsDir() {
		http.NotFound(w, r)
		return
	}
	rs, ok := f.(io.ReadSeeker)
	if !ok {
		http.Error(w, "unsupported file", http.StatusInternalServerError)
		return
	}
	http.ServeContent(w, r, name, time.Time{}, rs)
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
	srv := NewHTTPServer(addr, home, false)
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
