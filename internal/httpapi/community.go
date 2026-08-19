package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"time"

	"github.com/rainhuang0220/whereToken/internal/community"
	"github.com/rainhuang0220/whereToken/internal/scan"
)

func osLookup(k string) string { return os.Getenv(k) }

func (s *server) snapshotLast() (scan.Result, bool, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.last == nil {
		return scan.Result{}, s.scanning, false
	}
	cur := *s.last
	if s.last.Community != nil {
		v := *s.last.Community
		cur.Community = &v
	}
	return cur, s.scanning, true
}

func (s *server) attachCommunity(res *scan.Result) {
	if res == nil || res.Community != nil {
		return
	}
	req := community.Request{
		Home:    s.home,
		Getenv:  osLookup,
		Offline: s.offline || res.Offline,
		OptOut:  s.noCommunity,
		Version: s.version,
		Now:     time.Now(),
		Loc:     time.Local,
	}
	if req.OptOut || req.Offline || community.EnvDisabled(req.Getenv) || community.EnvURL(req.Getenv) == "" {
		v := community.Resolve(req, res.Events)
		res.Community = &v
		return
	}
	c, err := s.ensureClient()
	if err != nil {
		v := community.EmptyView(community.StatusUnavailable, community.DisclaimerEN)
		res.Community = &v
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	s.commMu.Lock()
	c.Offline = req.Offline
	v := c.Sync(ctx, res.Events, req.Now, req.Loc)
	s.commMu.Unlock()
	res.Community = &v
}

func (s *server) ensureClient() (*community.Client, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.comm != nil {
		return s.comm, nil
	}
	path := community.ConfigPath(s.home)
	f, err := community.LoadOrCreate(path, time.Now().Format("2006-01-02"))
	if err != nil {
		return nil, err
	}
	ver := s.version
	if ver == "" {
		ver = "dev"
	}
	s.comm = &community.Client{
		BaseURL: community.EnvURL(osLookup),
		File:    f,
		Path:    path,
		Offline: s.offline,
		Version: ver,
	}
	return s.comm, nil
}

func (s *server) handleCommunity(w http.ResponseWriter, r *http.Request) {
	if !localHost(r) || !localPage(r) {
		http.Error(w, "localhost only", http.StatusForbidden)
		return
	}
	switch r.Method {
	case http.MethodGet:
		s.getCommunity(w, r)
	case http.MethodPost:
		s.postCommunity(w, r)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

type communityJSON struct {
	Enabled bool               `json:"enabled"`
	Note    string             `json:"note"`
	Today   community.Standing `json:"today"`
	All     community.Standing `json:"all"`
}

func (s *server) getCommunity(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	cur, _, ok := s.snapshotLast()
	var snap *community.View
	if ok && cur.Community != nil {
		snap = cur.Community
	}
	if snap != nil {
		_ = json.NewEncoder(w).Encode(communityJSON{
			Enabled: snap.Enabled,
			Note:    snap.Note,
			Today:   snap.Today,
			All:     snap.All,
		})
		return
	}
	urlSet := community.EnvURL(osLookup) != ""
	enabled := urlSet
	if f, err := community.Load(community.ConfigPath(s.home)); err == nil {
		enabled = enabled && f.Enabled
	}
	if s.noCommunity || community.EnvDisabled(osLookup) {
		enabled = false
	}
	status := community.StatusUnavailable
	note := community.DisclaimerEN
	if !urlSet {
		status = community.StatusServiceUnconfigured
		note = "Community Rank service is not configured."
	} else if s.noCommunity || community.EnvDisabled(osLookup) {
		status = community.StatusOptedOut
	}
	v := community.EmptyView(status, note)
	v.Enabled = enabled
	_ = json.NewEncoder(w).Encode(communityJSON{Enabled: enabled, Note: v.Note, Today: v.Today, All: v.All})
}

func (s *server) postCommunity(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Enabled *bool `json:"enabled"`
	}
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&body); err != nil || body.Enabled == nil {
		http.Error(w, "invalid community settings", http.StatusBadRequest)
		return
	}
	c, err := s.ensureClient()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.commMu.Lock()
	if !*body.Enabled {
		ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
		_ = c.Leave(ctx)
		cancel()
	}
	err = c.File.SetEnabled(c.Path, *body.Enabled)
	s.commMu.Unlock()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.mu.Lock()
	if s.last != nil {
		s.last.Community = nil
	}
	s.mu.Unlock()
	s.getCommunity(w, r)
}
