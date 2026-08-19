package httpapi

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

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

func TestGetSummaryNeverDialsRankService(t *testing.T) {
	var mu sync.Mutex
	hits := 0
	rank := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		hits++
		mu.Unlock()
		if r.Method == http.MethodPost {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(community.Standing{
			Status: community.StatusInsufficientParticipants, Period: "today",
			Metric: community.MetricTokens, SelfReported: true, Note: community.DisclaimerEN,
		})
	}))
	t.Cleanup(rank.Close)
	t.Setenv("WHERETOKEN_COMMUNITY_URL", rank.URL)
	t.Setenv("WHERETOKEN_EXTRA_ROOTS", "")
	dir := writeKimiHome(t)
	srv := httptest.NewServer(NewMux(testhome.New(dir)))
	t.Cleanup(srv.Close)

	res, err := http.Get(srv.URL + "/api/summary")
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("cold summary %d", res.StatusCode)
	}
	mu.Lock()
	if hits != 0 {
		mu.Unlock()
		t.Fatal("cold GET /api/summary must not dial the rank service")
	}
	mu.Unlock()

	postScanJSON(t, srv)
	mu.Lock()
	afterScan := hits
	mu.Unlock()
	for i := 0; i < 5; i++ {
		res, err := http.Get(srv.URL + "/api/summary?since=today")
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
	afterGet := hits
	mu.Unlock()
	if afterGet != afterScan {
		t.Fatalf("GET /api/summary dialed rank after scan: scan=%d get=%d", afterScan, afterGet)
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
	store := community.NewStore(community.DefaultMinParticipants)
	h := community.NewHandler(store)
	day := time.Now().In(time.Local).Format("2006-01-02")
	for i := 0; i < community.DefaultMinParticipants-1; i++ {
		id := fmt.Sprintf("aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeee%02d", i)
		if err := store.Put(community.Upload{
			ParticipantID: id, Period: day, Tokens: 10, ClientVersion: "dev",
		}); err != nil {
			t.Fatal(err)
		}
	}

	var mu sync.Mutex
	usagePosts := 0
	inner := h.Mux()
	rank := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && strings.Contains(r.URL.Path, "/usage") {
			mu.Lock()
			usagePosts++
			mu.Unlock()
		}
		inner.ServeHTTP(w, r)
	}))
	t.Cleanup(rank.Close)

	t.Setenv("WHERETOKEN_COMMUNITY_URL", rank.URL)
	t.Setenv("WHERETOKEN_COMMUNITY", "")
	t.Setenv("DO_NOT_TRACK", "")
	t.Setenv("WHERETOKEN_EXTRA_ROOTS", "")

	dir := writeKimiHome(t)
	writeTodayPricedClaude(t, dir)
	home := testhome.New(dir)
	srv := httptest.NewServer(NewMuxOpts(home, scan.AllAdapters(), false))
	t.Cleanup(srv.Close)
	client := srv.Client()
	origin := "http://" + strings.TrimPrefix(srv.URL, "http://")

	scanReq, err := http.NewRequest(http.MethodPost, srv.URL+"/api/scan", nil)
	if err != nil {
		t.Fatal(err)
	}
	scanReq.Header.Set("Origin", origin)
	code, raw, err := doHTTP(client, scanReq)
	if err != nil {
		t.Fatal(err)
	}
	if code != http.StatusOK {
		t.Fatalf("scan %d %s", code, raw)
	}
	if leak := zeroRankLeak(raw); leak != "" {
		t.Fatalf("scan leaked %s: %s", leak, raw)
	}
	mu.Lock()
	afterScan := usagePosts
	mu.Unlock()
	if afterScan < 1 {
		t.Fatal("scan with today events must upload Community Rank usage")
	}

	var (
		errMu sync.Mutex
		errs  []string
	)
	note := func(format string, args ...any) {
		errMu.Lock()
		errs = append(errs, fmt.Sprintf(format, args...))
		errMu.Unlock()
	}
	check := func(label string, code int, raw []byte, err error) {
		if err != nil {
			note("%s: %v", label, err)
			return
		}
		if code != http.StatusOK {
			note("%s status %d %s", label, code, raw)
			return
		}
		if leak := zeroRankLeak(raw); leak != "" {
			note("%s leaked %s: %s", label, leak, raw)
		}
	}

	sinces := []string{"", "today", "7d", "1d", "all"}
	var wg sync.WaitGroup
	for g := 0; g < 3; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 20; i++ {
				u := srv.URL + "/api/summary"
				if q := sinces[i%len(sinces)]; q != "" {
					u += "?since=" + q
				}
				req, err := http.NewRequest(http.MethodGet, u, nil)
				if err != nil {
					note("summary req: %v", err)
					continue
				}
				code, raw, err := doHTTP(client, req)
				check("GET /api/summary", code, raw, err)
			}
		}()
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 20; i++ {
			body := `{"enabled":true}`
			if i%2 == 0 {
				body = `{"enabled":false}`
			}
			req, err := http.NewRequest(http.MethodPost, srv.URL+"/api/community", strings.NewReader(body))
			if err != nil {
				note("community post req: %v", err)
				continue
			}
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Origin", origin)
			code, raw, err := doHTTP(client, req)
			check("POST /api/community", code, raw, err)
		}
	}()
	for g := 0; g < 2; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 20; i++ {
				req, err := http.NewRequest(http.MethodGet, srv.URL+"/api/community", nil)
				if err != nil {
					note("community get req: %v", err)
					continue
				}
				code, raw, err := doHTTP(client, req)
				check("GET /api/community", code, raw, err)
			}
		}()
	}
	wg.Wait()
	if len(errs) > 0 {
		t.Fatalf("concurrent: %s", strings.Join(errs, "; "))
	}

	off, err := http.NewRequest(http.MethodPost, srv.URL+"/api/community", strings.NewReader(`{"enabled":false}`))
	if err != nil {
		t.Fatal(err)
	}
	off.Header.Set("Content-Type", "application/json")
	off.Header.Set("Origin", origin)
	code, raw, err = doHTTP(client, off)
	if err != nil {
		t.Fatal(err)
	}
	if code != http.StatusOK {
		t.Fatalf("disable %d %s", code, raw)
	}
	if leak := zeroRankLeak(raw); leak != "" {
		t.Fatalf("disable leaked %s: %s", leak, raw)
	}

	mu.Lock()
	afterDisable := usagePosts
	mu.Unlock()
	for i := 0; i < 8; i++ {
		u := srv.URL + "/api/summary"
		if i%2 == 0 {
			u += "?since=today"
		}
		req, err := http.NewRequest(http.MethodGet, u, nil)
		if err != nil {
			t.Fatal(err)
		}
		code, raw, err := doHTTP(client, req)
		if err != nil {
			t.Fatal(err)
		}
		if code != http.StatusOK {
			t.Fatalf("post-disable summary %d %s", code, raw)
		}
		if leak := zeroRankLeak(raw); leak != "" {
			t.Fatalf("post-disable leaked %s: %s", leak, raw)
		}
	}
	mu.Lock()
	afterGets := usagePosts
	mu.Unlock()
	if afterGets != afterDisable {
		t.Fatalf("GET /api/summary uploaded after enabled:false (scan=%d disable=%d get=%d)", afterScan, afterDisable, afterGets)
	}
	if afterDisable != afterScan {
		t.Fatalf("usage POST rose after scan (scan=%d now=%d); GET must not upload", afterScan, afterDisable)
	}
}

func doHTTP(client *http.Client, req *http.Request) (int, []byte, error) {
	res, err := client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer res.Body.Close()
	raw, err := io.ReadAll(res.Body)
	return res.StatusCode, raw, err
}

func zeroRankLeak(raw []byte) string {
	s := string(raw)
	switch {
	case strings.Contains(s, `"rank": 0`), strings.Contains(s, `"rank":0`):
		return `"rank": 0`
	case strings.Contains(s, "#0"):
		return "#0"
	}
	return ""
}

func writeTodayPricedClaude(t *testing.T, dir string) {
	t.Helper()
	dst := filepath.Join(dir, ".claude", "projects", "today-race")
	if err := os.MkdirAll(dst, 0o755); err != nil {
		t.Fatal(err)
	}
	line := fmt.Sprintf(`{"type":"assistant","requestId":"today-race","timestamp":%q,"message":{"id":"today-race","model":"claude-opus-4.6","usage":{"input_tokens":2000,"output_tokens":50,"cache_read_input_tokens":0,"cache_creation_input_tokens":0}}}`, time.Now().Format(time.RFC3339))
	if err := os.WriteFile(filepath.Join(dst, "s.jsonl"), []byte(line+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
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
