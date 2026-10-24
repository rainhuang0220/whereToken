package ledger

import (
	"strings"

	"github.com/rainhuang0220/whereToken/internal/event"
)

// Key is the durable identity of one confirmed usage fact.
//
// A path is not an identity. When RequestID is path-scoped (it contains a
// slash or backslash), the key is source, session id, and the suffix after
// the filename. Codex `.../session.jsonl:7` and the same file under
// archived_sessions therefore share one key. A request id with no slash is
// used whole, because that is the id the source assigned.
//
// Provider API rows and other SkipRequest rows are window aggregates. They
// are not facts: a later response must not be inserted as another request,
// and the scan must not add them on top of local facts.
//
// An empty request id is not invented. The event still flows through the
// scan; it is simply not durable.
func Key(e event.UsageEvent) (string, bool) {
	if e.Derivation == event.DeriveProviderAPI || e.SkipRequest {
		return "", false
	}
	if e.Source == "" || e.RequestID == "" {
		return "", false
	}
	req := e.RequestID
	// A fact merged back into a scan uses sessionID + "\x1f" + request key as
	// its RequestID so two sessions that share a numeric suffix do not collapse
	// in Aggregate. Recognize that shape so the key round-trips.
	if e.SessionID != "" {
		prefix := e.SessionID + "\x1f"
		if strings.HasPrefix(req, prefix) && len(req) > len(prefix) && !strings.ContainsAny(req, `/\`) {
			req = req[len(prefix):]
		}
	}
	if strings.ContainsAny(req, `/\`) {
		if e.SessionID == "" {
			return "", false
		}
		tail, ok := pathTail(req)
		if !ok {
			return "", false
		}
		req = tail
	}
	return e.Source + "\x1f" + e.SessionID + "\x1f" + req, true
}

// pathTail returns the identity suffix of a path-scoped request id.
// `/dir/session.jsonl:7` and `C:\dir\session.jsonl:169:3:100` yield `7` and
// `169:3:100`. A bare path with no suffix is not an identity.
func pathTail(requestID string) (string, bool) {
	i := strings.LastIndexAny(requestID, `/\`)
	if i < 0 || i == len(requestID)-1 {
		return "", false
	}
	seg := requestID[i+1:]
	colon := strings.IndexByte(seg, ':')
	if colon <= 0 || colon == len(seg)-1 {
		return "", false
	}
	tail := seg[colon+1:]
	if tail == "" || strings.ContainsAny(tail, `/\`) {
		return "", false
	}
	return tail, true
}
