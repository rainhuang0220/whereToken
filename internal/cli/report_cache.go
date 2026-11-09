package cli

import (
	"os"

	"github.com/rainhuang0220/whereToken/internal/adapter"
	"github.com/rainhuang0220/whereToken/internal/index"
	"github.com/rainhuang0220/whereToken/internal/ledger"
	"github.com/rainhuang0220/whereToken/internal/metric"
	"github.com/rainhuang0220/whereToken/internal/publicprofile"
	"github.com/rainhuang0220/whereToken/internal/scan"
)

// cachedReport is a read-only, non-live report path for owners who already
// enabled the macOS background refresh. The watcher remains the only automatic
// refresher; interactive reads should not reparse multi-gigabyte Cursor DBs.
// Missing/corrupt cache or an incomplete ledger fails closed to the normal scan.
func (a *App) cachedReport(home adapter.Home) (scan.Result, bool) {
	if a.Scan != nil || a.GOOS != "darwin" || os.Getenv("WHERETOKEN_NO_INDEX") == "1" {
		return scan.Result{}, false
	}
	on, err := publicprofile.LoadRefreshSwitch(publicprofile.RefreshConfigPath(home))
	if err != nil || !on {
		return scan.Result{}, false
	}
	st, err := index.OpenReadOnly(index.PathFor(home))
	if err != nil {
		return scan.Result{}, false
	}
	defer st.Close()
	events, turns, gaps, err := st.ExportCached()
	if err != nil || gaps != 0 || len(events)+len(turns) == 0 {
		return scan.Result{}, false
	}
	if os.Getenv("WHERETOKEN_NO_LEDGER") != "1" {
		path, err := ledger.PathFor(home)
		if err != nil {
			return scan.Result{}, false
		}
		lg, err := ledger.OpenReadOnly(path)
		if err != nil {
			return scan.Result{}, false
		}
		defer lg.Close()
		extra, err := lg.Unseen(events)
		if err != nil {
			return scan.Result{}, false
		}
		events = append(events, extra...)
	}
	return scan.Result{
		Events:  events,
		Turns:   turns,
		Summary: metric.Aggregate(events, turns),
		Offline: true,
	}, true
}
