package hosted

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/rainhuang0220/whereToken/internal/event"
	"github.com/rainhuang0220/whereToken/internal/metric"
	"github.com/rainhuang0220/whereToken/internal/profile"
	"github.com/rainhuang0220/whereToken/internal/syncagg"
)

func (s *server) getDashboard(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	user, _, err := s.currentUser(r)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	has, err := s.opts.Store.HasUsage(r.Context(), user.ID)
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	devs, _ := s.opts.Store.ListDevices(r.Context(), user.ID)
	tzName := s.opts.Store.UserTimezone(r.Context(), user.ID)
	loc, err := time.LoadLocation(tzName)
	if err != nil {
		loc = time.UTC
	}
	now := s.opts.Now().In(loc)
	win, err := metric.ParseSinceQuery(r.URL.Query().Get("since"), now, loc)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	from, to := "0001-01-01", now.Format("2006-01-02")
	if !win.IsAll() {
		from = win.From.In(loc).Format("2006-01-02")
		to = win.To.Add(-time.Nanosecond).In(loc).Format("2006-01-02")
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if !has {
		_ = json.NewEncoder(w).Encode(emptyHostedSummary(user, devs, now))
		return
	}
	rows, err := s.opts.Store.ListUsage(r.Context(), user.ID, from, to)
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	sum := aggregateRows(rows, now, loc)
	portrait := profile.Evaluate(sum, user.ProfileSeed)
	payload := map[string]any{
		"scanned_at": now.Format(time.RFC3339),
		"all":        metric.View(sum.All),
		"by_source":  viewSlices(sum.BySource),
		"by_vendor":  viewSlices(sum.ByVendor),
		"by_model":   viewModels(sum.ByModel),
		"calendar":   sum.Calendar,
		"errors":     []string{},
		"portrait":   portrait,
		"hosted":     hostedMeta(devs, false),
	}
	_ = json.NewEncoder(w).Encode(payload)
}

func emptyHostedSummary(user User, devs []Device, now time.Time) map[string]any {
	sum := metric.Aggregate(nil, nil)
	return map[string]any{
		"scanned_at": now.Format(time.RFC3339),
		"all":        metric.View(sum.All),
		"by_source":  []metric.SliceView{},
		"by_vendor":  []metric.SliceView{},
		"calendar":   sum.Calendar,
		"errors":     []string{},
		"portrait":   profile.Evaluate(sum, user.ProfileSeed),
		"hosted":     hostedMeta(devs, true),
	}
}

func hostedMeta(devs []Device, never bool) map[string]any {
	list := []map[string]any{}
	var last time.Time
	for _, d := range devs {
		if d.Revoked {
			continue
		}
		if d.LastSyncAt.After(last) {
			last = d.LastSyncAt
		}
		list = append(list, map[string]any{
			"id":        d.PublicID,
			"label":     d.Label,
			"os":        d.OS,
			"arch":      d.Arch,
			"last_seen": d.LastSeenAt.UTC().Format(time.RFC3339),
			"last_sync": d.LastSyncAt.UTC().Format(time.RFC3339),
			"revoked":   d.Revoked,
		})
	}
	out := map[string]any{"never_synced": never, "devices": list}
	if !last.IsZero() {
		out["last_sync_at"] = last.UTC().Format(time.RFC3339)
	}
	return out
}

func aggregateRows(rows []syncagg.DailyModel, now time.Time, loc *time.Location) metric.Summary {
	var evs []event.UsageEvent
	var turns []event.TurnEvent
	seenTurns := map[string]struct{}{}
	for _, row := range rows {
		day, err := time.ParseInLocation("2006-01-02", row.Date, loc)
		if err != nil {
			continue
		}
		ts := day.Add(12 * time.Hour)
		evs = append(evs, event.UsageEvent{
			Source:      row.Tool,
			Vendor:      row.Vendor,
			Model:       row.Model,
			Timestamp:   ts,
			Miss:        row.Miss,
			CacheRead:   row.CacheRead,
			CacheCreate: row.CacheCreate,
			Output:      row.Output,
			Quality:     event.Quality(row.Quality),
			Derivation:  row.Derivation,
			SkipRequest: true,
		})
		tk := row.Tool + "\x00" + row.Date + "\x00" + string(row.SourceScope) + "\x00" + row.SourceKeyHash
		if _, ok := seenTurns[tk]; !ok {
			seenTurns[tk] = struct{}{}
			for i := int64(0); i < row.UserTurns; i++ {
				turns = append(turns, event.TurnEvent{Source: row.Tool, Timestamp: ts})
			}
		}
	}
	sum := metric.AggregateAt(evs, turns, now, loc)
	sum.Calendar = metric.BuildCalendar(evs, loc, now)
	var req int64
	bySrc := map[string]int64{}
	for _, row := range rows {
		req += row.Requests
		bySrc[row.Tool] += row.Requests
	}
	sum.All.Requests = req
	for i := range sum.BySource {
		sum.BySource[i].Requests = bySrc[sum.BySource[i].ID]
	}
	return sum
}

func viewSlices(in []metric.Slice) []metric.SliceView {
	out := make([]metric.SliceView, 0, len(in))
	for _, s := range in {
		out = append(out, metric.View(s))
	}
	return out
}

func viewModels(in []metric.ModelSlice) []metric.ModelView {
	out := make([]metric.ModelView, 0, len(in))
	for _, s := range in {
		out = append(out, metric.ViewModel(s))
	}
	return out
}
