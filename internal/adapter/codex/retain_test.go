package codex

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/rainhuang0220/whereToken/internal/adapter"
	"github.com/rainhuang0220/whereToken/internal/event"
	"github.com/rainhuang0220/whereToken/internal/index"
)

func bindIndex(t *testing.T, dir string) {
	t.Helper()
	st, err := index.Open(filepath.Join(dir, "index.v1.db"))
	if err != nil {
		t.Fatal(err)
	}
	undo := index.Use(st)
	t.Cleanup(func() {
		undo()
		st.Close()
	})
}

func writeRollout(t *testing.T, path string, input, output int64) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	body := fmt.Sprintf("{\"timestamp\":\"2026-01-01T00:00:00Z\",\"type\":\"event_msg\",\"payload\":{\"type\":\"token_count\",\"info\":{\"total_token_usage\":{\"input_tokens\":%d,\"output_tokens\":%d}}}}\n", input, output)
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func parseSum(t *testing.T, dir string) int64 {
	t.Helper()
	var sum int64
	if err := (Adapter{}).Parse(adapter.SourceRoot{ID: "codex", Path: dir}, func(e event.UsageEvent) {
		sum += e.Miss + e.CacheRead + e.CacheCreate + e.Output
	}, func(event.TurnEvent) {}); err != nil {
		t.Fatal(err)
	}
	return sum
}

func TestDeletedRolloutStaysInTheScan(t *testing.T) {
	dir := t.TempDir()
	bindIndex(t, dir)
	old := filepath.Join(dir, "sessions", "2026", "10", "01", "rollout-old.jsonl")
	writeRollout(t, old, 100, 10)
	if got := parseSum(t, dir); got != 110 {
		t.Fatalf("first %d", got)
	}
	if err := os.Remove(old); err != nil {
		t.Fatal(err)
	}
	if got := parseSum(t, dir); got != 110 {
		t.Fatalf("deleted rollout dropped tokens: %d", got)
	}
	live := filepath.Join(dir, "sessions", "2026", "10", "23", "rollout-new.jsonl")
	writeRollout(t, live, 50, 5)
	if got := parseSum(t, dir); got != 165 {
		t.Fatalf("new rollout %d", got)
	}
}

func TestPresentRolloutIsNotDoubledFromCache(t *testing.T) {
	dir := t.TempDir()
	bindIndex(t, dir)
	path := filepath.Join(dir, "sessions", "2026", "10", "01", "rollout-live.jsonl")
	writeRollout(t, path, 80, 20)
	if got := parseSum(t, dir); got != 100 {
		t.Fatalf("first %d", got)
	}
	if got := parseSum(t, dir); got != 100 {
		t.Fatalf("second %d", got)
	}
}

func TestAbsentRolloutWithoutIndexIsNotInvented(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sessions", "2026", "10", "01", "rollout-old.jsonl")
	writeRollout(t, path, 100, 10)
	if got := parseSum(t, dir); got != 110 {
		t.Fatalf("first %d", got)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if got := parseSum(t, dir); got != 0 {
		t.Fatalf("missing index invented %d", got)
	}
}

func TestDeletedRolloutDoesNotReplayOverLiveSession(t *testing.T) {
	dir := t.TempDir()
	bindIndex(t, dir)
	archived := filepath.Join(dir, "archived_sessions", "rollout-same.jsonl")
	writeRollout(t, archived, 100, 10)
	if got := parseSum(t, dir); got != 110 {
		t.Fatalf("archived %d", got)
	}
	if err := os.Remove(archived); err != nil {
		t.Fatal(err)
	}
	live := filepath.Join(dir, "sessions", "2026", "10", "02", "rollout-same.jsonl")
	writeRollout(t, live, 7, 1)
	if got := parseSum(t, dir); got != 8 {
		t.Fatalf("overlap %d", got)
	}
}
