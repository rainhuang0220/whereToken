package profile

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/rainhuang0220/whereToken/internal/adapter"
	"github.com/rainhuang0220/whereToken/internal/adapter/testhome"
	"github.com/rainhuang0220/whereToken/internal/community"
)

// FallbackSeed seeds portraits when no install identity is available:
// in-memory results without a home (tests, synthetic payloads) or an
// unwritable config dir. A constant keeps those deterministic. Never "".
const FallbackSeed = "anonymous"

// installIDRe accepts the UUIDv4 layout community.NewParticipantID emits.
var installIDRe = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

// Identity resolves the anonymous install identity for a home directory.
// It never returns "" and never touches PII: the id is a random UUID, and
// the username/HOME/hostname are never read, let alone mixed into the seed.
func Identity(home string) string {
	if strings.TrimSpace(home) == "" {
		return FallbackSeed
	}
	return IdentityFor(testhome.New(home))
}

// IdentityFor is Identity over an adapter.Home, so callers that already
// hold one (the scanner) share community.ConfigPath's exact path logic —
// WHERETOKEN_COMMUNITY_FILE, XDG_CONFIG_HOME, Windows AppData included.
//
// Resolution order: an existing community participant_id identifies the
// install; otherwise <config>/install-id is read, or created (0600,
// crypto/rand UUIDv4) on first use. If the id can be generated but not
// persisted, the constant FallbackSeed keeps the portrait stable instead of
// re-rolling a fresh id on every call.
func IdentityFor(home adapter.Home) string {
	if home == nil {
		return FallbackSeed
	}
	cfg := community.ConfigPath(home)
	if f, err := community.Load(cfg); err == nil && f.ParticipantID != "" {
		return f.ParticipantID
	}
	dir := filepath.Dir(cfg)
	path := filepath.Join(dir, "install-id")
	if raw, err := os.ReadFile(path); err == nil {
		if id := strings.TrimSpace(string(raw)); installIDRe.MatchString(id) {
			return id
		}
	}
	id, err := community.NewParticipantID()
	if err != nil {
		return FallbackSeed
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return FallbackSeed
	}
	if err := os.WriteFile(path, []byte(id+"\n"), 0o600); err != nil {
		return FallbackSeed
	}
	return id
}
