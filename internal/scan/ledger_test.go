package scan

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/rainhuang0220/whereToken/internal/adapter"
	"github.com/rainhuang0220/whereToken/internal/adapter/testhome"
	"github.com/rainhuang0220/whereToken/internal/event"
	"github.com/rainhuang0220/whereToken/internal/index"
	"github.com/rainhuang0220/whereToken/internal/ledger"
)

type staticAdapter struct {
	id     string
	events []event.UsageEvent
}

func (s staticAdapter) ID() string { return s.id }

func (s staticAdapter) Discover(adapter.Home) []adapter.SourceRoot {
	if len(s.events) == 0 {
		return nil
	}
	return []adapter.SourceRoot{{ID: s.id, Path: "mem:" + s.id}}
}

func (s staticAdapter) Parse(_ adapter.SourceRoot, emit func(event.UsageEvent), _ func(event.TurnEvent)) error {
	for _, e := range s.events {
		emit(e)
	}
	return nil
}

func TestRebuildKeepsImportedHistory(t *testing.T) {
	root := t.TempDir()
	home := testhome.New(root)
	idx := index.PathFor(home)
	st, err := index.Open(idx)
	if err != nil {
		t.Fatal(err)
	}
	src := filepath.Join(root, "sessions", "abc.jsonl")
	if err := os.MkdirAll(filepath.Dir(src), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(src, []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	ev := event.UsageEvent{
		Source: "codex", SessionID: "abc", RequestID: src + ":1",
		Model: "m", Vendor: "openai", Miss: 10, Output: 1,
		Quality: event.QualityAuthoritative, Derivation: event.DeriveDerived,
	}
	if _, _, _, err := st.LoadOrParse("codex", src, func(f *os.File) ([]event.UsageEvent, []event.TurnEvent, int64, error) {
		info, err := f.Stat()
		if err != nil {
			return nil, nil, 0, err
		}
		return []event.UsageEvent{ev}, nil, info.Size(), nil
	}); err != nil {
		t.Fatal(err)
	}
	st.Close()
	if err := os.Remove(src); err != nil {
		t.Fatal(err)
	}

	first := Run(home, nil)
	if first.Summary.All.Total() != 11 {
		t.Fatalf("import total %d", first.Summary.All.Total())
	}
	if err := index.Wipe(idx); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(idx); !os.IsNotExist(err) {
		t.Fatalf("index still present %v", err)
	}
	lp, err := ledger.PathFor(home)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(lp); err != nil {
		t.Fatal(err)
	}
	second := Run(home, nil)
	if second.Summary.All.Total() != 11 {
		t.Fatalf("rebuild dropped history %d", second.Summary.All.Total())
	}
	raw, err := os.ReadFile(lp)
	if err != nil {
		t.Fatal(err)
	}
	if len(raw) == 0 {
		t.Fatal("empty ledger")
	}
}

func TestScanDoesNotDoubleCountOrSumTheAccountAPI(t *testing.T) {
	home := testhome.New(t.TempDir())
	local := []event.UsageEvent{{
		Source: "cursor", SessionID: "s", RequestID: "bubble-1",
		Model: "m", Miss: 100, Quality: event.QualityDegraded, Derivation: event.DeriveRaw,
	}}
	first := Run(home, []adapter.Adapter{staticAdapter{id: "cursor", events: local}})
	if first.Summary.All.Total() != 100 {
		t.Fatalf("local %d", first.Summary.All.Total())
	}
	second := Run(home, []adapter.Adapter{staticAdapter{id: "cursor", events: local}})
	if second.Summary.All.Total() != 100 {
		t.Fatalf("rescan %d", second.Summary.All.Total())
	}
	api := []event.UsageEvent{{
		Source: "cursor", SessionID: "s", RequestID: "cursor-api:s:1",
		Model: "m", Miss: 40, Quality: event.QualityAuthoritative,
		Derivation: event.DeriveProviderAPI, SkipRequest: true,
	}}
	third := Run(home, []adapter.Adapter{staticAdapter{id: "cursor", events: api}})
	if third.Summary.All.Total() != 40 {
		t.Fatalf("api plus ledger %d", third.Summary.All.Total())
	}
}

func TestScanKeepsFactsTheAdapterNoLongerEmits(t *testing.T) {
	home := testhome.New(t.TempDir())
	all := make([]event.UsageEvent, 100)
	for i := range all {
		all[i] = event.UsageEvent{
			Source: "codex", SessionID: "s" + itoa(i), RequestID: "req-" + itoa(i),
			Model: "m", Miss: 10, Output: 1,
			Quality: event.QualityAuthoritative, Derivation: event.DeriveRaw,
		}
	}
	full := Run(home, []adapter.Adapter{staticAdapter{id: "codex", events: all}})
	if full.Summary.All.Total() != 1100 {
		t.Fatalf("full %d", full.Summary.All.Total())
	}
	partial := Run(home, []adapter.Adapter{staticAdapter{id: "codex", events: all[:70]}})
	if partial.Summary.All.Total() != 1100 {
		t.Fatalf("partial %d", partial.Summary.All.Total())
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [16]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}
