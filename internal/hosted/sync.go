package hosted

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/rainhuang0220/whereToken/internal/syncagg"
)

func (s *server) putSyncBatch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut && r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !s.limit(r, "sync", 30) {
		http.Error(w, "rate limited", http.StatusTooManyRequests)
		return
	}
	dev, user, err := s.deviceFromBearer(r)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 2<<20)
	var raw json.RawMessage
	if err := json.NewDecoder(r.Body).Decode(&raw); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	batch, err := syncagg.DecodeBatch(raw)
	if err != nil {
		http.Error(w, "invalid schema", http.StatusBadRequest)
		return
	}
	if batch.DeviceID != "" && batch.DeviceID != dev.PublicID {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	batch.DeviceID = dev.PublicID
	if len(batch.DailyModelUsage) > 20000 || len(batch.Sources) > 32 {
		http.Error(w, "payload too large", http.StatusRequestEntityTooLarge)
		return
	}
	if err := s.opts.Store.SyncBatch(r.Context(), user.ID, dev.ID, batch); err != nil {
		switch err {
		case ErrStaleRevision:
			http.Error(w, "stale revision", http.StatusConflict)
		case ErrRevisionClash, ErrIdempotency:
			http.Error(w, err.Error(), http.StatusConflict)
		default:
			http.Error(w, "server error", http.StatusInternalServerError)
		}
		return
	}
	_ = s.opts.Store.TouchSync(r.Context(), dev.ID)
	w.WriteHeader(http.StatusOK)
}

func (s *server) deviceRoutes(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/devices/")
	if strings.HasSuffix(path, "/revoke") && r.Method == http.MethodPost {
		id := strings.TrimSuffix(path, "/revoke")
		id = strings.Trim(id, "/")
		user, sess, err := s.currentUser(r)
		if err != nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if !s.requireCSRF(w, r, sess) {
			return
		}
		if err := s.opts.Store.RevokeDevice(r.Context(), user.ID, id); err != nil {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusNoContent)
		return
	}
	http.NotFound(w, r)
}

func (s *server) revokeSelf(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	dev, _, err := s.deviceFromBearer(r)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if err := s.opts.Store.RevokeDevice(r.Context(), dev.UserID, dev.PublicID); err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *server) deviceFromBearer(r *http.Request) (Device, User, error) {
	h := r.Header.Get("Authorization")
	if !strings.HasPrefix(h, "Bearer ") {
		return Device{}, User{}, ErrUnauthorized
	}
	return s.opts.Store.DeviceByToken(r.Context(), strings.TrimPrefix(h, "Bearer "))
}
