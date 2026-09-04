package syncagg

import (
	"fmt"

	"github.com/rainhuang0220/whereToken/internal/event"
)

type SourceScope string

const (
	ScopeDeviceLocal    SourceScope = "device_local"
	ScopeAccountGlobal  SourceScope = "account_global"
	SchemaVersion       = 1
)

func ParseScope(s string) (SourceScope, error) {
	switch SourceScope(s) {
	case ScopeDeviceLocal, ScopeAccountGlobal:
		return SourceScope(s), nil
	default:
		return "", fmt.Errorf("invalid source_scope %q", s)
	}
}

// ScopeOf is the adapter/domain rule. Only Cursor's account usage API is
// account-global; everything else, including Trae, stays device-local.
func ScopeOf(e event.UsageEvent) SourceScope {
	if e.Source == "cursor" && e.Derivation == event.DeriveProviderAPI {
		return ScopeAccountGlobal
	}
	return ScopeDeviceLocal
}
