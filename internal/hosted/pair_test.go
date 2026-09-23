package hosted

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
)

func TestPairStartStatusConfirmAndSingleToken(t *testing.T) {
	h := testMux(t, githubStub())
	auth := loginAuth(t, h)

	startReq := httptest.NewRequest(http.MethodPost, "/api/v1/pair/start", strings.NewReader(`{"os":"darwin","arch":"arm64","client_version":"0.7.0","label":"MacBook Pro"}`))
	startReq.Header.Set("Content-Type", "application/json")
	startRec := httptest.NewRecorder()
	h.ServeHTTP(startRec, startReq)
	if startRec.Code != http.StatusOK {
		t.Fatalf("start %d %s", startRec.Code, startRec.Body.String())
	}
	var started map[string]any
	if err := json.Unmarshal(startRec.Body.Bytes(), &started); err != nil {
		t.Fatal(err)
	}
	code, _ := started["display_code"].(string)
	secret, _ := started["device_secret"].(string)
	if len(code) < 8 || secret == "" {
		t.Fatalf("%s", startRec.Body.Bytes())
	}
	// The CLI-generated device_secret is the eventual bearer credential
	// itself, held only by the process that started pairing; the server
	// never has to hand a second, separately-generated secret back later.
	if !strings.HasPrefix(secret, "wtd_1.") {
		t.Fatalf("device_secret must already carry the bearer prefix: %q", secret)
	}

	peek := httptest.NewRequest(http.MethodGet, "/api/v1/pair/challenge?code="+url.QueryEscape(code), nil)
	auth.apply(peek)
	peekRec := httptest.NewRecorder()
	h.ServeHTTP(peekRec, peek)
	if peekRec.Code != http.StatusOK {
		t.Fatalf("peek %d %s", peekRec.Code, peekRec.Body.String())
	}
	if !strings.Contains(peekRec.Body.String(), "MacBook Pro") || strings.Contains(peekRec.Body.String(), secret) {
		t.Fatalf("peek body %s", peekRec.Body.String())
	}

	status := func() (int, map[string]any) {
		body, _ := json.Marshal(map[string]string{"display_code": code, "device_secret": secret})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/pair/status", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		var m map[string]any
		_ = json.Unmarshal(rec.Body.Bytes(), &m)
		return rec.Code, m
	}
	if _, m := status(); m["status"] != "pending" {
		t.Fatalf("pending %+v", m)
	}

	wrong, _ := json.Marshal(map[string]string{"display_code": code, "device_secret": "nope"})
	bad := httptest.NewRequest(http.MethodPost, "/api/v1/pair/status", bytes.NewReader(wrong))
	bad.Header.Set("Content-Type", "application/json")
	badRec := httptest.NewRecorder()
	h.ServeHTTP(badRec, bad)
	if badRec.Code != http.StatusUnauthorized && badRec.Code != http.StatusForbidden {
		t.Fatalf("wrong secret %d", badRec.Code)
	}

	confirm, _ := json.Marshal(map[string]any{"display_code": code, "accept": true})
	cr := httptest.NewRequest(http.MethodPost, "/api/v1/pair/confirm", bytes.NewReader(confirm))
	cr.Header.Set("Content-Type", "application/json")
	auth.apply(cr)
	crRec := httptest.NewRecorder()
	h.ServeHTTP(crRec, cr)
	if crRec.Code != http.StatusOK {
		t.Fatalf("confirm %d %s", crRec.Code, crRec.Body.String())
	}

	st, m := status()
	if st != http.StatusOK || m["status"] != "approved" {
		t.Fatalf("approved %d %+v", st, m)
	}
	tok, _ := m["device_token"].(string)
	if tok != secret {
		t.Fatalf("device_token must equal the original device_secret, got %q want %q", tok, secret)
	}
	if m["source_hmac_key"] == nil || m["source_hmac_key"] == "" {
		t.Fatal("approved poll must return the user HMAC key")
	}

	// A dropped response, a slow client, or a server restart must not turn a
	// successful approval into an unrecoverable "consumed" dead end: polling
	// again with the same (code, secret) must keep returning the same
	// approval rather than requiring the user to re-pair.
	_, m2 := status()
	if m2["status"] != "approved" {
		t.Fatalf("repeated poll after approval: %+v", m2)
	}
	if m2["device_token"] != secret {
		t.Fatalf("repeated poll returned a different token: %+v", m2)
	}
}

// TestPairConcurrentConfirmCreatesOneDevice proves that two overlapping
// approvals of the same pairing challenge by the same signed-in user cannot
// race into two devices sharing one challenge, and that both requests
// observe the same terminal device identity.
func TestPairConcurrentConfirmCreatesOneDevice(t *testing.T) {
	h := testMux(t, githubStub())
	auth := loginAuth(t, h)

	startReq := httptest.NewRequest(http.MethodPost, "/api/v1/pair/start", strings.NewReader(`{"os":"linux","arch":"amd64","client_version":"0.7.0"}`))
	startReq.Header.Set("Content-Type", "application/json")
	startRec := httptest.NewRecorder()
	h.ServeHTTP(startRec, startReq)
	var started map[string]any
	if err := json.Unmarshal(startRec.Body.Bytes(), &started); err != nil {
		t.Fatal(err)
	}
	code := started["display_code"].(string)

	confirmOnce := func() (int, map[string]any) {
		body, _ := json.Marshal(map[string]any{"display_code": code, "accept": true})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/pair/confirm", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		auth.apply(req)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		var m map[string]any
		_ = json.Unmarshal(rec.Body.Bytes(), &m)
		return rec.Code, m
	}

	const n = 8
	results := make([]struct {
		code int
		body map[string]any
	}, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			results[i].code, results[i].body = confirmOnce()
		}(i)
	}
	wg.Wait()

	deviceIDs := map[string]struct{}{}
	for _, r := range results {
		if r.code != http.StatusOK {
			t.Fatalf("confirm %d: %+v", r.code, r.body)
		}
		if id, _ := r.body["device_id"].(string); id != "" {
			deviceIDs[id] = struct{}{}
		}
	}
	if len(deviceIDs) != 1 {
		t.Fatalf("concurrent confirms produced %d distinct devices: %v", len(deviceIDs), deviceIDs)
	}
}

func TestPairReject(t *testing.T) {
	h := testMux(t, githubStub())
	auth := loginAuth(t, h)
	startReq := httptest.NewRequest(http.MethodPost, "/api/v1/pair/start", strings.NewReader(`{"os":"linux","arch":"amd64","client_version":"0.7.0"}`))
	startReq.Header.Set("Content-Type", "application/json")
	startRec := httptest.NewRecorder()
	h.ServeHTTP(startRec, startReq)
	var started map[string]any
	_ = json.Unmarshal(startRec.Body.Bytes(), &started)
	code := started["display_code"].(string)
	secret := started["device_secret"].(string)
	confirm, _ := json.Marshal(map[string]any{"display_code": code, "accept": false})
	cr := httptest.NewRequest(http.MethodPost, "/api/v1/pair/confirm", bytes.NewReader(confirm))
	cr.Header.Set("Content-Type", "application/json")
	auth.apply(cr)
	crRec := httptest.NewRecorder()
	h.ServeHTTP(crRec, cr)
	if crRec.Code != http.StatusOK {
		t.Fatalf("reject %d %s", crRec.Code, crRec.Body.String())
	}
	body, _ := json.Marshal(map[string]string{"display_code": code, "device_secret": secret})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/pair/status", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	var m map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &m)
	if m["status"] != "denied" {
		t.Fatalf("%+v", m)
	}
}

func githubStub() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "access_token") {
			w.Header().Set("Content-Type", "application/json")
			ioWrite(w, `{"access_token":"gho_x","token_type":"bearer"}`)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		ioWrite(w, `{"id":99,"login":"pairuser","avatar_url":""}`)
	})
}

func ioWrite(w http.ResponseWriter, s string) {
	_, _ = w.Write([]byte(s))
}

type authCookies struct {
	Session, CSRF *http.Cookie
}

func (a authCookies) apply(req *http.Request) {
	if a.Session != nil {
		req.AddCookie(a.Session)
	}
	if a.CSRF != nil {
		req.AddCookie(a.CSRF)
		req.Header.Set("X-CSRF-Token", a.CSRF.Value)
	}
}

func loginSession(t *testing.T, h http.Handler) *http.Cookie {
	t.Helper()
	return loginAuth(t, h).Session
}

func loginAuth(t *testing.T, h http.Handler) authCookies {
	t.Helper()
	start := httptest.NewRequest(http.MethodGet, "/api/v1/auth/github", nil)
	startRec := httptest.NewRecorder()
	h.ServeHTTP(startRec, start)
	loc := startRec.Header().Get("Location")
	u, err := url.Parse(loc)
	if err != nil {
		t.Fatal(err)
	}
	var oc *http.Cookie
	for _, c := range startRec.Result().Cookies() {
		if c.Name == cookieOAuthState {
			oc = c
		}
	}
	cb := httptest.NewRequest(http.MethodGet, "/api/v1/auth/github/callback?code=ok&state="+u.Query().Get("state"), nil)
	cb.AddCookie(oc)
	cbRec := httptest.NewRecorder()
	h.ServeHTTP(cbRec, cb)
	out := authCookies{}
	for _, c := range cbRec.Result().Cookies() {
		switch c.Name {
		case cookieSession:
			out.Session = c
		case cookieCSRF:
			out.CSRF = c
		}
	}
	if out.Session == nil {
		t.Fatalf("login failed %d %s", cbRec.Code, cbRec.Body.String())
	}
	return out
}
