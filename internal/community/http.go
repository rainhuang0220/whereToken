package community

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

var (
	errInvalidParticipant = errors.New("invalid participant_id")
	errRateLimited        = errors.New("rate limited")
)

const maxUploadBytes = 8 << 10 // 8KiB — aggregates only

// Handler is the remote Community Rank HTTP surface. It does not serve
// local usage, does not store connection IPs, and does not list users.
type Handler struct {
	Store *Store
	ipHit map[string][]time.Time
	mu    sync.Mutex
	now   func() time.Time
}

func NewHandler(store *Store) *Handler {
	if store == nil {
		store = NewStore(DefaultMinParticipants)
	}
	return &Handler{Store: store, ipHit: map[string][]time.Time{}, now: time.Now}
}

func (h *Handler) Mux() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/community/usage", h.putUsage)
	mux.HandleFunc("/v1/community/rank", h.getRank)
	mux.HandleFunc("/v1/community/leave", h.postLeave)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	})
	return mux
}

func (h *Handler) putUsage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !h.allowIP(r) {
		http.Error(w, "rate limited", http.StatusTooManyRequests)
		return
	}
	raw, err := io.ReadAll(io.LimitReader(r.Body, maxUploadBytes+1))
	if err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if len(raw) > maxUploadBytes {
		http.Error(w, "payload too large", http.StatusRequestEntityTooLarge)
		return
	}
	u, err := DecodeUpload(raw)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	now := time.Now()
	if h.now != nil {
		now = h.now()
	}
	if !periodNearNow(u.Period, now) {
		http.Error(w, "period outside allowed window", http.StatusBadRequest)
		return
	}
	if err := h.Store.Put(u); err != nil {
		if errors.Is(err, errRateLimited) {
			http.Error(w, "rate limited", http.StatusTooManyRequests)
			return
		}
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) getRank(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !h.allowIP(r) {
		http.Error(w, "rate limited", http.StatusTooManyRequests)
		return
	}
	id := strings.TrimSpace(r.URL.Query().Get("participant_id"))
	if !uuidRe.MatchString(id) {
		http.Error(w, "invalid participant_id", http.StatusBadRequest)
		return
	}
	period := strings.TrimSpace(r.URL.Query().Get("period"))
	metric := strings.TrimSpace(r.URL.Query().Get("metric"))
	if metric == "" {
		metric = MetricTokens
	}
	if metric != MetricTokens && metric != MetricCost {
		http.Error(w, "invalid metric", http.StatusBadRequest)
		return
	}
	kind := PeriodToday
	date := period
	if period == "" || period == PeriodToday {
		http.Error(w, "period must be a local YYYY-MM-DD or all", http.StatusBadRequest)
		return
	}
	if period == PeriodAll {
		kind = PeriodAll
		date = ""
	} else if !periodRe.MatchString(period) {
		http.Error(w, "invalid period", http.StatusBadRequest)
		return
	}
	st := SanitizeStanding(h.Store.Rank(id, kind, date, metric))
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(st)
}

func periodNearNow(period string, now time.Time) bool {
	d, err := time.Parse("2006-01-02", period)
	if err != nil {
		return false
	}
	utc := now.UTC()
	today := time.Date(utc.Year(), utc.Month(), utc.Day(), 0, 0, 0, 0, time.UTC)
	days := int(today.Sub(d).Hours() / 24)
	if days < 0 {
		days = -days
	}
	return days <= 2
}

func (h *Handler) postLeave(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !h.allowIP(r) {
		http.Error(w, "rate limited", http.StatusTooManyRequests)
		return
	}
	raw, err := io.ReadAll(io.LimitReader(r.Body, maxUploadBytes+1))
	if err != nil || len(raw) > maxUploadBytes {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	var body struct {
		ParticipantID string `json:"participant_id"`
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&body); err != nil {
		http.Error(w, "invalid leave", http.StatusBadRequest)
		return
	}
	if err := h.Store.Leave(body.ParticipantID); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// allowIP is an ephemeral connection throttle. It does not persist IPs and
// does not join them to participant_id.
func (h *Handler) allowIP(r *http.Request) bool {
	ip := r.RemoteAddr
	if host, _, ok := strings.Cut(ip, "]"); ok && strings.HasPrefix(ip, "[") {
		ip = strings.TrimPrefix(host, "[")
	} else if host, _, ok := strings.Cut(ip, ":"); ok {
		ip = host
	}
	if ip == "" {
		return true
	}
	now := h.now()
	if h.now == nil {
		now = time.Now()
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	cut := now.Add(-time.Minute)
	kept := h.ipHit[ip][:0]
	for _, t := range h.ipHit[ip] {
		if t.After(cut) {
			kept = append(kept, t)
		}
	}
	if len(kept) >= 60 {
		h.ipHit[ip] = kept
		return false
	}
	h.ipHit[ip] = append(kept, now)
	return true
}
