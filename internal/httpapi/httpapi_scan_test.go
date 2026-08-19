package httpapi

import (
	"bufio"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/rainhuang0220/whereToken/internal/adapter"
	"github.com/rainhuang0220/whereToken/internal/adapter/testhome"
	"github.com/rainhuang0220/whereToken/internal/event"
	"github.com/rainhuang0220/whereToken/internal/scan"
)

func TestGetSummaryReturnsLastScanUntilPost(t *testing.T) {
	t.Setenv("WHERETOKEN_EXTRA_ROOTS", "")
	dir := writeKimiHome(t)
	srv := httptest.NewServer(NewMux(testhome.New(dir)))
	t.Cleanup(srv.Close)

	firstGet := getSummaryJSON(t, srv)
	if firstGet.All.Total != 0 {
		t.Fatalf("GET before first firing must not scan, total=%d", firstGet.All.Total)
	}
	if firstGet.ScannedAt != "" {
		t.Fatalf("unscanned GET must omit scanned_at, got %q", firstGet.ScannedAt)
	}
	if cc := lastCacheControl(t, srv); !strings.Contains(cc, "no-store") {
		t.Fatalf("Cache-Control=%q want no-store", cc)
	}

	posted := postScanJSON(t, srv)
	if posted.All.Total != 1185 {
		t.Fatalf("POST total=%d", posted.All.Total)
	}
	if posted.ScannedAt == "" {
		t.Fatal("POST summary must include scanned_at")
	}

	cached := getSummaryJSON(t, srv)
	if cached.All.Total != 1185 {
		t.Fatalf("GET after firing total=%d", cached.All.Total)
	}

	if err := os.RemoveAll(filepath.Join(dir, ".kimi-code")); err != nil {
		t.Fatal(err)
	}
	stale := getSummaryJSON(t, srv)
	if stale.All.Total != 1185 {
		t.Fatalf("GET must keep last firing, total=%d", stale.All.Total)
	}

	refired := postScanJSON(t, srv)
	if refired.All.Total != 0 {
		t.Fatalf("POST must rescan, total=%d", refired.All.Total)
	}
}

func TestGetSummarySinceFiltersLastScan(t *testing.T) {
	t.Setenv("WHERETOKEN_EXTRA_ROOTS", "")
	dir := writeKimiHome(t)
	srv := httptest.NewServer(NewMux(testhome.New(dir)))
	t.Cleanup(srv.Close)
	posted := postScanJSON(t, srv)
	if posted.All.Total != 1185 {
		t.Fatalf("POST total=%d", posted.All.Total)
	}
	req, err := http.NewRequest(http.MethodGet, srv.URL+"/api/summary?since=1d", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("status=%d", resp.StatusCode)
	}
	var payload struct {
		All struct {
			Total int64 `json:"total"`
		} `json:"all"`
		Compare *struct {
			PreviousTotal int64 `json:"previous_total"`
		} `json:"compare"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	if payload.Compare == nil {
		t.Fatal("ranged summary should include compare")
	}
}

func TestGetSummarySinceTodayIs200(t *testing.T) {
	t.Setenv("WHERETOKEN_EXTRA_ROOTS", "")
	dir := writeKimiHome(t)
	srv := httptest.NewServer(NewMux(testhome.New(dir)))
	t.Cleanup(srv.Close)
	postScanJSON(t, srv)
	req, err := http.NewRequest(http.MethodGet, srv.URL+"/api/summary?since=today", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("dashboard 今日 must not 400: status=%d", resp.StatusCode)
	}
}

func TestPostScanStreamsProgressThenComplete(t *testing.T) {
	t.Setenv("WHERETOKEN_EXTRA_ROOTS", "")
	dir := writeKimiHome(t)
	srv := httptest.NewServer(NewMux(testhome.New(dir)))
	t.Cleanup(srv.Close)

	req, err := http.NewRequest(http.MethodPost, srv.URL+"/api/scan", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Accept", "text/event-stream")
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status=%d", resp.StatusCode)
	}
	if !strings.Contains(resp.Header.Get("Content-Type"), "text/event-stream") {
		t.Fatalf("content-type=%q", resp.Header.Get("Content-Type"))
	}

	events := readSSE(t, resp.Body)
	var sawProgress, sawComplete bool
	for _, ev := range events {
		switch ev.Event {
		case "progress":
			sawProgress = true
			var p scan.Progress
			if err := json.Unmarshal([]byte(ev.Data), &p); err != nil {
				t.Fatalf("progress json: %v", err)
			}
			if p.Source == "" || p.Label == "" || p.Index < 1 || p.Total < 1 {
				t.Fatalf("progress shape %+v", p)
			}
			if !strings.Contains(p.Label, "正在读") {
				t.Fatalf("label=%q", p.Label)
			}
			switch p.Status {
			case scan.ProgressReading, scan.ProgressDone, scan.ProgressError:
			default:
				t.Fatalf("status=%q", p.Status)
			}
		case "complete":
			sawComplete = true
			var payload struct {
				All struct {
					Total int64 `json:"total"`
				} `json:"all"`
			}
			if err := json.Unmarshal([]byte(ev.Data), &payload); err != nil {
				t.Fatalf("complete json: %v", err)
			}
			if payload.All.Total != 1185 {
				t.Fatalf("complete total=%d", payload.All.Total)
			}
		}
	}
	if !sawProgress || !sawComplete {
		t.Fatalf("events=%v", events)
	}
}

func TestGetSummaryReportsScanningDuringOverlap(t *testing.T) {
	t.Setenv("WHERETOKEN_EXTRA_ROOTS", "")
	started := make(chan struct{})
	gate := make(chan struct{})
	mux := NewMuxWith(testhome.New(t.TempDir()), []adapter.Adapter{holdAdapter{started: started, gate: gate}})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	client := srv.Client()

	done := make(chan int, 1)
	go func() {
		resp, err := client.Post(srv.URL+"/api/scan", "application/json", nil)
		if err != nil {
			done <- -1
			return
		}
		defer resp.Body.Close()
		io.Copy(io.Discard, resp.Body)
		done <- resp.StatusCode
	}()

	select {
	case <-started:
	case <-time.After(3 * time.Second):
		t.Fatal("first scan did not start")
	}

	resp, err := client.Get(srv.URL + "/api/summary")
	if err != nil {
		t.Fatal(err)
	}
	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	var mid struct {
		Scanning bool `json:"scanning"`
	}
	if err := json.Unmarshal(body, &mid); err != nil {
		t.Fatal(err)
	}
	if !mid.Scanning {
		t.Fatalf("GET during scan must mark scanning: %s", body)
	}

	close(gate)
	select {
	case code := <-done:
		if code != http.StatusOK {
			t.Fatalf("first scan status=%d", code)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("first scan did not finish")
	}

	after := getSummaryJSON(t, srv)
	if after.Scanning {
		t.Fatal("GET after scan must not stay scanning")
	}
}

func TestPostScanSecondAfterFirstCompletes(t *testing.T) {
	t.Setenv("WHERETOKEN_EXTRA_ROOTS", "")
	dir := writeKimiHome(t)
	srv := httptest.NewServer(NewMux(testhome.New(dir)))
	t.Cleanup(srv.Close)
	first := postScanJSON(t, srv)
	if first.All.Total != 1185 {
		t.Fatalf("first total=%d", first.All.Total)
	}
	second := postScanJSON(t, srv)
	if second.All.Total != 1185 {
		t.Fatalf("second total=%d", second.All.Total)
	}
}

func TestPostScanRejectsOverlap(t *testing.T) {
	t.Setenv("WHERETOKEN_EXTRA_ROOTS", "")
	started := make(chan struct{})
	gate := make(chan struct{})
	mux := NewMuxWith(testhome.New(t.TempDir()), []adapter.Adapter{holdAdapter{started: started, gate: gate}})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	client := srv.Client()
	done := make(chan int, 1)
	go func() {
		resp, err := client.Post(srv.URL+"/api/scan", "application/json", nil)
		if err != nil {
			done <- -1
			return
		}
		defer resp.Body.Close()
		io.Copy(io.Discard, resp.Body)
		done <- resp.StatusCode
	}()

	select {
	case <-started:
	case <-time.After(3 * time.Second):
		t.Fatal("first scan did not start")
	}

	second, err := client.Post(srv.URL+"/api/scan", "application/json", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer second.Body.Close()
	if second.StatusCode != http.StatusConflict {
		t.Fatalf("overlap status=%d want 409", second.StatusCode)
	}
	close(gate)

	select {
	case code := <-done:
		if code != http.StatusOK {
			t.Fatalf("first scan status=%d", code)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("first scan did not finish")
	}
}

type holdAdapter struct {
	started chan struct{}
	gate    chan struct{}
}

func (holdAdapter) ID() string { return "hold" }

func (a holdAdapter) Discover(adapter.Home) []adapter.SourceRoot {
	return []adapter.SourceRoot{{ID: "hold", Path: "hold"}}
}

func (a holdAdapter) Parse(adapter.SourceRoot, func(event.UsageEvent), func(event.TurnEvent)) error {
	select {
	case <-a.started:
	default:
		close(a.started)
	}
	<-a.gate
	return nil
}

type summaryWire struct {
	ScannedAt string `json:"scanned_at"`
	Scanning  bool   `json:"scanning"`
	All       struct {
		Total int64 `json:"total"`
	} `json:"all"`
}

func getSummaryJSON(t *testing.T, srv *httptest.Server) summaryWire {
	t.Helper()
	resp, err := srv.Client().Get(srv.URL + "/api/summary")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET status=%d", resp.StatusCode)
	}
	var out summaryWire
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	return out
}

func lastCacheControl(t *testing.T, srv *httptest.Server) string {
	t.Helper()
	resp, err := srv.Client().Get(srv.URL + "/api/summary")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)
	return resp.Header.Get("Cache-Control")
}

func postScanJSON(t *testing.T, srv *httptest.Server) summaryWire {
	t.Helper()
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
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("POST status=%d body=%s", resp.StatusCode, body)
	}
	var out summaryWire
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	return out
}

type sseEvent struct {
	Event string
	Data  string
}

func readSSE(t *testing.T, r io.Reader) []sseEvent {
	t.Helper()
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 8<<20)
	var events []sseEvent
	var cur sseEvent
	flush := func() {
		if cur.Event == "" && cur.Data == "" {
			return
		}
		events = append(events, cur)
		cur = sseEvent{}
	}
	for sc.Scan() {
		line := sc.Text()
		if line == "" {
			flush()
			continue
		}
		if strings.HasPrefix(line, "event:") {
			cur.Event = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
			continue
		}
		if strings.HasPrefix(line, "data:") {
			cur.Data = strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		}
	}
	flush()
	if err := sc.Err(); err != nil {
		t.Fatal(err)
	}
	return events
}

func writeKimiHome(t *testing.T) string {
	t.Helper()
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
	return dir
}
