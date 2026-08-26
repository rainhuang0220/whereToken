// Package price estimates API-equivalent USD cost from normalized usage.
//
// This is not a provider invoice and not a subscription bill. Missing or
// unknown prices stay unavailable; they are never rewritten as $0.
//
// Reasoning is not charged. Adapters that already folded reasoning into
// Output are priced on Output only.
package price

import (
	"fmt"
	"strings"
	"time"

	"github.com/rainhuang0220/whereToken/internal/event"
)

const (
	StatusUnavailable = "unavailable"
	StatusPartial     = "partial"
	StatusComplete    = "complete"
)

// CardVersion identifies the baked-in rate table. Historical events use the
// card whose [From, To) contains the timestamp. Undated events use the latest
// card and are marked partial if any sibling is dated.
const CardVersion = "2026-08-19"

// Rate is USD per 1,000,000 tokens.
type Rate struct {
	Vendor, Model                string
	Miss, CacheRead, CacheCreate float64
	Output                       float64
	From, To                     time.Time // To zero = open
	Source, Version              string
	// CreateFree marks a card whose cache write is listed as free (Z.ai
	// "Cached Input Storage: Limited-time Free") rather than unlisted.
	// A free component bills $0; an unlisted one makes the event unpriced.
	CreateFree bool
}

func (r Rate) Contains(t time.Time) bool {
	if !r.From.IsZero() && t.Before(r.From) {
		return false
	}
	if !r.To.IsZero() && !t.Before(r.To) {
		return false
	}
	return true
}

type Charge struct {
	OK                                   bool
	Micro                                int64
	Miss, CacheRead, CacheCreate, Output int64
	Version                              string
}

func Micro(tokens int64, usdPerM float64) int64 {
	if tokens <= 0 || usdPerM <= 0 {
		return 0
	}
	return int64(float64(tokens) * usdPerM)
}

func FormatUSD(micro int64) string {
	neg := micro < 0
	if neg {
		micro = -micro
	}
	s := fmt.Sprintf("$%.4f", float64(micro)/1e6)
	if s == "$0.0000" {
		return ""
	}
	if neg {
		return "-" + s
	}
	return s
}

func Event(e event.UsageEvent) Charge {
	r, ok := Lookup(e.Vendor, e.Model, e.Timestamp)
	if !ok {
		return Charge{}
	}
	if unpricedComponent(e, r) {
		return Charge{}
	}
	miss := Micro(e.Miss, r.Miss)
	cr := Micro(e.CacheRead, r.CacheRead)
	cc := Micro(e.CacheCreate, r.CacheCreate)
	out := Micro(e.Output, r.Output)
	return Charge{
		OK:          true,
		Micro:       miss + cr + cc + out,
		Miss:        miss,
		CacheRead:   cr,
		CacheCreate: cc,
		Output:      out,
		Version:     r.Version,
	}
}

func unpricedComponent(e event.UsageEvent, r Rate) bool {
	return (e.Miss > 0 && r.Miss <= 0) ||
		(e.CacheRead > 0 && r.CacheRead <= 0) ||
		(e.CacheCreate > 0 && r.CacheCreate <= 0 && !r.CreateFree) ||
		(e.Output > 0 && r.Output <= 0)
}

func Lookup(vendor, model string, ts time.Time) (Rate, bool) {
	canon := Canonical(model)
	vend := strings.ToLower(strings.TrimSpace(vendor))
	var best Rate
	found := false
	for _, r := range table {
		if r.Vendor != vend && r.Vendor != "*" {
			continue
		}
		if !matchModel(canon, r.Model) {
			continue
		}
		if !ts.IsZero() && !r.Contains(ts) {
			continue
		}
		if ts.IsZero() && !r.To.IsZero() {
			continue // undated events use the open-ended current card
		}
		if !found || moreSpecific(r, best) {
			best = r
			found = true
		}
	}
	return best, found
}

func moreSpecific(a, b Rate) bool {
	if len(a.Model) != len(b.Model) {
		return len(a.Model) > len(b.Model)
	}
	if a.From.Equal(b.From) {
		return false
	}
	return a.From.After(b.From)
}

func matchModel(canon, pattern string) bool {
	if pattern == "*" {
		return true
	}
	if canon == pattern {
		return true
	}
	for start := 0; start <= len(canon); {
		i := strings.Index(canon[start:], pattern)
		if i < 0 {
			return false
		}
		at := start + i
		if leftBoundaryOK(canon, at, pattern) && rightBoundaryOK(canon, at+len(pattern)) {
			return true
		}
		start = at + 1
	}
	return false
}

func leftBoundaryOK(canon string, at int, pattern string) bool {
	if at == 0 {
		return true
	}
	c := canon[at-1]
	if c == '/' {
		return true
	}
	if c != '-' {
		return false
	}
	// "chatgpt-4o" must not inherit "gpt-4o". A short id like "o3" must not
	// match inside "foo-o3". Multi-part patterns (opus-4.6) may sit after '-'.
	return strings.ContainsAny(pattern, "-.")
}

func rightBoundaryOK(canon string, after int) bool {
	if after >= len(canon) {
		return true
	}
	c := canon[after]
	// After the pattern, '.', '-', or a digit is a different id
	// (opus-4 must not steal opus-4.6; grok-4 must not steal grok-4-fast).
	return c != '.' && c != '-' && (c < '0' || c > '9')
}

func Canonical(model string) string {
	s := strings.ToLower(strings.TrimSpace(model))
	if i := strings.LastIndex(s, "/"); i >= 0 {
		s = s[i+1:]
	}
	s = strings.ReplaceAll(s, "_", "-")
	return foldVersionDots(s)
}

func foldVersionDots(s string) string {
	b := []byte(s)
	for i := 1; i < len(b)-1; i++ {
		if b[i] == '-' && b[i-1] >= '0' && b[i-1] <= '9' && b[i+1] >= '0' && b[i+1] <= '9' {
			b[i] = '.'
		}
	}
	return string(b)
}

func Status(pricedTokens, unpricedTokens int64) string {
	if pricedTokens == 0 && unpricedTokens == 0 {
		return StatusUnavailable
	}
	if unpricedTokens == 0 {
		return StatusComplete
	}
	if pricedTokens == 0 {
		return StatusUnavailable
	}
	return StatusPartial
}
