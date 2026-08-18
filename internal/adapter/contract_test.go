package adapter_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rainhuang0220/whereToken/internal/adapter"
	"github.com/rainhuang0220/whereToken/internal/adapter/claude"
	"github.com/rainhuang0220/whereToken/internal/adapter/codex"
	"github.com/rainhuang0220/whereToken/internal/adapter/cursor"
	"github.com/rainhuang0220/whereToken/internal/adapter/grok"
	"github.com/rainhuang0220/whereToken/internal/adapter/kimi"
	"github.com/rainhuang0220/whereToken/internal/adapter/minimax"
	"github.com/rainhuang0220/whereToken/internal/adapter/opencode"
	"github.com/rainhuang0220/whereToken/internal/adapter/testhome"
	"github.com/rainhuang0220/whereToken/internal/adapter/trae"
	"github.com/rainhuang0220/whereToken/internal/event"
)

func allAdapters() []adapter.Adapter {
	return []adapter.Adapter{
		claude.Adapter{},
		kimi.Adapter{},
		grok.Adapter{},
		minimax.Adapter{},
		opencode.Adapter{},
		codex.Adapter{},
		cursor.Adapter{Offline: true},
		trae.Adapter{Offline: true},
	}
}

func TestAdaptersHaveIDsAndSafeEmptyDiscover(t *testing.T) {
	home := testhome.New(t.TempDir())
	for _, a := range allAdapters() {
		if strings.TrimSpace(a.ID()) == "" {
			t.Fatalf("%T empty ID", a)
		}
		roots := a.Discover(home)
		for _, r := range roots {
			if r.ID == "" || r.Path == "" {
				t.Fatalf("%s invalid root %+v", a.ID(), r)
			}
		}
	}
}

func TestAdaptersParseEmptyDirWithoutPanic(t *testing.T) {
	root := adapter.SourceRoot{ID: "x", Path: t.TempDir()}
	for _, a := range allAdapters() {
		if err := a.Parse(root, func(event.UsageEvent) {}, func(event.TurnEvent) {}); err != nil && strings.Contains(err.Error(), "sk-") {
			t.Fatalf("%s: %v", a.ID(), err)
		}
	}
}

func TestAdaptersParseMalformedWithoutPanicOrSecrets(t *testing.T) {
	const secret = "sk-leak-fixture-SECRETVALUE99"
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "junk.jsonl"), []byte("{not json\n"+secret+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	root := adapter.SourceRoot{ID: "x", Path: dir, AuthPath: filepath.Join(dir, "auth.json")}
	if err := os.WriteFile(root.AuthPath, []byte(`{"access_token":"`+secret+`"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, a := range allAdapters() {
		var evs []event.UsageEvent
		err := a.Parse(root, func(e event.UsageEvent) {
			evs = append(evs, e)
		}, func(event.TurnEvent) {})
		if err != nil && strings.Contains(err.Error(), secret) {
			t.Fatalf("%s leaked secret in error: %v", a.ID(), err)
		}
		for _, e := range evs {
			blob := e.RequestID + e.Model + e.SessionID + e.Workspace + e.SourceRoot
			if strings.Contains(blob, secret) {
				t.Fatalf("%s leaked secret into event %+v", a.ID(), e)
			}
			if e.Miss < 0 || e.CacheRead < 0 || e.CacheCreate < 0 || e.Output < 0 {
				t.Fatalf("%s negative tokens %+v", a.ID(), e)
			}
		}
	}
}
