package hosted

import (
	"encoding/json"
	"io"
	"log"
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

func TestPKCEChallengeMatchesRFC7636(t *testing.T) {
	got := pkceChallenge("dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk")
	if got != "E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM" {
		t.Fatalf("challenge %s", got)
	}
	if len(got) != 43 {
		t.Fatalf("challenge len %d", len(got))
	}
	v, ch, err := newPKCE()
	if err != nil {
		t.Fatal(err)
	}
	if len(v) < 43 {
		t.Fatalf("verifier len %d", len(v))
	}
	if pkceChallenge(v) != ch {
		t.Fatal("challenge must be S256 of the raw verifier")
	}
}

func TestOAuthStartSendsS256AndLongVerifier(t *testing.T) {
	h := testMux(t, http.NotFoundHandler())
	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/github", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	u, err := url.Parse(rec.Header().Get("Location"))
	if err != nil {
		t.Fatal(err)
	}
	if u.Query().Get("code_challenge_method") != "S256" {
		t.Fatalf("%s", rec.Header().Get("Location"))
	}
	ch := u.Query().Get("code_challenge")
	if len(ch) != 43 {
		t.Fatalf("challenge len %d", len(ch))
	}
	var oc *http.Cookie
	for _, c := range rec.Result().Cookies() {
		if c.Name == cookieOAuthState {
			oc = c
		}
	}
	if oc == nil {
		t.Fatal("missing cookie")
	}
	parts := strings.SplitN(oc.Value, ":", 2)
	if len(parts) != 2 || len(parts[1]) < 43 {
		t.Fatalf("verifier cookie %q", oc.Value)
	}
	if pkceChallenge(parts[1]) != ch {
		t.Fatal("authorize challenge must match cookie verifier")
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

func TestOAuthExchangeSendsRawVerifierNotChallenge(t *testing.T) {
	var form url.Values
	gh := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "access_token") {
			_ = r.ParseForm()
			form = r.PostForm
			w.Header().Set("Content-Type", "application/json")
			io.WriteString(w, `{"access_token":"gho_ok","token_type":"bearer"}`)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"id":1,"login":"pkce","avatar_url":""}`)
	})
	h := testMux(t, gh)
	start := httptest.NewRequest(http.MethodGet, "/api/v1/auth/github", nil)
	startRec := httptest.NewRecorder()
	h.ServeHTTP(startRec, start)
	loc, _ := url.Parse(startRec.Header().Get("Location"))
	var oc *http.Cookie
	for _, c := range startRec.Result().Cookies() {
		if c.Name == cookieOAuthState {
			oc = c
		}
	}
	verifier := strings.SplitN(oc.Value, ":", 2)[1]
	challenge := loc.Query().Get("code_challenge")
	cb := httptest.NewRequest(http.MethodGet, "/api/v1/auth/github/callback?code=ok&state="+url.QueryEscape(loc.Query().Get("state")), nil)
	cb.AddCookie(oc)
	cbRec := httptest.NewRecorder()
	h.ServeHTTP(cbRec, cb)
	if cbRec.Code != http.StatusFound {
		t.Fatalf("callback %d %s", cbRec.Code, cbRec.Body.String())
	}
	if form.Get("code_verifier") != verifier {
		t.Fatalf("verifier %q vs cookie", form.Get("code_verifier"))
	}
	if form.Get("code_verifier") == challenge {
		t.Fatal("must not send the challenge as the verifier")
	}
	if form.Get("redirect_uri") != "https://wheretoken.plainlist.space/api/v1/auth/github/callback" {
		t.Fatalf("redirect_uri %s", form.Get("redirect_uri"))
	}
}

func TestOAuthMissingVerifierFails(t *testing.T) {
	h := testMux(t, http.NotFoundHandler())
	start := httptest.NewRequest(http.MethodGet, "/api/v1/auth/github", nil)
	startRec := httptest.NewRecorder()
	h.ServeHTTP(startRec, start)
	loc, _ := url.Parse(startRec.Header().Get("Location"))
	state := loc.Query().Get("state")
	cb := httptest.NewRequest(http.MethodGet, "/api/v1/auth/github/callback?code=ok&state="+url.QueryEscape(state), nil)
	cb.AddCookie(&http.Cookie{Name: cookieOAuthState, Value: state})
	cbRec := httptest.NewRecorder()
	h.ServeHTTP(cbRec, cb)
	if cbRec.Code != http.StatusBadRequest {
		t.Fatalf("status %d", cbRec.Code)
	}
}

func TestOAuthWrongVerifierFails(t *testing.T) {
	h := testMux(t, http.NotFoundHandler())
	start := httptest.NewRequest(http.MethodGet, "/api/v1/auth/github", nil)
	startRec := httptest.NewRecorder()
	h.ServeHTTP(startRec, start)
	loc, _ := url.Parse(startRec.Header().Get("Location"))
	state := loc.Query().Get("state")
	cb := httptest.NewRequest(http.MethodGet, "/api/v1/auth/github/callback?code=ok&state="+url.QueryEscape(state), nil)
	cb.AddCookie(&http.Cookie{Name: cookieOAuthState, Value: state + ":not-the-verifier-aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"})
	cbRec := httptest.NewRecorder()
	h.ServeHTTP(cbRec, cb)
	if cbRec.Code != http.StatusBadRequest {
		t.Fatalf("status %d", cbRec.Code)
	}
}

func TestOAuthGitHubErrorRedirectsToLoginWithoutSecrets(t *testing.T) {
	var logs strings.Builder
	st := readyStore(t)
	ghSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"error":"incorrect_client_credentials","error_description":"The client_id and/or client_secret passed are incorrect."}`)
	}))
	t.Cleanup(ghSrv.Close)
	h := NewMux(MuxOptions{
		Version: "0.7.0",
		Store:   st,
		Config: Config{
			Listen:             "127.0.0.1:0",
			GitHubClientID:     "cid",
			GitHubClientSecret: "csecret",
			PublicURL:          "https://wheretoken.plainlist.space",
			CookieSecure:       false,
		},
		GitHub: GitHubEndpoints{
			AuthorizeURL: ghSrv.URL + "/login/oauth/authorize",
			TokenURL:     ghSrv.URL + "/login/oauth/access_token",
			UserURL:      ghSrv.URL + "/user",
			HTTPClient:   ghSrv.Client(),
		},
		Log: log.New(&logs, "", 0),
	})
	start := httptest.NewRequest(http.MethodGet, "/api/v1/auth/github", nil)
	startRec := httptest.NewRecorder()
	h.ServeHTTP(startRec, start)
	loc, _ := url.Parse(startRec.Header().Get("Location"))
	var oc *http.Cookie
	for _, c := range startRec.Result().Cookies() {
		if c.Name == cookieOAuthState {
			oc = c
		}
	}
	cb := httptest.NewRequest(http.MethodGet, "/api/v1/auth/github/callback?code=ok&state="+url.QueryEscape(loc.Query().Get("state")), nil)
	cb.AddCookie(oc)
	cbRec := httptest.NewRecorder()
	h.ServeHTTP(cbRec, cb)
	if cbRec.Code != http.StatusFound {
		t.Fatalf("status %d %s", cbRec.Code, cbRec.Body.String())
	}
	redir := cbRec.Header().Get("Location")
	if !strings.Contains(redir, "/login?err=oauth") {
		t.Fatalf("redirect %s", redir)
	}
	if !strings.Contains(redir, "rid=") {
		t.Fatalf("missing rid %s", redir)
	}
	body := cbRec.Body.String() + redir + logs.String()
	if strings.Contains(body, "csecret") || strings.Contains(strings.ToLower(body), "gho_") {
		t.Fatalf("secret leaked: %s", body)
	}
	if !strings.Contains(logs.String(), "stage=github_token_exchange") {
		t.Fatalf("log %s", logs.String())
	}
	if !strings.Contains(logs.String(), "error=incorrect_client_credentials") {
		t.Fatalf("log %s", logs.String())
	}
	if strings.Contains(cbRec.Body.String(), "oauth exchange failed") {
		t.Fatal("must not dump raw exchange text")
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
	var sess, csrf *http.Cookie
	for _, c := range cbRec.Result().Cookies() {
		switch c.Name {
		case cookieSession:
			sess = c
		case cookieCSRF:
			csrf = c
		}
	}
	lo := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	lo.AddCookie(sess)
	if csrf != nil {
		lo.AddCookie(csrf)
		lo.Header.Set("X-CSRF-Token", csrf.Value)
	}
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
