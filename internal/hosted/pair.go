package hosted

import (
	"crypto/hmac"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

const pairAlphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"

type pairPending struct {
	rawToken string
	login    string
	deviceID string
	hmacKey  []byte
}

func (s *server) pairStart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !s.limit(r, "pair-start", 10) {
		http.Error(w, "rate limited", http.StatusTooManyRequests)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 8<<10)
	var in struct {
		OS            string `json:"os"`
		Arch          string `json:"arch"`
		ClientVersion string `json:"client_version"`
		Label         string `json:"label"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil && err.Error() != "EOF" {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	code, err := s.opts.Store.newDisplayCode(r.Context())
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	secret, secretHash, err := newOpaqueToken()
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	meta := DeviceMeta{Label: in.Label, OS: in.OS, Arch: in.Arch, ClientVersion: in.ClientVersion}
	exp := s.opts.Now().Add(10 * time.Minute)
	if err := s.opts.Store.CreatePairChallenge(r.Context(), code, secretHash, meta, exp); err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"display_code":     formatCode(code),
		"device_secret":    secret,
		"expires_at":       exp.UTC().Format(time.RFC3339),
		"verification_url": s.opts.Config.PublicURL + "/pair/" + formatCode(code),
	})
}

func (s *server) pairStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !s.limit(r, "pair-status", 30) {
		http.Error(w, "rate limited", http.StatusTooManyRequests)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 8<<10)
	var in struct {
		DisplayCode  string `json:"display_code"`
		DeviceSecret string `json:"device_secret"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	code := normalizeCode(in.DisplayCode)
	ch, err := s.opts.Store.GetPairChallenge(r.Context(), code)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if !hmac.Equal(ch.SecretHash, hashBytes([]byte(in.DeviceSecret))) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	now := s.opts.Now()
	switch {
	case ch.DeniedAt.Valid:
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "denied"})
	case !ch.ExpiresAt.After(now) && !ch.ConsumedAt.Valid:
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "expired"})
	case ch.ConsumedAt.Valid:
		if v, ok := s.takePending(code); ok {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status":          "approved",
				"device_token":    v.rawToken,
				"user_login":      v.login,
				"device_id":       v.deviceID,
				"source_hmac_key": base64.RawURLEncoding.EncodeToString(v.hmacKey),
			})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "consumed"})
	default:
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "pending"})
	}
}

func (s *server) getPairChallenge(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if _, _, err := s.currentUser(r); err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	code := normalizeCode(r.URL.Query().Get("code"))
	if code == "" {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	ch, err := s.opts.Store.GetPairChallenge(r.Context(), code)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"display_code":   formatCode(code),
		"label":          ch.Meta.Label,
		"os":             ch.Meta.OS,
		"arch":           ch.Meta.Arch,
		"client_version": ch.Meta.ClientVersion,
		"expires_at":     ch.ExpiresAt.UTC().Format(time.RFC3339),
		"consumed":       ch.ConsumedAt.Valid,
		"denied":         ch.DeniedAt.Valid,
	})
}

func (s *server) pairConfirm(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	user, sess, err := s.currentUser(r)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if !s.requireCSRF(w, r, sess) {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 8<<10)
	var in struct {
		DisplayCode string `json:"display_code"`
		Accept      bool   `json:"accept"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	code := normalizeCode(in.DisplayCode)
	ch, err := s.opts.Store.GetPairChallenge(r.Context(), code)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	if !ch.ExpiresAt.After(s.opts.Now()) || ch.ConsumedAt.Valid || ch.DeniedAt.Valid {
		http.Error(w, "expired", http.StatusGone)
		return
	}
	if !in.Accept {
		if err := s.opts.Store.DenyPair(r.Context(), code, user.ID); err != nil {
			http.Error(w, "server error", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		return
	}
	dev, raw, err := s.opts.Store.InsertDevice(r.Context(), user.ID, ch.Meta)
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	if err := s.opts.Store.ConsumePair(r.Context(), code, user.ID); err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	s.putPending(code, pairPending{rawToken: raw, login: user.Login, deviceID: dev.PublicID, hmacKey: user.SourceHMACKey})
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"status":    "approved",
		"device_id": dev.PublicID,
		"label":     dev.Label,
	})
}

func (s *server) putPending(code string, p pairPending) {
	s.pending.Store(code, p)
}

func (s *server) takePending(code string) (pairPending, bool) {
	v, ok := s.pending.LoadAndDelete(code)
	if !ok {
		return pairPending{}, false
	}
	return v.(pairPending), true
}

func formatCode(code string) string {
	code = normalizeCode(code)
	if len(code) == 8 {
		return code[:4] + "-" + code[4:]
	}
	return code
}

func normalizeCode(code string) string {
	return strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(code), "-", ""))
}

func (s *server) limit(r *http.Request, bucket string, perMin int) bool {
	ip := r.RemoteAddr
	if i := strings.LastIndex(ip, ":"); i >= 0 {
		ip = ip[:i]
	}
	key := bucket + "\x00" + ip
	now := s.opts.Now()
	s.limMu.Lock()
	defer s.limMu.Unlock()
	hits := s.limiter[key]
	var kept []time.Time
	cut := now.Add(-time.Minute)
	for _, ts := range hits {
		if ts.After(cut) {
			kept = append(kept, ts)
		}
	}
	if len(kept) >= perMin {
		s.limiter[key] = kept
		return false
	}
	s.limiter[key] = append(kept, now)
	return true
}
