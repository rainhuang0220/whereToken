package cli

import (
	"strings"
	"testing"

	"github.com/rainhuang0220/whereToken/internal/adapter"
	"github.com/rainhuang0220/whereToken/internal/event"
	"github.com/rainhuang0220/whereToken/internal/metric"
	"github.com/rainhuang0220/whereToken/internal/scan"
)

func TestParseDoctor(t *testing.T) {
	f, err := Parse([]string{"doctor"})
	if err != nil {
		t.Fatal(err)
	}
	if f.Command != CommandDoctor {
		t.Fatalf("cmd=%q", f.Command)
	}
}

func TestRunDoctorExplainsQualityFromScan(t *testing.T) {
	app, out, errb := testApp([]string{"doctor"})
	app.Scan = func(adapter.Home) scan.Result {
		return scan.Result{
			Roots: []adapter.SourceRoot{{ID: "kimi", Path: "/tmp/fake/.kimi-code"}},
			Summary: metric.Summary{
				BySource: []metric.Slice{
					{ID: "kimi", Label: "Kimi Code", Miss: 10, Quality: event.QualityAuthoritative},
					{ID: "cursor", Label: "Cursor", Quality: event.QualityDegraded},
				},
			},
			Errors: []string{"cursor: token 列不完整（该工具需要已登录）"},
		}
	}
	if code := app.Run(); code != ExitOK {
		t.Fatalf("code=%d %s", code, errb.String())
	}
	s := out.String()
	if !strings.Contains(s, "Kimi Code") || !strings.Contains(s, "authoritative") {
		t.Fatalf("doctor must reuse scan quality:\n%s", s)
	}
	if !strings.Contains(s, "Cursor") || !strings.Contains(s, "degraded") {
		t.Fatalf("doctor must show degraded cursor:\n%s", s)
	}
	if !strings.Contains(s, "已登录") && !strings.Contains(s, "sign") {
		t.Fatalf("doctor must carry the scan error:\n%s", s)
	}
}

func TestRunSourcesPrintsQualityTable(t *testing.T) {
	app, out, errb := testApp([]string{"sources"})
	app.Scan = func(adapter.Home) scan.Result {
		return scan.Result{
			Roots: []adapter.SourceRoot{{ID: "kimi", Path: "/tmp/fake/.kimi-code"}},
			Summary: metric.Summary{
				BySource: []metric.Slice{{ID: "kimi", Label: "Kimi Code", Miss: 1, Quality: event.QualityAuthoritative}},
			},
		}
	}
	if code := app.Run(); code != ExitOK {
		t.Fatalf("code=%d %s", code, errb.String())
	}
	s := out.String()
	if !strings.Contains(s, "kimi") || !strings.Contains(s, "/tmp/fake") {
		t.Fatalf("%s", s)
	}
	if !strings.Contains(s, "authoritative") {
		t.Fatalf("sources should show quality:\n%s", s)
	}
}
