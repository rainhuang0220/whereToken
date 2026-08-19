package cli

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/rainhuang0220/whereToken/internal/adapter"
	"github.com/rainhuang0220/whereToken/internal/community"
	"github.com/rainhuang0220/whereToken/internal/event"
	"github.com/rainhuang0220/whereToken/internal/report"
)

func (a *App) attachCommunity(snap *report.Snapshot, flags Flags, home adapter.Home, events []event.UsageEvent) {
	snap.RankPeriod = flags.RankPeriod
	snap.Community = community.Resolve(community.Request{
		Home:    home,
		Getenv:  a.LookupEnv,
		Offline: flags.Offline || a.wantOffline(flags),
		OptOut:  flags.NoCommunity,
		Version: a.Version,
		Now:     a.Now(),
		Loc:     a.Loc,
	}, events)
}

func (a *App) runCommunity(flags Flags, home adapter.Home) int {
	switch flags.CommunityAction {
	case "", "status":
		return a.communityStatus(flags, home)
	case "on":
		return a.communitySet(home, true)
	case "off":
		return a.communitySet(home, false)
	case "serve":
		return a.runCommunityServe(flags)
	default:
		fmt.Fprintf(a.Stderr, "unknown community action %q\ntry `wheretoken --help`\n", flags.CommunityAction)
		return ExitUsage
	}
}

func (a *App) communityStatus(flags Flags, home adapter.Home) int {
	path := community.ConfigPath(home)
	f, err := community.Load(path)
	if err != nil && !os.IsNotExist(err) {
		fmt.Fprintln(a.Stderr, err.Error())
		return ExitFail
	}
	on := "off"
	participant := "—"
	joined := "—"
	if f != nil {
		if f.Enabled {
			on = "on"
		}
		participant = f.ParticipantID
		if f.JoinedAt != "" {
			joined = f.JoinedAt
		}
	}
	url := community.EnvURL(a.LookupEnv)
	if url == "" {
		url = "(not configured — set WHERETOKEN_COMMUNITY_URL)"
	}
	if community.EnvDisabled(a.LookupEnv) || flags.NoCommunity {
		on = "off"
	}
	fmt.Fprintf(a.Stdout, "community rank: %s\n", on)
	fmt.Fprintf(a.Stdout, "participant: %s\n", participant)
	fmt.Fprintf(a.Stdout, "joined: %s\n", joined)
	fmt.Fprintf(a.Stdout, "file: %s\n", path)
	fmt.Fprintf(a.Stdout, "service: %s\n", url)
	fmt.Fprintf(a.Stdout, "%s\n", community.DisclaimerEN)
	return ExitOK
}

func (a *App) communitySet(home adapter.Home, on bool) int {
	path := community.ConfigPath(home)
	joined := a.Now().In(a.Loc).Format("2006-01-02")
	f, err := community.LoadOrCreate(path, joined)
	if err != nil {
		fmt.Fprintln(a.Stderr, err.Error())
		return ExitFail
	}
	if !on {
		c := &community.Client{
			BaseURL: community.EnvURL(a.LookupEnv),
			File:    f,
			Path:    path,
			Version: a.Version,
		}
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		_ = c.Leave(ctx)
		cancel()
	}
	if err := f.SetEnabled(path, on); err != nil {
		fmt.Fprintln(a.Stderr, err.Error())
		return ExitFail
	}
	if on {
		fmt.Fprintln(a.Stdout, "community rank on (anonymous daily totals only)")
	} else {
		fmt.Fprintln(a.Stdout, "community rank off (no upload)")
	}
	return ExitOK
}

func (a *App) runCommunityServe(flags Flags) int {
	port := flags.Port
	if port == 8787 {
		port = 8798
	}
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	if a.Serve != nil {
		fmt.Fprintf(a.Stderr, "community rank http://%s\n", addr)
		return ExitOK
	}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		fmt.Fprintln(a.Stderr, err.Error())
		return ExitFail
	}
	fmt.Fprintf(a.Stderr, "community rank http://%s\nin-process fake/host for tests and self-host. No public whereToken rank cluster is deployed.\n", addr)
	h := community.NewHandler(community.NewStore(community.DefaultMinParticipants))
	srv := &http.Server{Addr: addr, Handler: h.Mux(), ReadHeaderTimeout: 5 * time.Second}
	if err := srv.Serve(ln); err != nil && err != http.ErrServerClosed {
		fmt.Fprintln(a.Stderr, err.Error())
		return ExitFail
	}
	return ExitOK
}
