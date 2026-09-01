package cli

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/rainhuang0220/whereToken/internal/adapter"
	"github.com/rainhuang0220/whereToken/internal/adapter/testhome"
	"github.com/rainhuang0220/whereToken/internal/community"
	"github.com/rainhuang0220/whereToken/internal/httpapi"
	"github.com/rainhuang0220/whereToken/internal/index"
	"github.com/rainhuang0220/whereToken/internal/metric"
	"github.com/rainhuang0220/whereToken/internal/report"
	"github.com/rainhuang0220/whereToken/internal/scan"
	"github.com/rainhuang0220/whereToken/internal/table"
)

type App struct {
	Args       []string
	Stdout     io.Writer
	Stderr     io.Writer
	Version    string
	Now        func() time.Time
	Loc        *time.Location
	LookupEnv  func(string) string
	Scan       func(adapter.Home) scan.Result
	Home       adapter.Home
	Serve      func(addr string, home adapter.Home, offline bool) error
	Executable func() (string, error)
	HTTPGet    func(url string) ([]byte, error)
	RunCmd     func(name string, args ...string) error
	GOOS       string
	GOARCH     string
	StdoutTTY  bool
	StderrTTY  bool
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
	if a.GOARCH == "" {
		a.GOARCH = runtime.GOARCH
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
		return a.runScanJSON(home, flags.Quiet, flags.Offline)
	case CommandSources:
		return a.runSources(home, flags.Quiet, flags.Offline)
	case CommandDoctor:
		return a.runDoctor(home, flags.Quiet, flags.Offline || flags.NoCommunity)
	case CommandRebuild:
		return a.runRebuild(flags, home)
	case CommandUpdate:
		return a.runUpdate(flags.Quiet)
	case CommandUninstall:
		return a.runUninstall(flags.Quiet)
	case CommandCompletion:
		script, err := Completion(flags.CompletionShell)
		if err != nil {
			fmt.Fprintln(a.Stderr, err.Error())
			return ExitUsage
		}
		fmt.Fprint(a.Stdout, script)
		return ExitOK
	case CommandCommunity:
		return a.runCommunity(flags, home)
	case CommandPricing:
		return a.runPricing(flags)
	default:
		return a.runReport(flags, home)
	}
}

func (a *App) resolveHome(override string) adapter.Home {
	if override != "" {
		return testhome.New(override)
	}
	if a.LookupEnv != nil {
		if v := strings.TrimSpace(a.LookupEnv("WHERETOKEN_HOME")); v != "" {
			return testhome.New(v)
		}
	}
	if a.Home != nil {
		return a.Home
	}
	return scan.RealHome()
}

func (a *App) doScan(home adapter.Home, quiet, offline, ascii bool) scan.Result {
	if a.envOffline() {
		offline = true
	}
	if a.Scan != nil {
		res := a.Scan(home)
		res.Offline = offline
		return res
	}
	ads := scan.Adapters(offline)
	var res scan.Result
	if quiet || !a.StderrTTY {
		res = scan.Run(home, ads)
	} else {
		asciiHUD := ascii || table.UseASCII(false, a.GOOS, a.LookupEnv)
		hud := scanHUD{
			w:     a.Stderr,
			ascii: asciiHUD,
			color: table.UseColor(false, a.StderrTTY, a.LookupEnv),
			now:   a.Now,
		}
		res = scan.RunWithProgress(home, ads, func(p scan.Progress) {
			if p.Status == scan.ProgressReading {
				hud.Show(p)
				return
			}
			if p.Index >= p.Total {
				hud.Clear()
			}
		})
		hud.Clear()
	}
	res.Offline = offline
	if !quiet && a.StderrTTY {
		if msg := scan.FormatDeltas(res.Deltas); msg != "" {
			fmt.Fprint(a.Stderr, msg)
		}
	}
	return res
}

func (a *App) runRebuild(flags Flags, home adapter.Home) int {
	if err := index.Wipe(index.PathFor(home)); err != nil {
		fmt.Fprintln(a.Stderr, err.Error())
		return ExitFail
	}
	if !flags.Quiet {
		fmt.Fprintln(a.Stderr, "rebuilt local index")
	}
	return a.runReport(flags, home)
}

func (a *App) runReport(flags Flags, home adapter.Home) int {
	res := a.doScan(home, flags.Quiet, flags.Offline, flags.ASCII)
	win, err := metric.ParseWindow(flags.Today, flags.Since, flags.From, flags.To, a.Now(), a.Loc)
	if err != nil {
		fmt.Fprintln(a.Stderr, err.Error())
		return ExitUsage
	}
	fil := report.Filter{
		Today:      flags.Today,
		Days:       win.Days,
		From:       win.From,
		To:         win.To,
		Period:     win.Label,
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
	if a.wantOffline(flags) {
		const msg = "offline · 只用本机账本，没有请求 Cursor/Trae 云端"
		if !slices.Contains(snap.Notes, msg) {
			snap.Notes = append([]string{msg}, snap.Notes...)
		}
	}
	a.attachCommunity(&snap, flags, home, res.Events)
	if flags.JSON {
		if err := report.WriteJSON(a.Stdout, snap); err != nil {
			fmt.Fprintln(a.Stderr, err.Error())
			return ExitFail
		}
		return ExitOK
	}
	ascii := table.UseASCII(flags.ASCII, a.GOOS, a.LookupEnv)
	color := table.UseColor(flags.NoColor, a.StdoutTTY, a.LookupEnv)
	out := report.Render(snap, report.Options{ASCII: ascii, Color: color, Width: resolveWidth(flags.Width, a.LookupEnv, a.termWidth)})
	fmt.Fprint(a.Stdout, out)
	return ExitOK
}

func (a *App) runScanJSON(home adapter.Home, quiet, offline bool) int {
	res := a.doScan(home, quiet, offline, false)
	if err := scan.EncodeSummary(a.Stdout, res); err != nil {
		fmt.Fprintln(a.Stderr, err.Error())
		return ExitFail
	}
	return ExitOK
}

func (a *App) runSources(home adapter.Home, quiet, offline bool) int {
	res := a.doScan(home, quiet, offline, false)
	if len(res.Roots) == 0 {
		fmt.Fprintln(a.Stderr, "没有找到本机账本")
		return ExitOK
	}
	fmt.Fprintf(a.Stdout, "agent\tdetected\tusage\tquality\tpath\n")
	for _, st := range scan.Diagnose(res) {
		if !st.Detected {
			continue
		}
		fmt.Fprintf(a.Stdout, "%s\tyes\t%s\t%s\t%s\n",
			st.ID, yn(st.Usage), string(st.Quality), st.Path)
	}
	return ExitOK
}

func (a *App) runDoctor(home adapter.Home, quiet, offline bool) int {
	res := a.doScan(home, quiet, offline, false)
	fmt.Fprint(a.Stdout, FormatDoctor(scan.Diagnose(res)))
	fmt.Fprint(a.Stdout, FormatCommunityDoctor(home, a.LookupEnv, offline))
	return ExitOK
}

func yn(v bool) string {
	if v {
		return "yes"
	}
	return "no"
}

func FormatDoctor(rows []scan.AgentStatus) string {
	var b strings.Builder
	for _, st := range rows {
		fmt.Fprintf(&b, "%s\n", st.Label)
		if st.Detected {
			fmt.Fprintf(&b, "  ✓ Source detected\n")
			if st.Path != "" {
				fmt.Fprintf(&b, "    %s\n", st.Path)
			}
		} else {
			fmt.Fprintf(&b, "  · Source not found on this machine\n")
		}
		if st.Usage {
			fmt.Fprintf(&b, "  ✓ Usage data available\n")
		} else if st.Detected {
			fmt.Fprintf(&b, "  ⚠ Usage data unavailable\n")
		}
		if st.Quality != "" {
			fmt.Fprintf(&b, "  · Quality: %s\n", st.Quality)
		}
		if st.Error != "" {
			fmt.Fprintf(&b, "    %s\n", st.Error)
		}
		b.WriteByte('\n')
	}
	return b.String()
}

func (a *App) runServe(flags Flags, home adapter.Home) int {
	start, end := flags.Port, flags.Port
	if flags.Port == 8787 {
		end = 8797
	}
	var lastErr error
	offline := a.wantOffline(flags)
	for p := start; p <= end; p++ {
		addr := fmt.Sprintf("127.0.0.1:%d", p)
		if a.Serve != nil {
			fmt.Fprint(a.Stderr, ServeStartedMessage(addr))
			if err := a.Serve(addr, home, offline); err != nil {
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
		fmt.Fprint(a.Stderr, ServeStartedMessage(addr))
		srv := httpapi.NewHTTPServerFull(addr, home, offline, flags.NoCommunity || community.EnvDisabled(a.LookupEnv), a.Version)
		if err := srv.Serve(ln); err != nil && err != http.ErrServerClosed {
			fmt.Fprintln(a.Stderr, err.Error())
			return ExitFail
		}
		return ExitOK
	}
	fmt.Fprintln(a.Stderr, lastErr.Error())
	return ExitFail
}

func ServeStartedMessage(addr string) string {
	return fmt.Sprintf("http://%s\n页内「刷新」重新扫描本机；浏览器重载只会显示上次结果。\n", addr)
}

func resolveWidth(flag int, getenv func(string) string, size func() int) int {
	if flag > 0 {
		return flag
	}
	if getenv != nil {
		c := strings.TrimSpace(getenv("COLUMNS"))
		if c != "" {
			n, err := strconv.Atoi(c)
			if err == nil && n > 0 {
				return n
			}
		}
	}
	if size != nil {
		if n := size(); n > 0 {
			return n
		}
	}
	return 0
}

func (a *App) termWidth() int {
	if !a.StdoutTTY {
		return 0
	}
	w, _, err := terminalSize(int(os.Stdout.Fd()))
	if err != nil || w <= 0 {
		return 0
	}
	return w
}

func (a *App) wantOffline(flags Flags) bool {
	return flags.Offline || a.envOffline()
}

func (a *App) envOffline() bool {
	if a.LookupEnv == nil {
		return false
	}
	v := a.LookupEnv("WHERETOKEN_OFFLINE")
	return v == "1" || v == "true"
}
