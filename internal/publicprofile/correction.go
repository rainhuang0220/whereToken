package publicprofile

import (
	"context"
	"strconv"
	"strings"
)

// Correction is the one unpublished net drop recorded by the ledger.
// Reason is the latest correction reason. Delta is a positive token count.
// It is not a second revision log.
type Correction struct {
	Reason string
	Delta  int64
}

// Explains reports whether this permit is exactly the drop from prev to next.
// A missing permit does not explain a drop, and it is not treated as zero.
func (c Correction) Explains(prev, next int64) bool {
	return strings.TrimSpace(c.Reason) != "" && c.Delta > 0 && next < prev && prev-next == c.Delta
}

// CorrectionAllows is the server check for the two correction headers.
// The reason is trimmed, must be non-empty, at most 200 bytes, and must not
// contain a carriage return or line feed. The delta must be the exact drop.
func CorrectionAllows(reason, deltaText string, prev, next int64) bool {
	reason = strings.TrimSpace(reason)
	if reason == "" || len(reason) > 200 || strings.ContainsAny(reason, "\r\n") {
		return false
	}
	delta, err := strconv.ParseInt(strings.TrimSpace(deltaText), 10, 64)
	if err != nil || delta <= 0 {
		return false
	}
	return (Correction{Reason: reason, Delta: delta}).Explains(prev, next)
}

type correctionKey struct{}

// ContextWithCorrection carries a permit into DecidePublication and the PUT.
// An empty reason or a non-positive delta leaves the context unchanged.
func ContextWithCorrection(ctx context.Context, c Correction) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	c.Reason = strings.TrimSpace(c.Reason)
	if c.Reason == "" || c.Delta <= 0 {
		return ctx
	}
	return context.WithValue(ctx, correctionKey{}, c)
}

// CorrectionFrom reads the permit attached to ctx. A missing permit is the
// zero value.
func CorrectionFrom(ctx context.Context) Correction {
	if ctx == nil {
		return Correction{}
	}
	c, _ := ctx.Value(correctionKey{}).(Correction)
	return c
}
