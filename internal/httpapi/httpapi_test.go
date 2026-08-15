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

func TestSummaryMatchesScan(t *testing.T) {
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

	resp, err := http.Get(srv.URL + "/api/summary")
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
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatal(err)
	}
	if payload.All.Total != want.Summary.All.Total() {
		t.Fatalf("http total=%d scan total=%d", payload.All.Total, want.Summary.All.Total())
	}
}

func TestSummaryIncludesCalendar(t *testing.T) {
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
	resp, err := http.Get(srv.URL + "/api/summary")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
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

func TestListenRefusesNonLocalhost(t *testing.T) {
	err := Listen("0.0.0.0:8787", testhome.New(t.TempDir()))
	if err == nil {
		t.Fatal("expected refuse")
	}
}
