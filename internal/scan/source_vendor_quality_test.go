package scan

import (
	"testing"

	"github.com/rainhuang0220/whereToken/internal/event"
	"github.com/rainhuang0220/whereToken/internal/metric"
)

func TestCrossTokenTextKeepsMeasuredZero(t *testing.T) {
	if got := crossTokenText(0, event.QualityDegraded, 0); got != "不可用" {
		t.Fatalf("degraded zero=%q", got)
	}
	if got := crossTokenText(0, event.QualityAbsent, 0); got != "不可用" {
		t.Fatalf("absent=%q", got)
	}
	if got := crossTokenText(0, event.QualityAuthoritative, 0); got != "0.00 M" {
		t.Fatalf("measured zero=%q", got)
	}
	if got := crossTokenText(110, event.QualityDegraded, 110); got != metric.FormatM(110) {
		t.Fatalf("partial=%q", got)
	}
}
