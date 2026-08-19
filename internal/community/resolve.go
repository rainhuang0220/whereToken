package community

import (
	"context"
	"os"
	"time"

	"github.com/rainhuang0220/whereToken/internal/adapter"
	"github.com/rainhuang0220/whereToken/internal/event"
)

// Request is how the CLI and dashboard ask for a Community Rank view.
// Missing URL, opt-out, or offline never fail local analytics.
type Request struct {
	Home    adapter.Home
	Getenv  func(string) string
	Offline bool
	OptOut  bool
	Version string
	Now     time.Time
	Loc     *time.Location
}

// Resolve builds the view the KPI column displays. It does not create
// community.json unless a service URL is configured and the user has not
// opted out. No URL means no upload.
func Resolve(req Request, events []event.UsageEvent) View {
	if req.Loc == nil {
		req.Loc = time.Local
	}
	if req.Now.IsZero() {
		req.Now = time.Now()
	}
	if req.OptOut || EnvDisabled(req.Getenv) {
		v := EmptyView(StatusOptedOut, DisclaimerEN)
		v.Enabled = false
		return v
	}
	if req.Offline {
		return EmptyView(StatusOffline, DisclaimerEN)
	}
	url := EnvURL(req.Getenv)
	path := ConfigPath(req.Home)
	f, err := Load(path)
	if err != nil && !os.IsNotExist(err) {
		return EmptyView(StatusUnavailable, DisclaimerEN)
	}
	if f != nil && !f.Enabled {
		v := EmptyView(StatusOptedOut, DisclaimerEN)
		v.Enabled = false
		return v
	}
	if url == "" {
		return EmptyView(StatusServiceUnconfigured, "Community Rank service is not configured.")
	}
	if f == nil {
		joined := req.Now.In(req.Loc).Format("2006-01-02")
		f, err = LoadOrCreate(path, joined)
		if err != nil {
			return EmptyView(StatusUnavailable, DisclaimerEN)
		}
	}
	c := &Client{
		BaseURL: url,
		File:    f,
		Path:    path,
		Version: req.Version,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	return c.Sync(ctx, events, req.Now, req.Loc)
}
