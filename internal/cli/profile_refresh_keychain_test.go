package cli

import (
	"testing"

	"github.com/rainhuang0220/whereToken/internal/adapter/cursor"
	"github.com/rainhuang0220/whereToken/internal/scan"
)

// Public Profile checks must never enter a GUI-gated keychain call. Explicit
// user-initiated scans retain their normal cursor.Adapter configuration.
func TestProfileRefreshDisablesCursorOSKeychainOnly(t *testing.T) {
	ads := scan.Adapters(false)
	var found int
	for _, item := range ads {
		if c, ok := item.(cursor.Adapter); ok {
			found++
			if c.NoOSKeychain {
				t.Fatal("ordinary scan disabled Cursor keychain")
			}
		}
	}
	if found != 1 {
		t.Fatalf("expected exactly one Cursor adapter, got %d", found)
	}
	updated := prohibitCursorOSKeychain(ads)
	if len(updated) != len(ads) {
		t.Fatal("lost adapters")
	}
	for i, item := range updated {
		before := ads[i]
		c, ok := item.(cursor.Adapter)
		if !ok {
			if item.ID() != before.ID() {
				t.Fatalf("other adapter %d changed: %s -> %s", i, before.ID(), item.ID())
			}
			continue
		}
		prev := before.(cursor.Adapter)
		if !c.NoOSKeychain || c.Offline != prev.Offline {
			t.Fatalf("Cursor mode not safely scoped: %+v", c)
		}
		if prev.NoOSKeychain {
			t.Fatal("mutated original adapter registry")
		}
	}
}
