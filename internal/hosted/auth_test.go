package hosted

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func testMux(t *testing.T, gh http.Handler) http.Handler {
	t.Helper()
	st := readyStore(t)
	ghSrv := httptest.NewServer(gh)
	t.Cleanup(ghSrv.Close)
	cfg := Config{
		Listen:             "127.0.0.1:0",
		GitHubClientID:     "cid",
		GitHubClientSecret: "csecret",
		PublicURL:          "https://wheretoken.plainlist.space",
		CookieSecure:       false,
	}
	return NewMux(MuxOptions{
		Version: "0.7.0",
		Store:   st,
		Config:  cfg,
		GitHub: GitHubEndpoints{
			AuthorizeURL: ghSrv.URL + "/login/oauth/authorize",
			TokenURL:     ghSrv.URL + "/login/oauth/access_token",
			UserURL:      ghSrv.URL + "/user",
			HTTPClient:   ghSrv.Client(),
		},
	})
}

func TestOAuthStartRedirectsWithState(t *testing.T) {
	h := testMux(t, http.NotFoundHandler())
	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/github", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusFound {
		t.Fatalf("status %d", rec.Code)
	}
	loc := rec.Header().Get("Location")
	u, err := url.Parse(loc)
	if err != nil {
		t.Fatal(err)
	}
	if u.Query().Get("client_id") != "cid" {
		t.Fatalf("client_id %s", loc)
	}
	if u.Query().Get("scope") != "" {
		t.Fatalf("must not request scopes: %s", loc)
	}
	if u.Query().Get("state") == "" || u.Query().Get("code_challenge") == "" {
		t.Fatalf("missing PKCE/state: %s", loc)
	}
	if rec.Header().Get("Set-Cookie") == "" {
		t.Fatal("missing oauth state cookie")
	}
}

func TestOAuthCallbackStateMismatch(t *testing.T) {
	h := testMux(t, http.NotFoundHandler())
	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/github/callback?code=x&state=nope", nil)
	req.AddCookie(&http.Cookie{Name: cookieOAuthState, Value: "other"})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status %d", rec.Code)
	}
}

func TestOAuthCallbackSuccessSetsSessionAndDropsGitHubToken(t *testing.T) {
	var sawToken bool
	gh := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "access_token"):
			w.Header().Set("Content-Type", "application/json")
			io.WriteString(w, `{"access_token":"gho_SHOULD_NOT_PERSIST","token_type":"bearer"}`)
		case strings.HasSuffix(r.URL.Path, "/user"):
			if a := r.Header.Get("Authorization"); strings.Contains(a, "gho_SHOULD_NOT_PERSIST") {
				sawToken = true
			}
			w.Header().Set("Content-Type", "application/json")
			io.WriteString(w, `{"id":4242,"login":"rainhuang0220","avatar_url":"https://avatars.githubusercontent.com/u/4242"}`)
		default:
			http.NotFound(w, r)
		}
	})
	h := testMux(t, gh)
	start := httptest.NewRequest(http.MethodGet, "/api/v1/auth/github", nil)
	startRec := httptest.NewRecorder()
	h.ServeHTTP(startRec, start)
	loc, _ := url.Parse(startRec.Header().Get("Location"))
	state := loc.Query().Get("state")
	var stateCookie *http.Cookie
	for _, c := range startRec.Result().Cookies() {
		if c.Name == cookieOAuthState {
			stateCookie = c
		}
	}
	if stateCookie == nil {
		t.Fatal("no state cookie")
	}
	cb := httptest.NewRequest(http.MethodGet, "/api/v1/auth/github/callback?code=ok&state="+url.QueryEscape(state), nil)
	cb.AddCookie(stateCookie)
	cbRec := httptest.NewRecorder()
	h.ServeHTTP(cbRec, cb)
	if cbRec.Code != http.StatusFound {
		t.Fatalf("callback %d %s", cbRec.Code, cbRec.Body.String())
	}
	if !strings.HasSuffix(cbRec.Header().Get("Location"), "/app") {
		t.Fatalf("redirect %s", cbRec.Header().Get("Location"))
	}
	var sessCookie *http.Cookie
	for _, c := range cbRec.Result().Cookies() {
		if c.Name == cookieSession {
			sessCookie = c
		}
	}
	if sessCookie == nil || !sessCookie.HttpOnly {
		t.Fatalf("session cookie %+v", sessCookie)
	}
	if !sawToken {
		t.Fatal("github user fetch must use the access token once")
	}
	// replay
	cb2 := httptest.NewRequest(http.MethodGet, "/api/v1/auth/github/callback?code=ok&state="+url.QueryEscape(state), nil)
	cb2.AddCookie(stateCookie)
	cb2Rec := httptest.NewRecorder()
	h.ServeHTTP(cb2Rec, cb2)
	if cb2Rec.Code != http.StatusBadRequest {
		t.Fatalf("replay status %d", cb2Rec.Code)
	}
	me := httptest.NewRequest(http.MethodGet, "/api/v1/session", nil)
	me.AddCookie(sessCookie)
	meRec := httptest.NewRecorder()
	h.ServeHTTP(meRec, me)
	if meRec.Code != http.StatusOK {
		t.Fatalf("session %d %s", meRec.Code, meRec.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(meRec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["login"] != "rainhuang0220" {
		t.Fatalf("%s", meRec.Body.Bytes())
	}
	raw, _ := json.Marshal(body)
	if strings.Contains(strings.ToLower(string(raw)), "gho_") {
		t.Fatalf("github token leaked: %s", raw)
	}
}

func TestLogoutRevokesSession(t *testing.T) {
	gh := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "access_token") {
			w.Header().Set("Content-Type", "application/json")
			io.WriteString(w, `{"access_token":"gho_x","token_type":"bearer"}`)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"id":7,"login":"out","avatar_url":""}`)
	})
	h := testMux(t, gh)
	start := httptest.NewRequest(http.MethodGet, "/api/v1/auth/github", nil)
	startRec := httptest.NewRecorder()
	h.ServeHTTP(startRec, start)
	loc, _ := url.Parse(startRec.Header().Get("Location"))
	var sc *http.Cookie
	for _, c := range startRec.Result().Cookies() {
		if c.Name == cookieOAuthState {
			sc = c
		}
	}
	cb := httptest.NewRequest(http.MethodGet, "/api/v1/auth/github/callback?code=ok&state="+url.QueryEscape(loc.Query().Get("state")), nil)
	cb.AddCookie(sc)
	cbRec := httptest.NewRecorder()
	h.ServeHTTP(cbRec, cb)
	var sess *http.Cookie
	for _, c := range cbRec.Result().Cookies() {
		if c.Name == cookieSession {
			sess = c
		}
	}
	lo := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	lo.AddCookie(sess)
	loRec := httptest.NewRecorder()
	h.ServeHTTP(loRec, lo)
	if loRec.Code != http.StatusNoContent && loRec.Code != http.StatusOK {
		t.Fatalf("logout %d", loRec.Code)
	}
	me := httptest.NewRequest(http.MethodGet, "/api/v1/session", nil)
	me.AddCookie(sess)
	meRec := httptest.NewRecorder()
	h.ServeHTTP(meRec, me)
	if meRec.Code != http.StatusUnauthorized {
		t.Fatalf("revoked session still live %d", meRec.Code)
	}
}
