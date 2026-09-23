package httpapi

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rainhuang0220/whereToken/internal/adapter/testhome"
	"github.com/rainhuang0220/whereToken/internal/publicprofile"
)

type fakePublisher struct {
	mu        sync.Mutex
	preflight int
	approve   int
	retry     int
	palette   string
	release   chan struct{}
}

func (f *fakePublisher) Preflight(_ context.Context, palette string) (publicprofile.Preflight, error) {
	f.mu.Lock()
	f.preflight++
	f.palette = palette
	f.mu.Unlock()
	return publicprofile.Preflight{
		ID: "pf-1", Phase: publicprofile.PhaseAwaitingApproval, PhaseLabel: publicprofile.PhaseLabel(publicprofile.PhaseAwaitingApproval),
		Ready: true, Palette: palette, CacheKey: "15c96d356fc90d8073524036f29bef83ef236f76f7b02a11e862b9cbb4c52e62-3e30d4babdb260f4570aa1fea89a21ff9f00376c190a0615cdf6cdbfa0d76fe5",
		ProductRepo: "rainhuang0220/whereToken", ProfileRepo: "rainhuang0220/rainhuang0220",
	}, nil
}

func (f *fakePublisher) Approve(_ context.Context, palette string, report func(publicprofile.Job)) (publicprofile.Job, error) {
	f.mu.Lock()
	f.approve++
	f.mu.Unlock()
	if f.release != nil {
		<-f.release
	}
	job := publicprofile.Job{Phase: publicprofile.PhaseVerified, PhaseLabel: publicprofile.PhaseLabel(publicprofile.PhaseVerified), Palette: palette}
	if report != nil {
		report(job)
	}
	return job, nil
}

func (f *fakePublisher) RetryReadme(_ context.Context, palette string, report func(publicprofile.Job)) (publicprofile.Job, error) {
	f.mu.Lock()
	f.retry++
	f.mu.Unlock()
	job := publicprofile.Job{Phase: publicprofile.PhaseVerified, Palette: palette}
	if report != nil {
		report(job)
	}
	return job, nil
}

func publishServer(t *testing.T, pub profilePublisher) *httptest.Server {
	t.Helper()
	s := &server{home: testhome.New(t.TempDir()), publisher: pub, version: "test"}
	srv := httptest.NewServer(withSafeHeaders(s.routes()))
	t.Cleanup(srv.Close)
	return srv
}

func TestPublishGETDoesNotApprove(t *testing.T) {
	pub := &fakePublisher{}
	srv := publishServer(t, pub)
	res, err := http.Get(srv.URL + "/api/public-profile/publish?palette=magenta")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status %d", res.StatusCode)
	}
	var body map[string]any
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body["palette"] != "magenta" || body["csrf"] == "" || body["phase"] != publicprofile.PhaseAwaitingApproval {
		t.Fatalf("%v", body)
	}
	if pub.approve != 0 || pub.retry != 0 {
		t.Fatalf("GET mutated approve=%d retry=%d", pub.approve, pub.retry)
	}
	raw, _ := json.Marshal(body)
	if strings.Contains(string(raw), "ghp_") || strings.Contains(string(raw), "/Users/") {
		t.Fatal("preflight leaked a secret or path")
	}
}

func TestPublishPOSTRequiresSameOriginCSRF(t *testing.T) {
	pub := &fakePublisher{}
	srv := publishServer(t, pub)
	token := publishToken(t, srv)
	cases := []struct {
		name   string
		header map[string]string
		body   string
		host   string
	}{
		{"foreign origin", map[string]string{"Origin": "https://evil.example", "X-WhereToken-CSRF": token.csrf, "Sec-Fetch-Site": "cross-site"}, token.body, ""},
		{"same site other port", map[string]string{"Origin": "http://127.0.0.1:9", "X-WhereToken-CSRF": token.csrf, "Sec-Fetch-Site": "same-site"}, token.body, ""},
		{"missing header", map[string]string{"Origin": token.origin}, token.body, ""},
		{"wrong token", map[string]string{"Origin": token.origin, "X-WhereToken-CSRF": "nope", "Sec-Fetch-Site": "same-origin"}, token.body, ""},
		{"foreign host", map[string]string{"Origin": "http://evil.example", "X-WhereToken-CSRF": token.csrf, "Sec-Fetch-Site": "same-origin"}, token.body, "evil.example"},
		{"bad palette", map[string]string{"Origin": token.origin, "X-WhereToken-CSRF": token.csrf, "Sec-Fetch-Site": "same-origin"}, `{"action":"approve","preflight_id":"pf-1","csrf":"` + token.csrf + `","palette":"kiln"}`, ""},
	}
	for _, tc := range cases {
		req, err := http.NewRequest(http.MethodPost, srv.URL+"/api/public-profile/publish", strings.NewReader(tc.body))
		if err != nil {
			t.Fatal(err)
		}
		for k, v := range tc.header {
			req.Header.Set(k, v)
		}
		if tc.host != "" {
			req.Host = tc.host
		}
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		if res.StatusCode < 400 {
			t.Fatalf("%s status %d", tc.name, res.StatusCode)
		}
	}
	if pub.approve != 0 {
		t.Fatalf("blocked posts approved %d times", pub.approve)
	}
}

func TestPublishPOSTApprovesOnce(t *testing.T) {
	pub := &fakePublisher{release: make(chan struct{})}
	srv := publishServer(t, pub)
	token := publishToken(t, srv)
	post := func() *http.Response {
		t.Helper()
		req, err := http.NewRequest(http.MethodPost, srv.URL+"/api/public-profile/publish", strings.NewReader(token.body))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Origin", token.origin)
		req.Header.Set("X-WhereToken-CSRF", token.csrf)
		req.Header.Set("Sec-Fetch-Site", "same-origin")
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		io.Copy(io.Discard, res.Body)
		return res
	}
	first := post()
	if first.StatusCode != http.StatusOK {
		t.Fatalf("first %d", first.StatusCode)
	}
	second := post()
	if second.StatusCode != http.StatusOK {
		t.Fatalf("second %d", second.StatusCode)
	}
	close(pub.release)
	waitApprove(t, pub, 1)
}

func TestPublishRejectsMalformedPaletteOnGET(t *testing.T) {
	srv := publishServer(t, &fakePublisher{})
	for _, palette := range []string{"kiln", "../whereToken", "magenta;rm", "magenta\nrm"} {
		res, err := http.Get(srv.URL + "/api/public-profile/publish?palette=" + url.QueryEscape(palette))
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		if res.StatusCode != http.StatusBadRequest {
			t.Fatalf("%s status %d", palette, res.StatusCode)
		}
	}
}

type issuedToken struct {
	csrf   string
	origin string
	body   string
}

func publishToken(t *testing.T, srv *httptest.Server) issuedToken {
	t.Helper()
	res, err := http.Get(srv.URL + "/api/public-profile/publish?palette=newsprint")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var body struct {
		ID   string `json:"preflight_id"`
		CSRF string `json:"csrf"`
	}
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	origin := srv.URL
	return issuedToken{
		csrf:   body.CSRF,
		origin: origin,
		body:   `{"action":"approve","preflight_id":"` + body.ID + `","csrf":"` + body.CSRF + `","palette":"newsprint"}`,
	}
}

func waitApprove(t *testing.T, pub *fakePublisher, want int) {
	t.Helper()
	for i := 0; i < 50; i++ {
		pub.mu.Lock()
		got := pub.approve
		pub.mu.Unlock()
		if got == want {
			return
		}
		timeSleep()
	}
	t.Fatalf("approve=%d want %d", pub.approve, want)
}

func timeSleep() { time.Sleep(20 * time.Millisecond) }
