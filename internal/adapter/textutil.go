package adapter

import (
	"strings"
	"time"
)

// ParseTS parses an RFC3339/RFC3339Nano timestamp and normalizes it to UTC.
// Empty or unparseable input yields the zero time; a missing timestamp is
// not an error.
func ParseTS(s string) time.Time {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}
	}
	if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
		return t.UTC()
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t.UTC()
	}
	return time.Time{}
}

// Clamp0 floors a negative token count at zero. A negative ledger value is
// corrupt, not a negative charge.
func Clamp0(n int64) int64 {
	if n < 0 {
		return 0
	}
	return n
}
