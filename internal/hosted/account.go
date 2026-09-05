package hosted

import (
	"net/http"
	"time"
)

func (s *server) deleteUsage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete && r.Method != http.MethodPost {
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
	if err := s.opts.Store.DeleteUsage(r.Context(), user.ID); err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *server) deleteAccount(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete && r.Method != http.MethodPost {
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
	if err := s.opts.Store.DeleteAccount(r.Context(), user.ID); err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	s.setCookie(w, cookieSession, "", -time.Hour)
	s.setCSRFCookie(w, "")
	w.WriteHeader(http.StatusNoContent)
}
