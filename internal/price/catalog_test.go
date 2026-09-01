package price

import (
	"strings"
	"testing"
	"time"
)

func TestEveryRateHasProvenance(t *testing.T) {
	for _, r := range Rates() {
		meta, ok := SourceFor(r.Source)
		if !ok {
			t.Fatalf("%s/%s: source %q has no provenance", r.Vendor, r.Model, r.Source)
		}
		if meta.Vendor != r.Vendor {
			t.Fatalf("%s: provenance vendor %q", r.Model, meta.Vendor)
		}
		if !strings.HasPrefix(meta.URL, "https://") {
			t.Fatalf("%s: source must be an official https page, got %q", r.Model, meta.URL)
		}
		if _, err := time.Parse("2006-01-02", meta.Verified); err != nil {
			t.Fatalf("%s: verified %q is not a date", r.Model, meta.Verified)
		}
	}
}

func TestEveryVendorInTableHasASource(t *testing.T) {
	have := map[string]bool{}
	for _, s := range Sources() {
		have[s.Vendor] = true
	}
	for _, r := range Rates() {
		if !have[r.Vendor] {
			t.Fatalf("vendor %q has rows but no source metadata", r.Vendor)
		}
	}
}

func TestEveryRowPricesInputAndOutput(t *testing.T) {
	// The pricing catalog prints input/output unconditionally; a row that
	// leaves either unlisted would render as unknown there.
	for _, r := range Rates() {
		if r.Miss <= 0 || r.Output <= 0 {
			t.Fatalf("%s/%s: input and output must be listed (miss=%v output=%v)", r.Vendor, r.Model, r.Miss, r.Output)
		}
	}
}

func TestMatchModelKeepsLookupBoundaries(t *testing.T) {
	if !MatchModel(Canonical("claude-opus-4-1"), "opus-4.1") {
		t.Fatal("provider-prefixed id must match its family row")
	}
	if MatchModel(Canonical("chatgpt-4o"), "gpt-4o") {
		t.Fatal("chatgpt-4o must not inherit gpt-4o")
	}
	if MatchModel(Canonical("opus-4"), "opus-4.6") {
		t.Fatal("shorter id must not steal a longer row")
	}
}

func TestRatesReturnsACopy(t *testing.T) {
	a := Rates()
	b := Rates()
	if len(a) == 0 || &a[0] == &b[0] {
		t.Fatal("Rates must hand out an independent slice")
	}
}
