package cli

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"runtime"
	"time"

	"github.com/rainhuang0220/whereToken/internal/adapter"
	"github.com/rainhuang0220/whereToken/internal/adapter/testhome"
	"github.com/rainhuang0220/whereToken/internal/httpapi"
	"github.com/rainhuang0220/whereToken/internal/report"
	"github.com/rainhuang0220/whereToken/internal/scan"
	"github.com/rainhuang0220/whereToken/internal/table"
)

type App struct {
	Args      []string
	Stdout    io.Writer
	Stderr    io.Writer
	Version   string
	Now       func() time.Time
	Loc       *time.Location
	LookupEnv func(string) string
	Scan      func(adapter.Home) scan.Result
	Home      adapter.Home
	Serve     func(addr string, home adapter.Home) error
	GOOS      string
	StdoutTTY bool
}

func (a *App) Run() int {
	if a.Stdout == nil {
		a.Stdout = os.Stdout
	}
	if a.Stderr == nil {
		a.Stderr = os.Stderr
	}
	if a.LookupEnv == nil {
		a.LookupEnv = os.Getenv
	}
	if a.Now == nil {
		a.Now = time.Now
	}
	if a.Loc == nil {
		a.Loc = time.Local
	}
	if a.GOOS == "" {
		a.GOOS = runtime.GOOS
	}
	if a.Version == "" {
		a.Version = ResolveVersion("dev")
	}

	flags, err := Parse(a.Args)
	if err != nil {
		fmt.Fprintln(a.Stderr, err.Error())
		return ExitUsage
	}
	if flags.Help {
		fmt.Fprint(a.Stdout, HelpText())
		return ExitOK
	}
	if flags.Version {
		fmt.Fprintln(a.Stdout, "wheretoken "+a.Version)
		return ExitOK
	}

	home := a.resolveHome(flags.Home)

	switch flags.Command {
	case CommandServe:
		return a.runServe(flags, home)
	case CommandScan:
		return a.runScanJSON(home)
	case CommandSources:
		return a.runSources(home)
	case CommandCompletion:
		script, err := Completion(flags.CompletionShell)
		if err != nil {
			fmt.Fprintln(a.Stderr, err.Error())
			return ExitUsage
		}
		fmt.Fprint(a.Stdout, script)
		return ExitOK
	default:
		return a.runReport(flags, home)
	}
}

func (a *App) resolveHome(override string) adapter.Home {
	if override != "" {
		return testhome.New(override)
	}
	if a.Home != nil {
		return a.Home
	}
	return scan.RealHome()
}

func (a *App) doScan(home adapter.Home) scan.Result {
	if a.Scan != nil {
		return a.Scan(home)
	}
	return scan.Run(home, scan.AllAdapters())
}

func (a *App) runReport(flags Flags, home adapter.Home) int {
	res := a.doScan(home)
	fil := report.Filter{
		Today:      flags.Today,
		Tool:       flags.Tool,
		Vendor:     flags.Vendor,
		Model:      flags.Model,
		Discovered: res.Summary.BySource,
	}
	snap, err := report.Build(res.Events, res.Turns, res.Errors, fil, a.Now(), a.Loc)
	if err != nil {
		fmt.Fprintln(a.Stderr, err.Error())
		if report.IsUsage(err) {
			return ExitUsage
		}
		return ExitFail
	}
	if flags.JSON {
		if err := report.WriteJSON(a.Stdout, snap); err != nil {
			fmt.Fprintln(a.Stderr, err.Error())
			return ExitFail
		}
		return ExitOK
	}
	ascii := table.UseASCII(flags.ASCII, a.GOOS, a.LookupEnv)
	color := table.UseColor(flags.NoColor, a.StdoutTTY, a.LookupEnv)
	out := report.Render(snap, report.Options{ASCII: ascii, Color: color})
	fmt.Fprint(a.Stdout, out)
	return ExitOK
}

func (a *App) runScanJSON(home adapter.Home) int {
	res := a.doScan(home)
	if err := scan.EncodeSummary(a.Stdout, res); err != nil {
		fmt.Fprintln(a.Stderr, err.Error())
		return ExitFail
	}
	return ExitOK
}

func (a *App) runSources(home adapter.Home) int {
	res := a.doScan(home)
	for _, root := range res.Roots {
		fmt.Fprintf(a.Stdout, "%s\t%s\n", root.ID, root.Path)
	}
	return ExitOK
}

func (a *App) runServe(flags Flags, home adapter.Home) int {
	start, end := flags.Port, flags.Port
	if flags.Port == 8787 {
		end = 8797
	}
	var lastErr error
	for p := start; p <= end; p++ {
		addr := fmt.Sprintf("127.0.0.1:%d", p)
		if a.Serve != nil {
			if err := a.Serve(addr, home); err != nil {
				fmt.Fprintln(a.Stderr, err.Error())
				return ExitFail
			}
			return ExitOK
		}
		ln, err := net.Listen("tcp", addr)
		if err != nil {
			lastErr = err
			continue
		}
		fmt.Fprintf(a.Stderr, "http://%s\n", addr)
		srv := &http.Server{Addr: addr, Handler: httpapi.NewMux(home)}
		if err := srv.Serve(ln); err != nil && err != http.ErrServerClosed {
			fmt.Fprintln(a.Stderr, err.Error())
			return ExitFail
		}
		return ExitOK
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("no port available")
	}
	fmt.Fprintln(a.Stderr, lastErr.Error())
	return ExitFail
}
