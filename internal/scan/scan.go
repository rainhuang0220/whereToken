package scan

import (
	"encoding/json"
	"io"

	"github.com/rainhuang0220/whereToken/internal/adapter"
	"github.com/rainhuang0220/whereToken/internal/adapter/claude"
	"github.com/rainhuang0220/whereToken/internal/adapter/codex"
	"github.com/rainhuang0220/whereToken/internal/adapter/kimi"
	"github.com/rainhuang0220/whereToken/internal/adapter/opencode"
	"github.com/rainhuang0220/whereToken/internal/event"
	"github.com/rainhuang0220/whereToken/internal/metric"
)

type Result struct {
	Summary metric.Summary
	Roots   []adapter.SourceRoot
	Errors  []string
}

func AllAdapters() []adapter.Adapter {
	return []adapter.Adapter{
		claude.Adapter{},
		kimi.Adapter{},
		opencode.Adapter{},
		codex.Adapter{},
	}
}

func Run(home adapter.Home, adapters []adapter.Adapter) Result {
	var events []event.UsageEvent
	var turns []event.TurnEvent
	var roots []adapter.SourceRoot
	var errs []string
	for _, a := range adapters {
		found := a.Discover(home)
		roots = append(roots, found...)
		for _, root := range found {
			err := a.Parse(root, func(e event.UsageEvent) {
				events = append(events, e)
			}, func(te event.TurnEvent) {
				turns = append(turns, te)
			})
			if err != nil {
				errs = append(errs, a.ID()+": "+err.Error())
			}
		}
	}
	if errs == nil {
		errs = []string{}
	}
	return Result{
		Summary: metric.Aggregate(events, turns),
		Roots:   roots,
		Errors:  errs,
	}
}

type sourceVendorView struct {
	Source        string `json:"source"`
	Vendor        string `json:"vendor"`
	SourceLabel   string `json:"source_label"`
	VendorLabel   string `json:"vendor_label"`
	Miss          int64  `json:"miss"`
	CacheRead     int64  `json:"cache_read"`
	CacheCreate   int64  `json:"cache_create"`
	Output        int64  `json:"output"`
	Total         int64  `json:"total"`
	MissM         string `json:"miss_m"`
	CacheReadM    string `json:"cache_read_m"`
	CacheCreateM  string `json:"cache_create_m"`
	OutputM       string `json:"output_m"`
	TotalM        string `json:"total_m"`
	Requests      int64  `json:"requests"`
}

type summaryJSON struct {
	All            metric.SliceView              `json:"all"`
	BySource       []metric.SliceView            `json:"by_source"`
	ByVendor       []metric.SliceView            `json:"by_vendor"`
	BySourceVendor []sourceVendorView            `json:"by_source_vendor"`
	Calendar       metric.Calendar               `json:"calendar"`
	Errors         []string                      `json:"errors"`
}

func EncodeSummary(w io.Writer, r Result) error {
	out := summaryJSON{
		All:      metric.View(r.Summary.All),
		Calendar: r.Summary.Calendar,
		Errors:   r.Errors,
	}
	if out.Errors == nil {
		out.Errors = []string{}
	}
	for _, s := range r.Summary.BySource {
		out.BySource = append(out.BySource, metric.View(s))
	}
	for _, s := range r.Summary.ByVendor {
		out.ByVendor = append(out.ByVendor, metric.View(s))
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
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}
