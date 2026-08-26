package adapter_test

import (
	"encoding/json"
	"testing"

	"github.com/rainhuang0220/whereToken/internal/adapter"
)

// FlexInt semantics: numeric strings parse strictly (all-or-nothing, no
// partial prefix); scientific notation and floats count; junk counts as
// zero. Token counters arrive as JSON numbers or strings depending on the
// provider API.
func TestFlexIntBoundaries(t *testing.T) {
	cases := []struct {
		name string
		in   any
		want int64
	}{
		{"nil", nil, 0},
		{"int", 7, 7},
		{"int64", int64(8), 8},
		{"float64 truncates", 2.9, 2},
		{"bool", true, 0},

		{"string int", "123", 123},
		{"string zero", "0", 0},
		{"string negative", "-123", -123},
		{"string padded", " 42 ", 42},
		{"string float truncates", "1.5", 1},
		{"string scientific", "1e12", 1_000_000_000_000},
		{"string junk suffix stays zero", "123abc", 0},
		{"string empty", "", 0},
		{"string null word", "null", 0},
		{"string non numeric", "abc", 0},
		{"string date is not a number", "2026-01-01T00:00:00Z", 0},

		{"json number int", json.Number("123"), 123},
		{"json number float", json.Number("1.5"), 1},
		{"json number scientific", json.Number("1e12"), 1_000_000_000_000},
		{"json number junk", json.Number("abc"), 0},
		{"json number empty", json.Number(""), 0},
	}
	for _, c := range cases {
		if got := adapter.FlexInt(c.in); got != c.want {
			t.Errorf("%s: FlexInt(%v)=%d want %d", c.name, c.in, got, c.want)
		}
	}
}
