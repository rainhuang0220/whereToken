package hosted

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	cookieSession    = "wt_session"
	cookieOAuthState = "wt_oauth"
	cookieCSRF       = "wt_csrf"
	sessionTTL       = 30 * 24 * time.Hour
	oauthTTL         = 10 * time.Minute
)

type GitHubEndpoints struct {
	AuthorizeURL string
	TokenURL     string
	UserURL      string
	HTTPClient   *http.Client
}

func (s *server) github() GitHubEndpoints {
	g := s.opts.GitHub
	if g.AuthorizeURL == "" {
		g.AuthorizeURL = "https://github.com/login/oauth/authorize"
	}
	if g.TokenURL == "" {
		g.TokenURL = "https://github.com/login/oauth/access_token"
	}
	if g.UserURL == "" {
		g.UserURL = "https://api.github.com/user"
	}
	if g.HTTPClient == nil {
		g.HTTPClient = &http.Client{Timeout: 10 * time.Second}
	}
	return g
}

func (s *server) startGitHub(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !s.limit(r, "oauth", 20) {
		http.Error(w, "rate limited", http.StatusTooManyRequests)
		return
	}
	state, err := randomBytes(16)
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	verifier, err := randomBytes(32)
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	stateStr := base64.RawURLEncoding.EncodeToString(state)
	verStr := base64.RawURLEncoding.EncodeToString(verifier)
	sum := sha256.Sum256([]byte(verStr))
	challenge := base64.RawURLEncoding.EncodeToString(sum[:])
	if err := s.opts.Store.PutOAuthState(r.Context(), hashBytes([]byte(stateStr)), hashBytes([]byte(verStr)), s.opts.Now().Add(oauthTTL)); err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	s.setCookie(w, cookieOAuthState, stateStr+":"+verStr, oauthTTL)
	if next := r.URL.Query().Get("next"); strings.HasPrefix(next, "/") && !strings.HasPrefix(next, "//") {
		s.setCookie(w, "wt_next", next, oauthTTL)
	}
	g := s.github()
	q := url.Values{}
	q.Set("client_id", s.opts.Config.GitHubClientID)
	q.Set("redirect_uri", s.opts.Config.PublicURL+"/api/v1/auth/github/callback")
	q.Set("state", stateStr)
	q.Set("code_challenge", challenge)
	q.Set("code_challenge_method", "S256")
	http.Redirect(w, r, g.AuthorizeURL+"?"+q.Encode(), http.StatusFound)
}

func (s *server) callbackGitHub(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	qState := r.URL.Query().Get("state")
	code := r.URL.Query().Get("code")
	c, _ := r.Cookie(cookieOAuthState)
	if c == nil || qState == "" || code == "" {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	parts := strings.SplitN(c.Value, ":", 2)
	if len(parts) != 2 || parts[0] != qState {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	verifier := parts[1]
	if err := s.opts.Store.TakeOAuthState(r.Context(), hashBytes([]byte(qState)), hashBytes([]byte(verifier))); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	s.setCookie(w, cookieOAuthState, "", -time.Hour)
	tok, err := s.exchangeGitHub(r.Context(), code, verifier)
	if err != nil {
		http.Error(w, "oauth exchange failed", http.StatusBadGateway)
		return
	}
	ident, err := s.fetchGitHubUser(r.Context(), tok)
	tok = "" // discard
	if err != nil {
		http.Error(w, "oauth profile failed", http.StatusBadGateway)
		return
	}
	user, err := s.opts.Store.UpsertGitHubUser(r.Context(), ident)
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	raw, sess, err := s.opts.Store.CreateSession(r.Context(), user.ID, sessionTTL)
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	s.setCookie(w, cookieSession, raw, sessionTTL)
	s.setCSRFCookie(w, sess.CSRFToken)
	next := "/app"
	if c, err := r.Cookie("wt_next"); err == nil && strings.HasPrefix(c.Value, "/") && !strings.HasPrefix(c.Value, "//") {
		next = c.Value
	}
	s.setCookie(w, "wt_next", "", -time.Hour)
	http.Redirect(w, r, s.opts.Config.PublicURL+next, http.StatusFound)
}

func (s *server) exchangeGitHub(ctx context.Context, code, verifier string) (string, error) {
	g := s.github()
	form := url.Values{}
	form.Set("client_id", s.opts.Config.GitHubClientID)
	form.Set("client_secret", s.opts.Config.GitHubClientSecret)
	form.Set("code", code)
	form.Set("redirect_uri", s.opts.Config.PublicURL+"/api/v1/auth/github/callback")
	form.Set("code_verifier", verifier)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, g.TokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	res, err := g.HTTPClient.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(res.Body, 1<<16))
	if err != nil {
		return "", err
	}
	var out struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(raw, &out); err != nil || out.AccessToken == "" {
		return "", fmt.Errorf("github token")
	}
	return out.AccessToken, nil
}

func (s *server) fetchGitHubUser(ctx context.Context, token string) (GitHubIdentity, error) {
	g := s.github()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, g.UserURL, nil)
	if err != nil {
		return GitHubIdentity{}, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")
	res, err := g.HTTPClient.Do(req)
	if err != nil {
		return GitHubIdentity{}, err
	}
	defer res.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(res.Body, 1<<16))
	if err != nil {
		return GitHubIdentity{}, err
	}
	var u struct {
		ID        int64  `json:"id"`
		Login     string `json:"login"`
		AvatarURL string `json:"avatar_url"`
	}
	if err := json.Unmarshal(raw, &u); err != nil || u.ID == 0 || u.Login == "" {
		return GitHubIdentity{}, fmt.Errorf("github user")
	}
	return GitHubIdentity{ID: u.ID, Login: u.Login, AvatarURL: u.AvatarURL}, nil
}

func (s *server) getSession(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	u, sess, err := s.currentUser(r)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	csrf := ""
	if c, err := r.Cookie(cookieCSRF); err == nil && sess.ValidCSRF(c.Value) {
		csrf = c.Value
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"login":      u.Login,
		"avatar_url": u.AvatarURL,
		"public_id":  u.PublicID,
		"csrf":       csrf,
	})
}

func (sess Session) ValidCSRF(token string) bool {
	if token == "" || len(sess.CSRFHash) == 0 {
		return false
	}
	return hmacEqual(sess.CSRFHash, hashBytes([]byte(token)))
}

func (s *server) requireCSRF(w http.ResponseWriter, r *http.Request, sess Session) bool {
	token := r.Header.Get("X-CSRF-Token")
	if token == "" {
		if c, err := r.Cookie(cookieCSRF); err == nil {
			token = c.Value
		}
	}
	if !sess.ValidCSRF(token) {
		http.Error(w, "csrf", http.StatusForbidden)
		return false
	}
	return true
}

func (s *server) logout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if _, sess, err := s.currentUser(r); err == nil {
		if !s.requireCSRF(w, r, sess) {
			return
		}
	}
	if c, err := r.Cookie(cookieSession); err == nil {
		_ = s.opts.Store.DeleteSession(r.Context(), c.Value)
	}
	s.setCookie(w, cookieSession, "", -time.Hour)
	s.setCSRFCookie(w, "")
	w.WriteHeader(http.StatusNoContent)
}

func (s *server) currentUser(r *http.Request) (User, Session, error) {
	c, err := r.Cookie(cookieSession)
	if err != nil || s.opts.Store == nil {
		return User{}, Session{}, ErrUnauthorized
	}
	sess, err := s.opts.Store.LookupSession(r.Context(), c.Value)
	if err != nil {
		return User{}, Session{}, ErrUnauthorized
	}
	u, err := s.opts.Store.UserByID(r.Context(), sess.UserID)
	if err != nil {
		return User{}, Session{}, ErrUnauthorized
	}
	return u, sess, nil
}

func (s *server) setCookie(w http.ResponseWriter, name, value string, ttl time.Duration) {
	c := &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     "/",
		HttpOnly: true,
		Secure:   s.opts.Config.CookieSecure,
		SameSite: http.SameSiteLaxMode,
	}
	if ttl < 0 {
		c.MaxAge = -1
	} else {
		c.Expires = s.opts.Now().Add(ttl)
	}
	http.SetCookie(w, c)
}

func (s *server) setCSRFCookie(w http.ResponseWriter, value string) {
	c := &http.Cookie{
		Name:     cookieCSRF,
		Value:    value,
		Path:     "/",
		HttpOnly: false,
		Secure:   s.opts.Config.CookieSecure,
		SameSite: http.SameSiteLaxMode,
	}
	if value == "" {
		c.MaxAge = -1
	} else {
		c.Expires = s.opts.Now().Add(sessionTTL)
	}
	http.SetCookie(w, c)
}
