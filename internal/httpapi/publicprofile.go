package httpapi

import (
	"encoding/json"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/rainhuang0220/whereToken/internal/profilewebembed"
	"github.com/rainhuang0220/whereToken/internal/publicprofile"
)

func (s *server) publicProfile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !localHost(r) || !localPage(r) {
		http.Error(w, "localhost only", http.StatusForbidden)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	path := publicprofile.ConfigPath(s.home)
	owner := publicprofile.FallbackOwner(path)
	fail := ""
	if r.Method == http.MethodPost {
		var body struct {
			PublicPalette string `json:"public_palette"`
		}
		dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&body); err != nil {
			http.Error(w, "invalid public palette", http.StatusBadRequest)
			return
		}
		if err := publicprofile.ValidatePalette(body.PublicPalette); err != nil {
			http.Error(w, "invalid public palette", http.StatusBadRequest)
			return
		}
		if err := owner.ApplyPalette(body.PublicPalette, time.Now().UTC().Format(time.RFC3339)); err != nil {
			http.Error(w, "invalid public palette", http.StatusBadRequest)
			return
		}
		if dir := publicprofile.ResolveBundleDir(owner); dir != "" {
			if abs, err := filepath.Abs(dir); err == nil {
				owner.BundleDir = abs
			} else {
				owner.BundleDir = dir
			}
		}
		if err := publicprofile.SaveOwner(path, owner); err != nil {
			fail = "could not save the public palette"
		} else if owner.BundleDir != "" {
			snap, err := publicprofile.LoadFile(owner.BundleDir)
			if err != nil {
				fail = "could not read the local public bundle"
			} else if err := publicprofile.WriteBundle(owner.BundleDir, snap, owner.Presentation()); err != nil {
				fail = "could not write the local public bundle"
			}
		}
	}
	dir := publicprofile.ResolveBundleDir(owner)
	var bundle publicprofile.Presentation
	have := false
	if dir != "" {
		have = true
		if got, err := publicprofile.ReadBundlePresentation(dir); err == nil {
			bundle = got
		} else {
			bundle = publicprofile.DefaultPresentation()
		}
	}
	view := publicprofile.DescribePublication(owner, dir, bundle, have, fail)
	if err := json.NewEncoder(w).Encode(view); err != nil {
		http.Error(w, "could not encode public palette", http.StatusInternalServerError)
	}
}

func (s *server) newsprintSurface(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !localHost(r) || !localPage(r) {
		http.Error(w, "localhost only", http.StatusForbidden)
		return
	}
	raw, err := profilewebembed.Read("assets/newsprint-surface.jpg")
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "image/jpeg")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(raw)
}

func (s *server) previewPublicProfile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !localHost(r) || !localPage(r) {
		http.Error(w, "localhost only", http.StatusForbidden)
		return
	}
	owner := publicprofile.FallbackOwner(publicprofile.ConfigPath(s.home))
	dir := publicprofile.ResolveBundleDir(owner)
	if dir == "" {
		http.NotFound(w, r)
		return
	}
	rel := strings.TrimPrefix(r.URL.Path, "/preview/public-profile")
	rel = strings.TrimPrefix(rel, "/")
	if rel == "" {
		rel = "index.html"
	}
	rel = path.Clean("/" + rel)
	if rel == "/" {
		rel = "/index.html"
	}
	full := filepath.Join(dir, filepath.FromSlash(strings.TrimPrefix(rel, "/")))
	if !insideDir(dir, full) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	if st, err := os.Stat(full); err != nil || st.IsDir() {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	http.ServeFile(w, r, full)
}
