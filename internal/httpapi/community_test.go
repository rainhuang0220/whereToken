package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/rainhuang0220/whereToken/internal/adapter/testhome"
	"github.com/rainhuang0220/whereToken/internal/community"
	"github.com/rainhuang0220/whereToken/internal/scan"
)

func TestCommunityAPILocalOnlyAndNoEnumerate(t *testing.T) {
	srv := httptest.NewServer(NewMux(testhome.New(t.TempDir())))
	t.Cleanup(srv.Close)

	res, err := http.Get(srv.URL + "/api/community")
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
	if _, ok := body["participant_id"]; ok {
		t.Fatal("local API must not put participant_id on the dashboard payload")
	}
	if _, ok := body["users"]; ok {
		t.Fatal("must not enumerate users")
	}
	if body["enabled"] != false {
		t.Fatalf("unconfigured rank must not look opted-in: %v", body)
	}

	req, err := http.NewRequest(http.MethodPost, srv.URL+"/api/community", strings.NewReader(`{"enabled":false}`))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "https://evil.example")
	res, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusForbidden {
		t.Fatalf("foreign origin %d", res.StatusCode)
	}

	for _, path := range []string{
		"/v1/community/users",
		"/v1/community/participants",
		"/v1/community/rank",
		"/v1/community/rank?period=2026-08-19",
	} {
		t.Run(path, func(t *testing.T) {
			res, err := http.Get(srv.URL + path)
			if err != nil {
				t.Fatal(err)
			}
			res.Body.Close()
			if res.StatusCode != http.StatusNotFound {
				t.Fatalf("%s → %d", path, res.StatusCode)
			}
		})
	}
}

func TestCommunityRejectsForeignReferer(t *testing.T) {
	srv := httptest.NewServer(NewMux(testhome.New(t.TempDir())))
	t.Cleanup(srv.Close)
	req, err := http.NewRequest(http.MethodPost, srv.URL+"/api/community", strings.NewReader(`{"enabled":false}`))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Referer", "https://evil.example/page")
	res, err := srv.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusForbidden {
		t.Fatalf("foreign referer %d", res.StatusCode)
	}
}

func TestCommunityRejectsNonLocalHost(t *testing.T) {
	srv := httptest.NewServer(NewMux(testhome.New(t.TempDir())))
	t.Cleanup(srv.Close)
	req, err := http.NewRequest(http.MethodGet, srv.URL+"/api/community", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Host = "evil.example"
	res, err := srv.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusForbidden {
		t.Fatalf("status=%d want 403 for foreign Host", res.StatusCode)
	}
}

func TestCommunityAPISanitizesZeroRank(t *testing.T) {
	s := &server{home: testhome.New(t.TempDir())}
	view := community.EmptyView(community.StatusOK, community.DisclaimerEN)
	view.Today.Rank = 0
	view.Today.Display = "#0 / 20"
	view.All = view.Today
	view.All.Period = community.PeriodAll
	s.last = &scan.Result{Community: &view, Errors: []string{}}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/community", s.getCommunity)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	res, err := http.Get(srv.URL + "/api/community")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var body map[string]any
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	today, ok := body["today"].(map[string]any)
	if !ok {
		t.Fatalf("today: %v", body)
	}
	if today["status"] == "ok" {
		t.Fatalf("zero podium must not stay ok: %v", body)
	}
	if _, has := today["rank"]; has {
		t.Fatalf("rank must be omitted: %v", body)
	}
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "#0") {
		t.Fatalf("#0 leaked: %s", raw)
	}
}

func TestCommunityDisableKeepsOnWhenLeaveFails(t *testing.T) {
	t.Setenv("WHERETOKEN_COMMUNITY_URL", "http://127.0.0.1:1")
	dir := t.TempDir()
	home := testhome.New(dir)
	path := community.ConfigPath(home)
	f := &community.File{ParticipantID: "aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee", Enabled: true, JoinedAt: "2026-08-19"}
	if err := f.Save(path); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(NewMux(home))
	t.Cleanup(srv.Close)
	origin := "http://" + strings.TrimPrefix(srv.URL, "http://")
	req, err := http.NewRequest(http.MethodPost, srv.URL+"/api/community", strings.NewReader(`{"enabled":false}`))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", origin)
	res, err := srv.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode == http.StatusOK {
		t.Fatal("dashboard must not report opted-out when remote leave fails")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"enabled": true`) {
		t.Fatalf("must keep participation on after failed leave: %s", raw)
	}
}

func TestNoCommunityPOSTDoesNotMint(t *testing.T) {
	t.Setenv("WHERETOKEN_COMMUNITY_URL", "http://127.0.0.1:1")
	dir := t.TempDir()
	home := testhome.New(dir)
	srv := httptest.NewServer(NewMuxOpts(home, scan.AllAdapters(), true))
	t.Cleanup(srv.Close)
	origin := "http://" + strings.TrimPrefix(srv.URL, "http://")
	for _, enabled := range []string{"true", "false"} {
		req, err := http.NewRequest(http.MethodPost, srv.URL+"/api/community", strings.NewReader(`{"enabled":`+enabled+`}`))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Origin", origin)
		res, err := srv.Client().Do(req)
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		if res.StatusCode != http.StatusForbidden {
			t.Fatalf("enabled=%s status=%d want 403", enabled, res.StatusCode)
		}
	}
	if _, err := os.Stat(community.ConfigPath(home)); !os.IsNotExist(err) {
		t.Fatal("serve --no-community must not mint community.json")
	}
}

func TestGetSummaryDoesNotUpload(t *testing.T) {
	var mu sync.Mutex
	posts := 0
	rank := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && strings.Contains(r.URL.Path, "/usage"):
			mu.Lock()
			posts++
			mu.Unlock()
			w.WriteHeader(http.StatusNoContent)
		case strings.Contains(r.URL.Path, "/rank"):
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(community.Standing{
				Status: community.StatusInsufficientParticipants, Period: "today",
				Metric: community.MetricTokens, SelfReported: true, Note: community.DisclaimerEN,
			})
		case strings.Contains(r.URL.Path, "/leave"):
			w.WriteHeader(http.StatusNoContent)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(rank.Close)
	t.Setenv("WHERETOKEN_COMMUNITY_URL", rank.URL)
	t.Setenv("WHERETOKEN_EXTRA_ROOTS", "")
	dir := writeKimiHome(t)
	srv := httptest.NewServer(NewMux(testhome.New(dir)))
	t.Cleanup(srv.Close)
	postScanJSON(t, srv)
	mu.Lock()
	afterScan := posts
	mu.Unlock()
	for i := 0; i < 5; i++ {
		res, err := http.Get(srv.URL + "/api/summary")
		if err != nil {
			t.Fatal(err)
		}
		if res.StatusCode != http.StatusOK {
			res.Body.Close()
			t.Fatalf("summary %d", res.StatusCode)
		}
		res.Body.Close()
	}
	mu.Lock()
	afterGet := posts
	mu.Unlock()
	if afterGet != afterScan {
		t.Fatalf("GET /api/summary uploaded %d extra times (scan=%d get=%d)", afterGet-afterScan, afterScan, afterGet)
	}
}

func TestMuxOptsNoCommunityDisablesParticipation(t *testing.T) {
	t.Setenv("WHERETOKEN_COMMUNITY_URL", "http://127.0.0.1:1")
	srv := httptest.NewServer(NewMuxOpts(testhome.New(t.TempDir()), scan.AllAdapters(), true))
	t.Cleanup(srv.Close)
	res, err := http.Get(srv.URL + "/api/community")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var body map[string]any
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body["enabled"] != false {
		t.Fatalf("serve --no-community must disable rank: %v", body)
	}
}

func TestConcurrentSummaryAndCommunityToggle(t *testing.T) {
	t.Setenv("WHERETOKEN_EXTRA_ROOTS", "")
	dir := t.TempDir()
	srv := httptest.NewServer(NewMuxOpts(testhome.New(dir), scan.Adapters(true), true))
	t.Cleanup(srv.Close)
	origin := "http://" + strings.TrimPrefix(srv.URL, "http://")
	scanReq, err := http.NewRequest(http.MethodPost, srv.URL+"/api/scan", nil)
	if err != nil {
		t.Fatal(err)
	}
	scanReq.Header.Set("Origin", origin)
	res, err := srv.Client().Do(scanReq)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()

	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < 20; i++ {
			req, _ := http.NewRequest(http.MethodPost, srv.URL+"/api/community", strings.NewReader(`{"enabled":false}`))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Origin", origin)
			r, err := srv.Client().Do(req)
			if err == nil {
				r.Body.Close()
			}
			r2, err := http.Get(srv.URL + "/api/summary")
			if err == nil {
				r2.Body.Close()
			}
		}
	}()
	for i := 0; i < 20; i++ {
		r, err := http.Get(srv.URL + "/api/summary")
		if err == nil {
			r.Body.Close()
		}
	}
	<-done
}

func TestNoCommunityScanDoesNotMintOrUpload(t *testing.T) {
	t.Setenv("WHERETOKEN_COMMUNITY_URL", "http://127.0.0.1:1")
	t.Setenv("WHERETOKEN_EXTRA_ROOTS", "")
	dir := t.TempDir()
	home := testhome.New(dir)
	srv := httptest.NewServer(NewMuxOpts(home, scan.Adapters(true), true))
	t.Cleanup(srv.Close)
	req, err := http.NewRequest(http.MethodPost, srv.URL+"/api/scan", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Origin", "http://"+req.URL.Host)
	res, err := srv.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("scan %d", res.StatusCode)
	}
	var payload map[string]any
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	if comm, ok := payload["community"].(map[string]any); ok {
		if comm["enabled"] != false {
			t.Fatalf("scan with --no-community must not opt in: %v", comm)
		}
	}
	if _, err := os.Stat(community.ConfigPath(home)); !os.IsNotExist(err) {
		t.Fatalf("must not mint community.json: %v", err)
	}
}
