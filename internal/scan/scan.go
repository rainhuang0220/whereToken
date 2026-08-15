package scan

import (
	"encoding/json"
	"io"
	"os"
	"strings"
	"time"

	"github.com/rainhuang0220/whereToken/internal/adapter"
	"github.com/rainhuang0220/whereToken/internal/adapter/claude"
	"github.com/rainhuang0220/whereToken/internal/adapter/codex"
	"github.com/rainhuang0220/whereToken/internal/adapter/cursor"
	"github.com/rainhuang0220/whereToken/internal/adapter/kimi"
	"github.com/rainhuang0220/whereToken/internal/adapter/opencode"
	"github.com/rainhuang0220/whereToken/internal/adapter/testhome"
	"github.com/rainhuang0220/whereToken/internal/adapter/trae"
	"github.com/rainhuang0220/whereToken/internal/event"
	"github.com/rainhuang0220/whereToken/internal/metric"
)

type Result struct {
	Summary   metric.Summary
	Roots     []adapter.SourceRoot
	Errors    []string
	Events    []event.UsageEvent
	Turns     []event.TurnEvent
	ScannedAt time.Time
}

const (
	ProgressReading = "reading"
	ProgressDone    = "done"
	ProgressError   = "error"
)

type Progress struct {
	Source string `json:"source"`
	Label  string `json:"label"`
	Index  int    `json:"index"`
	Total  int    `json:"total"`
	Status string `json:"status"`
}

func readingLabel(id string) string {
	return "正在读 " + metric.SourceLabel(id) + "…"
}

func AllAdapters() []adapter.Adapter {
	return []adapter.Adapter{
		claude.Adapter{},
		kimi.Adapter{},
		opencode.Adapter{},
		codex.Adapter{},
		cursor.Adapter{},
		trae.Adapter{},
	}
}

func extraHomes() []adapter.Home {
	raw := strings.TrimSpace(os.Getenv("WHERETOKEN_EXTRA_ROOTS"))
	if raw == "" {
		return nil
	}
	parts := strings.FieldsFunc(raw, func(r rune) bool {
		return r == rune(os.PathListSeparator) || r == ','
	})
	var out []adapter.Home
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		out = append(out, testhome.New(p))
	}
	return out
}

func Run(home adapter.Home, adapters []adapter.Adapter) Result {
	return RunWithProgress(home, adapters, nil)
}

func RunWithProgress(home adapter.Home, adapters []adapter.Adapter, report func(Progress)) Result {
	var events []event.UsageEvent
	var turns []event.TurnEvent
	var roots []adapter.SourceRoot
	var errs []string
	seenPath := map[string]struct{}{}
	homes := append([]adapter.Home{home}, extraHomes()...)
	total := len(adapters)
	for i, a := range adapters {
		if report != nil {
			report(Progress{
				Source: a.ID(),
				Label:  readingLabel(a.ID()),
				Index:  i + 1,
				Total:  total,
				Status: ProgressReading,
			})
		}
		hadErr := false
		for _, h := range homes {
			found := a.Discover(h)
			for _, root := range found {
				if _, ok := seenPath[root.Path]; ok {
					continue
				}
				seenPath[root.Path] = struct{}{}
				roots = append(roots, root)
				err := a.Parse(root, func(e event.UsageEvent) {
					events = append(events, e)
				}, func(te event.TurnEvent) {
					turns = append(turns, te)
				})
				if err != nil {
					hadErr = true
					errs = append(errs, a.ID()+": "+err.Error())
				}
			}
		}
		if report != nil {
			st := ProgressDone
			if hadErr {
				st = ProgressError
			}
			report(Progress{
				Source: a.ID(),
				Label:  readingLabel(a.ID()),
				Index:  i + 1,
				Total:  total,
				Status: st,
			})
		}
	}
	if errs == nil {
		errs = []string{}
	}
	sum := metric.Aggregate(events, turns)
	seen := map[string]struct{}{}
	for _, e := range events {
		seen[e.Source] = struct{}{}
	}
	for _, root := range roots {
		if _, ok := seen[root.ID]; ok {
			continue
		}
		already := false
		for _, s := range sum.BySource {
			if s.ID == root.ID {
				already = true
				break
			}
		}
		if already {
			continue
		}
		q := event.QualityAbsent
		for _, msg := range errs {
			if strings.HasPrefix(msg, root.ID+": ") {
				q = event.QualityDegraded
				break
			}
		}
		sum.BySource = append(sum.BySource, metric.Slice{
			ID:      root.ID,
			Label:   metric.SourceLabel(root.ID),
			Quality: q,
		})
	}
	return Result{
		Summary:   sum,
		Roots:     roots,
		Errors:    errs,
		Events:    events,
		Turns:     turns,
		ScannedAt: time.Now(),
	}
}

type sourceVendorView struct {
	Source       string `json:"source"`
	Vendor       string `json:"vendor"`
	SourceLabel  string `json:"source_label"`
	VendorLabel  string `json:"vendor_label"`
	Miss         int64  `json:"miss"`
	CacheRead    int64  `json:"cache_read"`
	CacheCreate  int64  `json:"cache_create"`
	Output       int64  `json:"output"`
	Total        int64  `json:"total"`
	MissM        string `json:"miss_m"`
	CacheReadM   string `json:"cache_read_m"`
	CacheCreateM string `json:"cache_create_m"`
	OutputM      string `json:"output_m"`
	TotalM       string `json:"total_m"`
	Requests     int64  `json:"requests"`
}

type drillJSON struct {
	All      metric.DrillTablesView            `json:"all"`
	BySource map[string]metric.DrillTablesView `json:"by_source"`
	ByVendor map[string]metric.DrillTablesView `json:"by_vendor"`
}

type summaryJSON struct {
	ScannedAt      string             `json:"scanned_at,omitempty"`
	All            metric.SliceView   `json:"all"`
	BySource       []metric.SliceView `json:"by_source"`
	ByVendor       []metric.SliceView `json:"by_vendor"`
	BySourceVendor []sourceVendorView `json:"by_source_vendor"`
	Calendar       metric.Calendar    `json:"calendar"`
	Drill          drillJSON          `json:"drill"`
	Errors         []string           `json:"errors"`
}

func viewWithError(s metric.Slice, errs []string) metric.SliceView {
	v := metric.View(s)
	prefix := s.ID + ": "
	for _, msg := range errs {
		if strings.HasPrefix(msg, prefix) {
			v.Error = strings.TrimPrefix(msg, prefix)
			break
		}
	}
	return v
}

func buildSummaryJSON(r Result) summaryJSON {
	out := summaryJSON{
		All:      metric.View(r.Summary.All),
		Calendar: r.Summary.Calendar,
		Drill: drillJSON{
			All:      metric.ViewDrill(r.Summary.DrillAll),
			BySource: map[string]metric.DrillTablesView{},
			ByVendor: map[string]metric.DrillTablesView{},
		},
		Errors: r.Errors,
	}
	if !r.ScannedAt.IsZero() {
		out.ScannedAt = r.ScannedAt.Format(time.RFC3339)
	}
	if out.Errors == nil {
		out.Errors = []string{}
	}
	for _, s := range r.Summary.BySource {
		out.BySource = append(out.BySource, viewWithError(s, r.Errors))
	}
	for _, s := range r.Summary.ByVendor {
		out.ByVendor = append(out.ByVendor, viewWithError(s, r.Errors))
	}
	if out.BySource == nil {
		out.BySource = []metric.SliceView{}
	}
	if out.ByVendor == nil {
		out.ByVendor = []metric.SliceView{}
	}
	for _, s := range r.Summary.BySourceVendor {
		out.BySourceVendor = append(out.BySourceVendor, sourceVendorView{
			Source:       s.Source,
			Vendor:       s.Vendor,
			SourceLabel:  s.SourceLabel,
			VendorLabel:  s.VendorLabel,
			Miss:         s.Miss,
			CacheRead:    s.CacheRead,
			CacheCreate:  s.CacheCreate,
			Output:       s.Output,
			Total:        s.Total(),
			MissM:        metric.FormatM(s.Miss),
			CacheReadM:   metric.FormatM(s.CacheRead),
			CacheCreateM: metric.FormatM(s.CacheCreate),
			OutputM:      metric.FormatM(s.Output),
			TotalM:       metric.FormatM(s.Total()),
			Requests:     s.Requests,
		})
	}
	if out.BySourceVendor == nil {
		out.BySourceVendor = []sourceVendorView{}
	}
	for id, pack := range r.Summary.DrillBySource {
		out.Drill.BySource[id] = metric.ViewDrill(pack)
	}
	for id, pack := range r.Summary.DrillByVendor {
		out.Drill.ByVendor[id] = metric.ViewDrill(pack)
	}
	return out
}

func MarshalSummary(r Result) ([]byte, error) {
	return json.Marshal(buildSummaryJSON(r))
}

func EncodeSummary(w io.Writer, r Result) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(buildSummaryJSON(r))
}
