package scan

import (
	"strings"
	"testing"

	"github.com/rainhuang0220/whereToken/internal/adapter"
	"github.com/rainhuang0220/whereToken/internal/event"
	"github.com/rainhuang0220/whereToken/internal/metric"
)

func TestDiagnoseUsesScanResultNotASecondDiscovery(t *testing.T) {
	res := Result{
		Roots: []adapter.SourceRoot{{ID: "kimi", Path: "/tmp/fake/.kimi-code"}},
		Summary: metric.Summary{
			BySource: []metric.Slice{
				{ID: "kimi", Label: "Kimi Code", Miss: 10, Quality: event.QualityAuthoritative},
				{ID: "trae", Label: "Trae", Quality: event.QualityAbsent},
			},
		},
		Errors: []string{"trae: 登录态在加密存储中"},
	}
	got := Diagnose(res)
	var kimi, trae, claude AgentStatus
	for _, s := range got {
		switch s.ID {
		case "kimi":
			kimi = s
		case "trae":
			trae = s
		case "claude":
			claude = s
		}
	}
	if !kimi.Detected || !kimi.Usage || kimi.Quality != event.QualityAuthoritative {
		t.Fatalf("kimi %+v", kimi)
	}
	if !strings.Contains(kimi.Path, "/tmp/fake") {
		t.Fatalf("path %q", kimi.Path)
	}
	if trae.Quality != event.QualityAbsent || trae.Usage {
		t.Fatalf("trae %+v", trae)
	}
	if !strings.Contains(trae.Error, "加密") {
		t.Fatalf("trae error %q", trae.Error)
	}
	if claude.Detected {
		t.Fatalf("undiscovered claude must stay undetected: %+v", claude)
	}
}
