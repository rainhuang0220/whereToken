package httpapi

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/rainhuang0220/whereToken/internal/adapter/testhome"
	"github.com/rainhuang0220/whereToken/internal/scan"
)

func TestNewHTTPServerSetsReadHeaderTimeout(t *testing.T) {
	s := NewHTTPServer("127.0.0.1:0", testhome.New(t.TempDir()), false)
	if s.ReadHeaderTimeout <= 0 {
		t.Fatal("ReadHeaderTimeout must be set")
	}
	if s.Addr != "127.0.0.1:0" {
		t.Fatalf("addr=%s", s.Addr)
	}
}

func TestNewHTTPServerOfflineStillServes(t *testing.T) {
	s := NewHTTPServer("127.0.0.1:0", testhome.New(t.TempDir()), true)
	if s.Handler == nil {
		t.Fatal("offline server needs a handler")
	}
}

func TestSummaryMatchesScan(t *testing.T) {
	t.Setenv("WHERETOKEN_EXTRA_ROOTS", "")
	dir := t.TempDir()
	dstDir := filepath.Join(dir, ".kimi-code", "sessions", "x", "s", "agents", "main")
	if err := os.MkdirAll(dstDir, 0o755); err != nil {
		t.Fatal(err)
	}
	_, file, _, _ := runtime.Caller(0)
	src := filepath.Join(filepath.Dir(file), "..", "..", "testdata", "adapters", "kimi", "session", "agents", "main", "wire.jsonl")
	data, err := os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dstDir, "wire.jsonl"), data, 0o644); err != nil {
		t.Fatal(err)
	}

	home := testhome.New(dir)
	want := scan.Run(home, scan.AllAdapters())
	srv := httptest.NewServer(NewMux(home))
	defer srv.Close()

	posted := postScanJSON(t, srv)
	if posted.All.Total != want.Summary.All.Total() {
		t.Fatalf("http total=%d scan total=%d", posted.All.Total, want.Summary.All.Total())
	}
}

func TestSummaryIncludesCalendar(t *testing.T) {
	t.Setenv("WHERETOKEN_EXTRA_ROOTS", "")
	dir := t.TempDir()
	dstDir := filepath.Join(dir, ".kimi-code", "sessions", "x", "s", "agents", "main")
	if err := os.MkdirAll(dstDir, 0o755); err != nil {
		t.Fatal(err)
	}
	_, file, _, _ := runtime.Caller(0)
	src := filepath.Join(filepath.Dir(file), "..", "..", "testdata", "adapters", "kimi", "session", "agents", "main", "wire.jsonl")
	data, err := os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dstDir, "wire.jsonl"), data, 0o644); err != nil {
		t.Fatal(err)
	}

	srv := httptest.NewServer(NewMux(testhome.New(dir)))
	defer srv.Close()
	req, err := http.NewRequest(http.MethodPost, srv.URL+"/api/scan", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Accept", "application/json")
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status=%d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	var payload struct {
		All struct {
			Total int64 `json:"total"`
		} `json:"all"`
		Calendar struct {
			WeekStart string `json:"week_start"`
			All       struct {
				Days []struct {
					Total int64 `json:"total"`
				} `json:"days"`
				Stats struct {
					PeakTotalM string `json:"peak_total_m"`
				} `json:"stats"`
			} `json:"all"`
		} `json:"calendar"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Calendar.WeekStart != "monday" {
		t.Fatalf("week_start=%q", payload.Calendar.WeekStart)
	}
	if payload.Calendar.All.Stats.PeakTotalM == "" {
		t.Fatal("missing peak_total_m")
	}
	var daySum int64
	for _, d := range payload.Calendar.All.Days {
		daySum += d.Total
	}
	if daySum != payload.All.Total {
		t.Fatalf("calendar days=%d all=%d", daySum, payload.All.Total)
	}
}

func TestMuxSetsNoSniffAndDenyFrame(t *testing.T) {
	srv := httptest.NewServer(NewMux(testhome.New(t.TempDir())))
	defer srv.Close()
	resp, err := srv.Client().Get(srv.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.Header.Get("X-Content-Type-Options") != "nosniff" {
		t.Fatalf("nosniff=%q", resp.Header.Get("X-Content-Type-Options"))
	}
	if resp.Header.Get("X-Frame-Options") != "DENY" {
		t.Fatalf("frame=%q", resp.Header.Get("X-Frame-Options"))
	}
	if resp.Header.Get("Referrer-Policy") != "no-referrer" {
		t.Fatalf("referrer=%q", resp.Header.Get("Referrer-Policy"))
	}
}

func TestListenRefusesNonLocalhost(t *testing.T) {
	err := Listen("0.0.0.0:8787", testhome.New(t.TempDir()))
	if err == nil {
		t.Fatal("expected refuse")
	}
}

func TestSPAFallbackServesThemes(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte("<!doctype html>SPA"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "asset.js"), []byte("js"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("WHERETOKEN_WEB", dir)

	srv := httptest.NewServer(NewMux(testhome.New(t.TempDir())))
	defer srv.Close()

	resp, err := srv.Client().Get(srv.URL + "/themes")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("themes status=%d", resp.StatusCode)
	}
	if string(body) != "<!doctype html>SPA" {
		t.Fatalf("themes body=%q", body)
	}

	asset, err := srv.Client().Get(srv.URL + "/asset.js")
	if err != nil {
		t.Fatal(err)
	}
	defer asset.Body.Close()
	js, err := io.ReadAll(asset.Body)
	if err != nil {
		t.Fatal(err)
	}
	if string(js) != "js" {
		t.Fatalf("asset body=%q", js)
	}

	missing, err := srv.Client().Get(srv.URL + "/missing.js")
	if err != nil {
		t.Fatal(err)
	}
	defer missing.Body.Close()
	if missing.StatusCode != http.StatusNotFound {
		t.Fatalf("missing asset status=%d", missing.StatusCode)
	}
}

func TestWebDistIgnoresCwdFolder(t *testing.T) {
	dir := t.TempDir()
	hijack := filepath.Join(dir, "web", "dist")
	if err := os.MkdirAll(hijack, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(hijack, "index.html"), []byte("<h1>hijack</h1>"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)
	t.Setenv("WHERETOKEN_WEB", "")
	if got := webDist(); got != "" {
		t.Fatalf("cwd web/dist must not replace the embed: %s", got)
	}
}

func TestSummaryBeforeScanHasCalendarWindow(t *testing.T) {
	t.Setenv("WHERETOKEN_EXTRA_ROOTS", "")
	srv := httptest.NewServer(NewMux(testhome.New(t.TempDir())))
	defer srv.Close()
	resp, err := srv.Client().Get(srv.URL + "/api/summary")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	var payload struct {
		ScannedAt string `json:"scanned_at"`
		Calendar  struct {
			WeekStart  string `json:"week_start"`
			WindowFrom string `json:"window_from"`
			WindowTo   string `json:"window_to"`
		} `json:"calendar"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatal(err)
	}
	if payload.ScannedAt != "" {
		t.Fatalf("unsanned summary must not pretend it was scanned: %s", payload.ScannedAt)
	}
	if payload.Calendar.WeekStart != "monday" || payload.Calendar.WindowFrom == "" || payload.Calendar.WindowTo == "" {
		t.Fatalf("cold GET /api/summary must still describe the kiln window: %s", body)
	}
}

func TestLocalHost(t *testing.T) {
	cases := []struct {
		host string
		ok   bool
	}{
		{"127.0.0.1:8787", true},
		{"localhost:8787", true},
		{"[::1]:8787", true},
		{"", true},
		{"evil.example", false},
		{"0.0.0.0:8787", false},
		{"192.168.1.2:8787", false},
	}
	for _, c := range cases {
		req, err := http.NewRequest(http.MethodGet, "http://"+c.host+"/api/summary", nil)
		if err != nil {
			t.Fatal(err)
		}
		req.Host = c.host
		if got := localHost(req); got != c.ok {
			t.Fatalf("host %q got %v want %v", c.host, got, c.ok)
		}
	}
}

func TestScanRejectsNonLocalHost(t *testing.T) {
	t.Setenv("WHERETOKEN_EXTRA_ROOTS", "")
	srv := httptest.NewServer(NewMux(testhome.New(t.TempDir())))
	defer srv.Close()
	req, err := http.NewRequest(http.MethodPost, srv.URL+"/api/scan", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Host = "evil.example"
	req.Header.Set("Accept", "application/json")
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status=%d body want 403 for foreign Host", resp.StatusCode)
	}
}

func TestOfflineScanJSONMarksOffline(t *testing.T) {
	t.Setenv("WHERETOKEN_EXTRA_ROOTS", "")
	mux := NewMuxWith(testhome.New(t.TempDir()), scan.Adapters(true))
	srv := httptest.NewServer(mux)
	defer srv.Close()
	req, err := http.NewRequest(http.MethodPost, srv.URL+"/api/scan", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Accept", "application/json")
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	var payload struct {
		Offline bool `json:"offline"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatal(err)
	}
	if !payload.Offline {
		t.Fatalf("offline mux scan JSON must mark offline: %s", body)
	}
}
