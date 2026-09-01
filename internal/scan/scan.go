package scan

import (
	"encoding/json"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/rainhuang0220/whereToken/internal/adapter"
	"github.com/rainhuang0220/whereToken/internal/adapter/claude"
	"github.com/rainhuang0220/whereToken/internal/adapter/cline"
	"github.com/rainhuang0220/whereToken/internal/adapter/codex"
	"github.com/rainhuang0220/whereToken/internal/adapter/cursor"
	"github.com/rainhuang0220/whereToken/internal/adapter/gemini"
	"github.com/rainhuang0220/whereToken/internal/adapter/grok"
	"github.com/rainhuang0220/whereToken/internal/adapter/kilo"
	"github.com/rainhuang0220/whereToken/internal/adapter/kimi"
	"github.com/rainhuang0220/whereToken/internal/adapter/minimax"
	"github.com/rainhuang0220/whereToken/internal/adapter/openclaw"
	"github.com/rainhuang0220/whereToken/internal/adapter/opencode"
	"github.com/rainhuang0220/whereToken/internal/adapter/qwen"
	"github.com/rainhuang0220/whereToken/internal/adapter/roo"
	"github.com/rainhuang0220/whereToken/internal/adapter/testhome"
	"github.com/rainhuang0220/whereToken/internal/adapter/trae"
	"github.com/rainhuang0220/whereToken/internal/adapter/zcode"
	"github.com/rainhuang0220/whereToken/internal/community"
	"github.com/rainhuang0220/whereToken/internal/event"
	"github.com/rainhuang0220/whereToken/internal/index"
	"github.com/rainhuang0220/whereToken/internal/insight"
	"github.com/rainhuang0220/whereToken/internal/metric"
	"github.com/rainhuang0220/whereToken/internal/report"
)

type Result struct {
	Summary   metric.Summary
	Roots     []adapter.SourceRoot
	Errors    []string
	Events    []event.UsageEvent
	Turns     []event.TurnEvent
	ScannedAt time.Time
	Offline   bool
	Scanning  bool
	Deltas    []index.Delta
	Compare   *metric.Compare
	Community *community.View
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

func (p Progress) DisplayLabel(ascii bool) string {
	if !ascii {
		return p.Label
	}
	return strings.ReplaceAll(p.Label, "…", "...")
}

func AllAdapters() []adapter.Adapter {
	return Adapters(false)
}

func Adapters(offline bool) []adapter.Adapter {
	return []adapter.Adapter{
		claude.Adapter{},
		kimi.Adapter{},
		grok.Adapter{},
		minimax.Adapter{},
		openclaw.Adapter{},
		opencode.Adapter{},
		codex.Adapter{},
		cursor.Adapter{Offline: offline},
		trae.Adapter{Offline: offline},
		gemini.Adapter{},
		qwen.Adapter{},
		cline.Adapter{},
		roo.Adapter{},
		kilo.Adapter{},
		zcode.Adapter{},
	}
}

func CloudSkipped(adapters []adapter.Adapter) bool {
	for _, a := range adapters {
		switch v := a.(type) {
		case cursor.Adapter:
			if v.Offline {
				return true
			}
		case trae.Adapter:
			if v.Offline {
				return true
			}
		}
	}
	return false
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
	index.ResetDeltas()
	if os.Getenv("WHERETOKEN_NO_INDEX") != "1" {
		if st, err := index.Open(index.PathFor(home)); err == nil {
			defer st.Close()
			defer index.Use(st)()
		}
	}
	var events []event.UsageEvent
	var turns []event.TurnEvent
	var roots []adapter.SourceRoot
	var errs []string
	var seenInfos []os.FileInfo
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
				if st, err := os.Stat(root.Path); err == nil {
					dup := false
					for _, prev := range seenInfos {
						if os.SameFile(prev, st) {
							dup = true
							break
						}
					}
					if dup {
						continue
					}
					seenInfos = append(seenInfos, st)
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
	fillMissingSources(&sum, roots, errs, events)
	return Result{
		Summary:   sum,
		Roots:     roots,
		Errors:    errs,
		Events:    events,
		Turns:     turns,
		ScannedAt: time.Now(),
		Deltas:    index.Deltas(),
	}
}

func fillMissingSources(sum *metric.Summary, roots []adapter.SourceRoot, errs []string, events []event.UsageEvent) {
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
}

// ApplyWindow returns a copy of r whose events, turns, and summary are limited
// to w. The original scan (and index) is not re-run.
func ApplyWindow(r Result, w metric.Window, loc *time.Location) Result {
	if w.IsAll() {
		return r
	}
	var evs []event.UsageEvent
	for _, e := range r.Events {
		if w.Contains(e.Timestamp, loc) {
			evs = append(evs, e)
		}
	}
	var turns []event.TurnEvent
	for _, t := range r.Turns {
		if w.Contains(t.Timestamp, loc) {
			turns = append(turns, t)
		}
	}
	out := r
	if r.Community != nil {
		v := *r.Community
		out.Community = &v
	}
	out.Events = evs
	out.Turns = turns
	sum := metric.Aggregate(evs, turns)
	// Presence is all-time: a source that has history must not become
	// "absent" just because this window is empty.
	fillMissingSources(&sum, r.Roots, r.Errors, r.Events)
	out.Summary = sum
	return out
}

func CompareWindows(full Result, w metric.Window, loc *time.Location) *metric.Compare {
	prevWin := w.Previous()
	if !w.Bounded() || !prevWin.Bounded() {
		return nil
	}
	cur := ApplyWindow(full, w, loc)
	prev := ApplyWindow(full, prevWin, loc)
	c := metric.NewCompare(cur.Summary, prev.Summary)
	return &c
}

func FormatDeltas(ds []index.Delta) string {
	if len(ds) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("Scanning usage data...\n\n")
	for _, d := range ds {
		extra := ""
		if d.Mode != index.ModeUnchanged && d.Added > 0 {
			extra = "+" + strconv.Itoa(d.Added)
		}
		b.WriteString(pad(metric.SourceLabel(d.Source), 16))
		b.WriteString(pad(d.Mode, 14))
		b.WriteString(extra)
		b.WriteByte('\n')
	}
	return b.String()
}

func pad(s string, n int) string {
	if len(s) >= n {
		return s + " "
	}
	return s + strings.Repeat(" ", n-len(s))
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
	CostStatus   string `json:"cost_status,omitempty"`
	CostUSD      string `json:"cost_usd,omitempty"`
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
	Offline        bool               `json:"offline,omitempty"`
	Scanning       bool               `json:"scanning,omitempty"`
	Scan           []scanDeltaJSON    `json:"scan,omitempty"`
	Why            []whyJSON          `json:"why,omitempty"`
	Compare        *metric.Compare    `json:"compare,omitempty"`
	Insights       []insight.Line     `json:"insights,omitempty"`
	Evaluation     insight.Evaluation `json:"evaluation"`
	Community      *community.View    `json:"community,omitempty"`
}

type scanDeltaJSON struct {
	Source string `json:"source"`
	Label  string `json:"label"`
	Mode   string `json:"mode"`
	Added  int    `json:"added"`
}

type whyJSON struct {
	Source      string `json:"source"`
	Label       string `json:"label"`
	Records     int64  `json:"records"`
	Miss        int64  `json:"miss"`
	CacheRead   int64  `json:"cache_read"`
	CacheCreate int64  `json:"cache_create"`
	Output      int64  `json:"output"`
	Total       int64  `json:"total"`
	Quality     string `json:"quality"`
	Derivation  string `json:"derivation"`
}

func viewWithError(s metric.Slice, errs []string) metric.SliceView {
	v := metric.View(s)
	prefix := s.ID + ": "
	for _, msg := range errs {
		msg = report.Redact(msg)
		if strings.HasPrefix(msg, prefix) {
			v.Error = strings.TrimPrefix(msg, prefix)
			break
		}
	}
	return v
}

func redactErrors(errs []string) []string {
	if errs == nil {
		return []string{}
	}
	out := make([]string, len(errs))
	for i, e := range errs {
		out[i] = report.Redact(e)
	}
	return out
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
		Errors:     redactErrors(r.Errors),
		Offline:    r.Offline,
		Scanning:   r.Scanning,
		Compare:    r.Compare,
		Insights:   insight.Lines(r.Summary),
		Evaluation: insight.Evaluate(r.Summary),
		Community:  r.Community,
	}
	if r.Community != nil {
		v := *r.Community
		v.Today = community.SanitizeStanding(v.Today)
		v.All = community.SanitizeStanding(v.All)
		out.Community = &v
		// All-time observatory may mention 累计 rank (uploaded days), never
		// today's podium. A 7d/today window has different totals; skip rank.
		if r.Compare == nil {
			out.Insights = insight.AppendStanding(out.Insights, v.All.Status, v.All.Display, v.All.Rank)
		}
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
		row := sourceVendorView{
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
			CostStatus:   s.CostStatus,
			CostUSD:      metric.FormatCostUSD(s.CostStatus, s.CostMicro),
		}
		out.BySourceVendor = append(out.BySourceVendor, row)
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
	for _, d := range r.Deltas {
		out.Scan = append(out.Scan, scanDeltaJSON{
			Source: d.Source,
			Label:  metric.SourceLabel(d.Source),
			Mode:   d.Mode,
			Added:  d.Added,
		})
	}
	for _, s := range r.Summary.BySource {
		out.Why = append(out.Why, whyJSON{
			Source:      s.ID,
			Label:       s.Label,
			Records:     s.Records,
			Miss:        s.Miss,
			CacheRead:   s.CacheRead,
			CacheCreate: s.CacheCreate,
			Output:      s.Output,
			Total:       s.Total(),
			Quality:     string(s.Quality),
			Derivation:  s.Derivation,
		})
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
