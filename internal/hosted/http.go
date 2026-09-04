package hosted

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"
)

type MuxOptions struct {
	Version string
	Store   *Store
	Config  Config
	Now     func() time.Time
	GitHub  GitHubEndpoints
}

func NewMux(opts MuxOptions) http.Handler {
	if opts.Version == "" {
		opts.Version = "dev"
	}
	if opts.Now == nil {
		opts.Now = time.Now
	}
	s := &server{opts: opts, limiter: map[string][]time.Time{}}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/health", s.getHealth)
	mux.HandleFunc("/api/v1/auth/github", s.startGitHub)
	mux.HandleFunc("/api/v1/auth/github/callback", s.callbackGitHub)
	mux.HandleFunc("/api/v1/auth/logout", s.logout)
	mux.HandleFunc("/api/v1/session", s.getSession)
	mux.HandleFunc("/api/v1/pair/start", s.pairStart)
	mux.HandleFunc("/api/v1/pair/status", s.pairStatus)
	mux.HandleFunc("/api/v1/pair/confirm", s.pairConfirm)
	mux.HandleFunc("/api/v1/devices/self/revoke", s.revokeSelf)
	mux.HandleFunc("/api/v1/devices/", s.deviceRoutes)
	mux.HandleFunc("/api/v1/sync/batch", s.putSyncBatch)
	mux.HandleFunc("/api/v1/dashboard/summary", s.getDashboard)
	return withSecurityHeaders(mux)
}

type server struct {
	opts    MuxOptions
	pending sync.Map
	limiter map[string][]time.Time
	limMu   sync.Mutex
}

func withSecurityHeaders(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; img-src 'self' data: https://avatars.githubusercontent.com; style-src 'self' 'unsafe-inline'; script-src 'self'")
		h.ServeHTTP(w, r)
	})
}

func (s *server) getHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(map[string]string{
		"status":  "ok",
		"version": s.opts.Version,
	})
}
