// Command gendemo fabricates a synthetic multi-source ledger and writes the
// static JSON payloads the public demo site serves. Every row is invented:
// no real user, path, or provider data is read. Run from the repo root:
//
//	go run ./scripts/gendemo
//
// Output lands in web/public/sample/{all,today,7d,30d}.json and ships with
// the VITE_DEMO=1 build so the dashboard works without any backend.
package main

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"time"

	"github.com/rainhuang0220/whereToken/internal/adapter"
	"github.com/rainhuang0220/whereToken/internal/community"
	"github.com/rainhuang0220/whereToken/internal/event"
	"github.com/rainhuang0220/whereToken/internal/metric"
	"github.com/rainhuang0220/whereToken/internal/scan"
	"github.com/rainhuang0220/whereToken/internal/vendor"
)

type profile struct {
	source     string
	models     []string
	provider   string
	workspaces []string
	// tokens per request, rough ranges
	missLo, missHi int64
	// probability the tool is used on a weekday / weekend day
	weekday, weekend float64
	reasoning        bool
}

var profiles = []profile{
	{"claude", []string{"claude-opus-4.6", "claude-sonnet-4.6"}, "", []string{"demo/loom", "demo/kiln"}, 60_000, 420_000, 0.9, 0.5, false},
	{"kimi", []string{"kimi-k3"}, "", []string{"demo/loom"}, 40_000, 260_000, 0.7, 0.3, false},
	{"codex", []string{"gpt-5.3-codex"}, "", []string{"demo/ember"}, 30_000, 200_000, 0.65, 0.35, true},
	{"cursor", []string{"claude-sonnet-4.6"}, "cursor", []string{"demo/kiln"}, 20_000, 150_000, 0.6, 0.3, false},
	{"grok", []string{"grok-4.6-build"}, "", []string{"demo/ember"}, 15_000, 120_000, 0.4, 0.2, true},
	{"gemini", []string{"gemini-3.5-flash", "gemini-2.5-pro"}, "", []string{"demo/loom"}, 20_000, 180_000, 0.45, 0.25, false},
	{"zcode", []string{"glm-5.2"}, "", []string{"demo/kiln", "demo/ember"}, 25_000, 160_000, 0.5, 0.3, false},
	{"opencode", []string{"deepseek-v4-flash"}, "deepseek", []string{"demo/loom"}, 18_000, 140_000, 0.35, 0.15, false},
}

func fabricate(now time.Time, loc *time.Location) ([]event.UsageEvent, []event.TurnEvent) {
	rng := rand.New(rand.NewSource(42))
	var evs []event.UsageEvent
	var turns []event.TurnEvent
	// ~45 days of plausible activity ending today, so every period tab
	// (today / 7d / 30d / all) has something to show.
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
	for d := 45; d >= 0; d-- {
		day := today.AddDate(0, 0, -d)
		weekend := day.Weekday() == time.Saturday || day.Weekday() == time.Sunday
		for _, p := range profiles {
			prob := p.weekday
			if weekend {
				prob = p.weekend
			}
			if rng.Float64() > prob {
				continue
			}
			sess := fmt.Sprintf("demo-%s-%d", p.source, rng.Intn(4)+1)
			ws := p.workspaces[rng.Intn(len(p.workspaces))]
			requests := 2 + rng.Intn(7)
			// Today gets a morning burst so the 当日 KPI is alive.
			baseHour := 9 + rng.Intn(10)
			if d == 0 {
				baseHour = 8
				requests = 3 + rng.Intn(3)
			}
			for i := 0; i < requests; i++ {
				miss := p.missLo + rng.Int63n(p.missHi-p.missLo)
				cacheRead := miss * int64(rng.Intn(300)) / 100
				cacheCreate := miss * int64(rng.Intn(20)) / 100
				output := miss*int64(8+rng.Intn(20))/100 + int64(rng.Intn(4000))
				var reasoning int64
				if p.reasoning {
					reasoning = output * int64(rng.Intn(40)) / 100
				}
				hour := baseHour + i/2
				if d == 0 {
					hour = baseHour + i
				}
				ts := time.Date(day.Year(), day.Month(), day.Day(), hour, rng.Intn(60), 0, 0, loc)
				if ts.After(now) {
					ts = now.Add(-time.Duration(rng.Intn(50)+1) * time.Minute)
				}
				model := p.models[rng.Intn(len(p.models))]
				evs = append(evs, event.UsageEvent{
					Source:      p.source,
					Vendor:      vendor.Lookup(model, p.provider),
					Provider:    p.provider,
					RequestID:   fmt.Sprintf("demo-%s-%d-%d", p.source, d, i),
					SessionID:   sess,
					Workspace:   ws,
					Model:       model,
					Timestamp:   ts,
					Miss:        miss,
					CacheRead:   cacheRead,
					CacheCreate: cacheCreate,
					Output:      output,
					Reasoning:   reasoning,
					Quality:     event.QualityAuthoritative,
					Derivation:  event.DeriveDerived,
				})
			}
			for tn := 0; tn < 1+rng.Intn(4); tn++ {
				ts := time.Date(day.Year(), day.Month(), day.Day(), baseHour, rng.Intn(60), 0, 0, loc)
				if ts.After(now) {
					ts = now.Add(-time.Hour)
				}
				turns = append(turns, event.TurnEvent{Source: p.source, SessionID: sess, Timestamp: ts})
			}
		}
	}
	return evs, turns
}

func write(base scan.Result, period string, now time.Time, loc *time.Location, outDir string) error {
	cur := base
	if period != "all" {
		win, err := metric.ParseSinceQuery(period, now, loc)
		if err != nil {
			return err
		}
		cur = scan.ApplyWindow(base, win, loc)
		cur.Compare = scan.CompareWindows(base, win, loc)
	}
	raw, err := scan.MarshalSummary(cur)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(outDir, period+".json"), raw, 0o644)
}

func main() {
	outDir := filepath.Join("web", "public", "sample")
	if len(os.Args) > 1 {
		outDir = os.Args[1]
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	loc := time.Local
	now := time.Now()
	evs, turns := fabricate(now, loc)
	unconfigured := community.EmptyView(community.StatusServiceUnconfigured, "")
	base := scan.Result{
		Summary:   metric.AggregateAt(evs, turns, now, loc),
		Errors:    []string{},
		ScannedAt: now,
		Events:    evs,
		Turns:     turns,
		Community: &unconfigured,
	}
	for _, p := range profiles {
		base.Roots = append(base.Roots, adapter.SourceRoot{ID: p.source, Path: "demo"})
	}
	for _, period := range []string{"all", "today", "7d", "30d"} {
		if err := write(base, period, now, loc, outDir); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
	fmt.Printf("demo payloads: %d events, %d turns → %s\n", len(evs), len(turns), outDir)
}
