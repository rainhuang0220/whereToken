package scan

import (
	"strings"
	"testing"

	"github.com/rainhuang0220/whereToken/internal/adapter/testhome"
)

func TestRunWithProgressNamesEachSource(t *testing.T) {
	t.Setenv("WHERETOKEN_EXTRA_ROOTS", "")
	adapters := AllAdapters()
	var got []Progress
	_ = RunWithProgress(testhome.New(t.TempDir()), adapters, func(p Progress) {
		got = append(got, p)
	})
	if len(got) == 0 {
		t.Fatal("expected per-source progress events")
	}
	seen := map[string]bool{}
	for _, a := range adapters {
		seen[a.ID()] = false
	}
	for _, p := range got {
		if p.Source == "" {
			t.Fatalf("progress missing source: %+v", p)
		}
		if p.Index < 1 || p.Total != len(adapters) {
			t.Fatalf("index/total %+v want total=%d", p, len(adapters))
		}
		switch p.Status {
		case ProgressReading, ProgressDone, ProgressError:
		default:
			t.Fatalf("status=%q", p.Status)
		}
		if !strings.Contains(p.Label, "正在读") || !strings.HasSuffix(p.Label, "…") {
			t.Fatalf("label=%q", p.Label)
		}
		if _, ok := seen[p.Source]; !ok {
			t.Fatalf("unexpected source %q", p.Source)
		}
		seen[p.Source] = true
	}
	for id, ok := range seen {
		if !ok {
			t.Fatalf("missing progress for %s", id)
		}
	}
}
