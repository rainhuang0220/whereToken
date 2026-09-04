package syncagg

import (
	"testing"

	"github.com/rainhuang0220/whereToken/internal/event"
)

func TestParseScopeAcceptsOnlyEnum(t *testing.T) {
	t.Parallel()
	for _, s := range []string{string(ScopeDeviceLocal), string(ScopeAccountGlobal)} {
		got, err := ParseScope(s)
		if err != nil {
			t.Fatalf("ParseScope(%q): %v", s, err)
		}
		if string(got) != s {
			t.Fatalf("ParseScope(%q)=%q", s, got)
		}
	}
	for _, s := range []string{"", "local", "global", "ACCOUNT_GLOBAL", "device-local"} {
		if _, err := ParseScope(s); err == nil {
			t.Fatalf("ParseScope(%q) accepted", s)
		}
	}
}

func TestScopeOfCursorAPIIsAccountGlobal(t *testing.T) {
	t.Parallel()
	e := event.UsageEvent{
		Source:      "cursor",
		Derivation:  event.DeriveProviderAPI,
		SkipRequest: true,
	}
	if got := ScopeOf(e); got != ScopeAccountGlobal {
		t.Fatalf("ScopeOf(cursor provider_api)=%q", got)
	}
}

func TestScopeOfLocalLedgersIsDeviceLocal(t *testing.T) {
	t.Parallel()
	cases := []event.UsageEvent{
		{Source: "claude", Derivation: event.DeriveRaw},
		{Source: "codex", Derivation: event.DeriveDerived},
		{Source: "grok", Derivation: event.DeriveDerived},
		{Source: "kimi", Derivation: event.DeriveRaw},
		{Source: "minimax", Derivation: event.DeriveRaw},
		{Source: "openclaw", Derivation: event.DeriveRaw},
		{Source: "opencode", Derivation: event.DeriveDerived},
		{Source: "gemini", Derivation: event.DeriveDerived},
		{Source: "qwen", Derivation: event.DeriveDerived},
		{Source: "cline", Derivation: event.DeriveRaw},
		{Source: "roo", Derivation: event.DeriveRaw},
		{Source: "kilo", Derivation: event.DeriveRaw},
		{Source: "zcode", Derivation: event.DeriveDerived},
		{Source: "cursor", Derivation: event.DeriveRaw}, // local bubbles / fallback
		{Source: "trae", Derivation: event.DeriveDerived},
		{Source: "trae", Derivation: event.DeriveProviderAPI},
		{Source: "unknown-tool", Derivation: event.DeriveRaw},
	}
	for _, e := range cases {
		if got := ScopeOf(e); got != ScopeDeviceLocal {
			t.Fatalf("ScopeOf(%s %s)=%q, want device_local", e.Source, e.Derivation, got)
		}
	}
}
