package httpapi

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/rainhuang0220/whereToken/internal/publicprofile"
)

type profilePublisher interface {
	Preflight(ctx context.Context, palette string) (publicprofile.Preflight, error)
	Approve(ctx context.Context, palette string, report func(publicprofile.Job)) (publicprofile.Job, error)
	RetryReadme(ctx context.Context, palette string, report func(publicprofile.Job)) (publicprofile.Job, error)
}

type publishTicket struct {
	ID       string
	CSRF     string
	Palette  string
	CacheKey string
	Expires  time.Time
}

func (s *server) publisherOrNew() profilePublisher {
	s.publishMu.Lock()
	defer s.publishMu.Unlock()
	if s.publisher != nil {
		return s.publisher
	}
	return publicprofile.NewPublisher(s.home)
}

func (s *server) publicProfilePublish(w http.ResponseWriter, r *http.Request) {
	if !localHost(r) {
		http.Error(w, "localhost only", http.StatusForbidden)
		return
	}
	switch r.Method {
	case http.MethodGet:
		s.publishPreflight(w, r)
	case http.MethodPost:
		s.publishExecute(w, r)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *server) publicProfilePublishJob(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !localHost(r) {
		http.Error(w, "localhost only", http.StatusForbidden)
		return
	}
	s.publishMu.Lock()
	job := s.publishJob
	s.publishMu.Unlock()
	if job == nil {
		http.Error(w, "no publish job", http.StatusNotFound)
		return
	}
	writePublishJSON(w, job)
}

func (s *server) publishPreflight(w http.ResponseWriter, r *http.Request) {
	palette := strings.TrimSpace(r.URL.Query().Get("palette"))
	if palette == "" {
		palette = publicprofile.DefaultPalette
	}
	if err := publicprofile.ValidatePalette(palette); err != nil {
		http.Error(w, "invalid public palette", http.StatusBadRequest)
		return
	}
	pf, err := s.publisherOrNew().Preflight(r.Context(), palette)
	if err != nil {
		http.Error(w, "could not prepare publish", http.StatusBadRequest)
		return
	}
	token := newPublishToken()
	s.publishMu.Lock()
	if s.tickets == nil {
		s.tickets = map[string]publishTicket{}
	}
	s.tickets[pf.ID] = publishTicket{
		ID: pf.ID, CSRF: token, Palette: pf.Palette, CacheKey: pf.CacheKey,
		Expires: time.Now().Add(20 * time.Minute),
	}
	s.publishMu.Unlock()
	writePublishJSON(w, struct {
		publicprofile.Preflight
		CSRF string `json:"csrf"`
	}{Preflight: pf, CSRF: token})
}

func (s *server) publishExecute(w http.ResponseWriter, r *http.Request) {
	if !publishPostAllowed(r) {
		http.Error(w, "localhost only", http.StatusForbidden)
		return
	}
	var body struct {
		Action      string `json:"action"`
		PreflightID string `json:"preflight_id"`
		CSRF        string `json:"csrf"`
		Palette     string `json:"palette"`
	}
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&body); err != nil {
		http.Error(w, "invalid publish request", http.StatusBadRequest)
		return
	}
	if err := publicprofile.ValidatePalette(body.Palette); err != nil {
		http.Error(w, "invalid public palette", http.StatusBadRequest)
		return
	}
	if body.CSRF == "" || body.CSRF != r.Header.Get("X-WhereToken-CSRF") {
		http.Error(w, "localhost only", http.StatusForbidden)
		return
	}
	s.publishMu.Lock()
	if s.publishRun {
		job := s.publishJob
		s.publishMu.Unlock()
		if job == nil {
			job = &publicprofile.Job{Phase: publicprofile.PhaseUpdatingBundle, PhaseLabel: publicprofile.PhaseLabel(publicprofile.PhaseUpdatingBundle), Palette: body.Palette}
		}
		writePublishJSON(w, job)
		return
	}
	ticket, ok := s.tickets[body.PreflightID]
	if !ok || time.Now().After(ticket.Expires) || ticket.CSRF != body.CSRF || ticket.Palette != body.Palette {
		s.publishMu.Unlock()
		http.Error(w, "localhost only", http.StatusForbidden)
		return
	}
	delete(s.tickets, body.PreflightID)
	s.publishRun = true
	s.publishMu.Unlock()

	pub := s.publisherOrNew()
	report := func(job publicprofile.Job) {
		s.publishMu.Lock()
		s.publishJob = &job
		s.publishMu.Unlock()
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	go func() {
		defer cancel()
		var job publicprofile.Job
		var err error
		if body.Action == "retry_readme" {
			job, err = pub.RetryReadme(ctx, body.Palette, report)
		} else if body.Action == "approve" {
			job, err = pub.Approve(ctx, body.Palette, report)
		} else {
			job = publicprofile.Job{Phase: publicprofile.PhaseFailed, PhaseLabel: publicprofile.PhaseLabel(publicprofile.PhaseFailed), Palette: body.Palette, Error: "未知动作"}
			err = errUnknownPublish
		}
		if err != nil && job.Error == "" {
			job.Error = publicprofile.Redact(err.Error())
		}
		s.publishMu.Lock()
		s.publishJob = &job
		s.publishRun = false
		s.publishMu.Unlock()
	}()
	writePublishJSON(w, publicprofile.Job{
		Phase: publicprofile.PhaseUpdatingBundle, PhaseLabel: publicprofile.PhaseLabel(publicprofile.PhaseUpdatingBundle),
		Palette: body.Palette,
	})
}

var errUnknownPublish = errString("unknown publish action")

type errString string

func (e errString) Error() string { return string(e) }

func publishPostAllowed(r *http.Request) bool {
	if !localHost(r) {
		return false
	}
	if site := strings.TrimSpace(r.Header.Get("Sec-Fetch-Site")); site != "" && site != "same-origin" {
		return false
	}
	origin := strings.TrimSpace(r.Header.Get("Origin"))
	if origin == "" {
		return false
	}
	u, err := url.Parse(origin)
	if err != nil || !strings.EqualFold(u.Host, r.Host) {
		return false
	}
	return true
}

func writePublishJSON(w http.ResponseWriter, body any) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(body)
}

func newPublishToken() string {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return hex.EncodeToString([]byte(time.Now().UTC().Format(time.RFC3339Nano)))
	}
	return hex.EncodeToString(b[:])
}
