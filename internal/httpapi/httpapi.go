package httpapi

import (
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/rainhuang0220/whereToken/internal/adapter"
	"github.com/rainhuang0220/whereToken/internal/community"
	"github.com/rainhuang0220/whereToken/internal/metric"
	"github.com/rainhuang0220/whereToken/internal/scan"
	"github.com/rainhuang0220/whereToken/internal/webembed"
)

type server struct {
	home        adapter.Home
	adapters    []adapter.Adapter
	mu          sync.Mutex
	last        *scan.Result
	scanning    bool
	offline     bool
	noCommunity bool
	version     string
	comm        *community.Client
}

func NewHTTPServer(addr string, home adapter.Home, offline bool) *http.Server {
	return NewHTTPServerOpts(addr, home, offline, false)
}

func NewHTTPServerOpts(addr string, home adapter.Home, offline, noCommunity bool) *http.Server {
	return NewHTTPServerFull(addr, home, offline, noCommunity, "dev")
}

func NewHTTPServerFull(addr string, home adapter.Home, offline, noCommunity bool, version string) *http.Server {
	return &http.Server{
		Addr:              addr,
		Handler:           NewMuxFull(home, scan.Adapters(offline), noCommunity, version),
		ReadHeaderTimeout: 5 * time.Second,
	}
}

func NewMux(home adapter.Home) http.Handler {
	return NewMuxWith(home, scan.AllAdapters())
}

func NewMuxWith(home adapter.Home, adapters []adapter.Adapter) http.Handler {
	return NewMuxOpts(home, adapters, false)
}

func NewMuxOpts(home adapter.Home, adapters []adapter.Adapter, noCommunity bool) http.Handler {
	return NewMuxFull(home, adapters, noCommunity, "dev")
}

func NewMuxFull(home adapter.Home, adapters []adapter.Adapter, noCommunity bool, version string) http.Handler {
	if version == "" {
		version = "dev"
	}
	s := &server{home: home, adapters: adapters, offline: scan.CloudSkipped(adapters), noCommunity: noCommunity, version: version}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/summary", s.getSummary)
	mux.HandleFunc("/api/scan", s.postScan)
	mux.HandleFunc("/api/community", s.handleCommunity)
	mux.HandleFunc("/v1/community/", func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	})
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
	if !localHost(r) || !localPage(r) {
		http.Error(w, "localhost only", http.StatusForbidden)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	cur, scanning, ok := s.snapshotLast()
	if !ok {
		empty := scan.Result{Errors: []string{}, Summary: metric.Aggregate(nil, nil), Scanning: scanning}
		if err := scan.EncodeSummary(w, empty); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}
	cur.Scanning = scanning
	s.attachCommunity(&cur)
	full := cur
	win, err := metric.ParseSinceQuery(r.URL.Query().Get("since"), time.Now(), time.Local)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if from, to := r.URL.Query().Get("from"), r.URL.Query().Get("to"); from != "" || to != "" {
		win, err = metric.ParseWindow(false, "", from, to, time.Now(), time.Local)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	}
	if !win.IsAll() {
		cur = scan.ApplyWindow(full, win, time.Local)
		cur.Compare = scan.CompareWindows(full, win, time.Local)
	}
	if err := scan.EncodeSummary(w, cur); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *server) postScan(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !localHost(r) || !localPage(r) {
		http.Error(w, "localhost only", http.StatusForbidden)
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
	res.Offline = scan.CloudSkipped(s.adapters)
	s.attachCommunity(&res)
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

func localPage(r *http.Request) bool {
	if origin := strings.TrimSpace(r.Header.Get("Origin")); origin != "" {
		return localOrigin(origin)
	}
	if ref := strings.TrimSpace(r.Header.Get("Referer")); ref != "" {
		return localOrigin(ref)
	}
	return true
}

func localOrigin(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return false
	}
	return localHost(&http.Request{Host: u.Host})
}

func localHost(r *http.Request) bool {
	host := r.Host
	if host == "" {
		return true
	}
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	switch strings.ToLower(host) {
	case "127.0.0.1", "localhost", "::1":
		return true
	}
	return false
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
	if dir := moduleWebDist(); dir != "" {
		return dir
	}
	var candidates []string
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

func moduleWebDist() string {
	wd, err := os.Getwd()
	if err != nil {
		return ""
	}
	if !isWhereTokenGoMod(filepath.Join(wd, "go.mod")) {
		return ""
	}
	cand := filepath.Join(wd, "web", "dist")
	if st, err := os.Stat(cand); err == nil && st.IsDir() {
		return cand
	}
	return ""
}

func isWhereTokenGoMod(path string) bool {
	raw, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if line == "module github.com/rainhuang0220/whereToken" {
			return true
		}
	}
	return false
}
