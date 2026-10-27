package cli

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/rainhuang0220/whereToken/internal/adapter"
	"github.com/rainhuang0220/whereToken/internal/credstore"
	"github.com/rainhuang0220/whereToken/internal/event"
	"github.com/rainhuang0220/whereToken/internal/ledger"
	"github.com/rainhuang0220/whereToken/internal/proclock"
	"github.com/rainhuang0220/whereToken/internal/publicprofile"
	"github.com/rainhuang0220/whereToken/internal/scan"
)

// profileRefreshInterval is the foreground watch period. Launchd does not
// use StartInterval on the watch job; this timer stays inside watch.
// A failed attempt is retried on the next wake. Midnight still wakes sooner.
// Tests shorten it.
var profileRefreshInterval = time.Hour

// profileRefreshTimeout bounds one refresh GET or PUT. http.DefaultClient
// has no timeout; refresh does not use it.
var profileRefreshTimeout = 30 * time.Second

func (a *App) refreshDo(req *http.Request) (*http.Response, error) {
	if a.HTTPDo != nil {
		return a.HTTPDo(req)
	}
	return (&http.Client{Timeout: profileRefreshTimeout}).Do(req)
}

func (a *App) runProfileRefreshCommand(flags Flags, home adapter.Home) int {
	switch flags.ProfileRefreshAction {
	case "status":
		return a.profileRefreshStatus(home)
	case "on":
		return a.profileRefreshSet(home, true)
	case "off":
		return a.profileRefreshSet(home, false)
	case "watch":
		return a.profileRefreshWatch(flags, home)
	case "doctor":
		return a.profileRefreshDoctor(home)
	case "repair":
		return a.profileRefreshRepair(home)
	case "":
		return a.executeProfileRefresh(context.Background(), flags, home, nil)
	default:
		fmt.Fprintf(a.Stderr, "unknown profile refresh action %q\ntry `wheretoken --help`\n", flags.ProfileRefreshAction)
		return ExitUsage
	}
}

func (a *App) profileRefreshStatus(home adapter.Home) int {
	return a.writeRefreshHealth(home, false)
}

func (a *App) profileRefreshDoctor(home adapter.Home) int {
	return a.writeRefreshHealth(home, true)
}

func (a *App) writeRefreshHealth(home adapter.Home, doctor bool) int {
	on, err := publicprofile.LoadRefreshSwitch(publicprofile.RefreshConfigPath(home))
	if err != nil && !os.IsNotExist(err) {
		fmt.Fprintln(a.Stderr, "public profile refresh failed; will retry")
		return ExitFail
	}
	enabled := err == nil && on
	if err != nil || !on {
		fmt.Fprintln(a.Stdout, "phase=idle 未开启")
	} else {
		fmt.Fprintln(a.Stdout, "phase=idle 等待下一次")
	}
	scheduler := a.schedulerStatus(enabled)
	code := a.lastRefreshCode(home)
	fmt.Fprintf(a.Stdout, "scheduler=%s\n", scheduler)
	fmt.Fprintf(a.Stdout, "last=%s\n", code)
	program, _ := a.refreshProgramPath()
	plist := a.refreshPlistState(enabled, program)
	if doctor {
		fmt.Fprintf(a.Stdout, "plist=%s\n", plist)
		facts, tokens, gaps := ledgerFigures(home)
		fmt.Fprintf(a.Stdout, "ledger_facts=%s\n", facts)
		fmt.Fprintf(a.Stdout, "ledger_tokens=%s\n", tokens)
		fmt.Fprintf(a.Stdout, "ledger_import_gaps=%s\n", gaps)
	}
	healthy := !refreshUnhealthy(enabled, scheduler, code, plist, doctor)
	if healthy {
		fmt.Fprintln(a.Stdout, "healthy=yes")
		return ExitOK
	}
	fmt.Fprintln(a.Stdout, "healthy=no")
	return ExitFail
}

func refreshUnhealthy(enabled bool, scheduler, code, plist string, doctor bool) bool {
	if !enabled {
		return false
	}
	if scheduler == "dead" {
		return true
	}
	switch code {
	case publicprofile.CodeBlockedData, publicprofile.CodeBlockedRegression, publicprofile.CodeBlockedAuth, publicprofile.CodeNotSignedIn, publicprofile.CodeWillRetry:
		return true
	}
	if doctor && (plist == "missing" || plist == "stale") {
		return true
	}
	return false
}

func ledgerFigures(home adapter.Home) (facts, tokens, gaps string) {
	unavailable := func() (string, string, string) {
		return "unavailable", "unavailable", "unavailable"
	}
	if os.Getenv("WHERETOKEN_NO_LEDGER") == "1" {
		return unavailable()
	}
	p, err := ledger.PathFor(home)
	if err != nil {
		return unavailable()
	}
	lg, err := ledger.Open(p)
	if err != nil {
		return unavailable()
	}
	defer lg.Close()
	st, err := lg.Status()
	if err != nil {
		return unavailable()
	}
	return strconv.Itoa(st.Facts), strconv.FormatInt(st.Tokens, 10), strconv.Itoa(st.ImportGaps)
}

func (a *App) lastRefreshCode(home adapter.Home) string {
	st, err := publicprofile.LoadRefreshState(publicprofile.RefreshStatePath(home))
	if err != nil || !publicprofile.AllowedRefreshCode(st.Code) {
		return ""
	}
	return st.Code
}

func (a *App) noteRefresh(home adapter.Home, phase, code string) {
	if a.Now == nil {
		return
	}
	_ = publicprofile.SaveRefreshState(publicprofile.RefreshStatePath(home), publicprofile.RefreshState{
		Phase:     phase,
		Code:      code,
		CheckedAt: a.Now(),
	})
}

func (a *App) profileRefreshSet(home adapter.Home, on bool) int {
	if !on {
		if err := publicprofile.SaveRefreshSwitch(publicprofile.RefreshConfigPath(home), false); err != nil {
			fmt.Fprintln(a.Stderr, "public profile refresh failed; will retry")
			return ExitFail
		}
		a.stopProfileRefreshAgent()
		fmt.Fprintln(a.Stdout, "public profile refresh off (GitHub profile stays as last published)")
		if a.GOOS == "darwin" {
			fmt.Fprintln(a.Stdout, "scheduler=stopped")
		} else {
			fmt.Fprintln(a.Stdout, "scheduler=unsupported")
		}
		return ExitOK
	}
	if a.GOOS != "darwin" {
		if err := publicprofile.SaveRefreshSwitch(publicprofile.RefreshConfigPath(home), true); err != nil {
			fmt.Fprintln(a.Stderr, "public profile refresh failed; will retry")
			return ExitFail
		}
		fmt.Fprintln(a.Stdout, "public profile refresh on (sanitized snapshot only — prompts and paths stay local)")
		fmt.Fprintf(a.Stdout, "scheduler=unsupported\nThis version does not install a background agent on %s. Public auto-refresh is macOS-only. Foreground loop: wheretoken profile refresh watch\n", a.GOOS)
		return ExitOK
	}
	if a.euid() == 0 {
		fmt.Fprintln(a.Stderr, "public profile refresh failed; will retry")
		return ExitFail
	}
	program, err := a.refreshProgramPath()
	if err != nil || program == "" {
		fmt.Fprintln(a.Stderr, "public profile refresh failed; will retry")
		return ExitFail
	}
	// watch treats a missing switch as off and exits 0. KeepAlive does not
	// restart that exit, so the file has to be durable before bootstrap.
	if err := publicprofile.SaveRefreshSwitch(publicprofile.RefreshConfigPath(home), true); err != nil {
		fmt.Fprintln(a.Stderr, "public profile refresh failed; will retry")
		return ExitFail
	}
	if err := a.installProfileRefreshAgent(program); err != nil {
		fmt.Fprintln(a.Stderr, "public profile refresh failed; will retry")
		return ExitFail
	}
	fmt.Fprintln(a.Stdout, "public profile refresh on (sanitized snapshot only — prompts and paths stay local)")
	return ExitOK
}

func (a *App) profileRefreshWatch(flags Flags, home adapter.Home) int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	rejected := map[string]struct{}{}
	for {
		if ctx.Err() != nil {
			return ExitOK
		}
		on, err := publicprofile.LoadRefreshSwitch(publicprofile.RefreshConfigPath(home))
		if err != nil && !os.IsNotExist(err) {
			fmt.Fprintln(a.Stderr, "public profile refresh failed; will retry")
			return ExitFail
		}
		if err != nil || !on {
			fmt.Fprintln(a.Stdout, "phase=idle 未开启")
			return ExitOK
		}
		a.maybeReexecProfileWatch()
		a.executeProfileRefresh(ctx, flags, home, rejected)
		wait := publicprofile.NextRefreshWake(a.Now(), a.Loc, profileRefreshInterval).Sub(a.Now())
		if wait < 0 {
			wait = 0
		}
		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ExitOK
		case <-timer.C:
		}
	}
}

func (a *App) executeProfileRefresh(ctx context.Context, flags Flags, home adapter.Home, rejected map[string]struct{}) int {
	if a.wantOffline(flags) {
		a.noteRefresh(home, publicprofile.PhaseSkipped, publicprofile.CodeOffline)
		fmt.Fprintln(a.Stdout, "phase=skipped 离线不覆盖已发布快照")
		return ExitOK
	}
	token, login, err := a.profileRefreshCreds(home)
	if err != nil {
		a.noteRefresh(home, publicprofile.PhaseIdle, publicprofile.CodeNotSignedIn)
		fmt.Fprintln(a.Stderr, "not logged in; run wheretoken login")
		return ExitOK
	}
	release, ok, lockErr := proclock.TryLock(a.scanLockPath(home))
	if lockErr != nil {
		a.noteRefresh(home, publicprofile.PhaseFailed, publicprofile.CodeWillRetry)
		return a.refreshFailed()
	}
	if !ok {
		a.noteRefresh(home, publicprofile.PhaseSkipped, publicprofile.CodeBusy)
		fmt.Fprintln(a.Stdout, "phase=skipped 已有扫描，跳过本次")
		return ExitOK
	}
	defer release()
	res := a.doScanOpt(home, flags.Quiet, flags.Offline, flags.ASCII, false)
	events, restored := a.retainAccountObservations(home, res.Events, res.Errors, nil)
	snap, err := a.refreshSnapshot(home, res, events)
	if err != nil {
		a.noteRefresh(home, publicprofile.PhaseFailed, publicprofile.CodeWillRetry)
		return a.refreshFailed()
	}
	if snap.SnapshotID != "" && rejected != nil {
		if _, seen := rejected[snap.SnapshotID]; seen {
			fmt.Fprintln(a.Stdout, "phase=idle 等待下一次")
			return ExitOK
		}
	}
	ctx = a.withCorrectionPermit(ctx, home)
	publisher := projectionClient{app: a, token: token, login: login}
	applied := publicprofile.ApplyRefresh(ctx, snap, a.Now(), false, publisher)
	if !restored && applied.Decision.Code == publicprofile.CodeBlockedData && applied.Remote != nil {
		seeded, ok := a.retainAccountObservations(home, res.Events, res.Errors, applied.Remote)
		if ok {
			snap, err = a.refreshSnapshot(home, res, seeded)
			if err != nil {
				a.noteRefresh(home, publicprofile.PhaseFailed, publicprofile.CodeWillRetry)
				return a.refreshFailed()
			}
			applied = publicprofile.ApplyRefresh(ctx, snap, a.Now(), false, publisher)
		}
	}
	if applied.Auth {
		a.noteRefresh(home, publicprofile.PhaseIdle, publicprofile.CodeNotSignedIn)
		fmt.Fprintln(a.Stderr, "not logged in; run wheretoken login")
		return ExitOK
	}
	if applied.Rejected && rejected != nil && snap.SnapshotID != "" {
		rejected[snap.SnapshotID] = struct{}{}
	}
	if applied.Transport || applied.Rejected {
		a.noteRefresh(home, publicprofile.PhaseFailed, publicprofile.CodeWillRetry)
		return a.refreshFailed()
	}
	a.clearPermitIfPublished(home, applied)
	a.noteRefresh(home, applied.Decision.Phase, applied.Decision.Code)
	a.writeRefresh(a.Stdout, applied.Decision, true)
	return ExitOK
}

func (a *App) refreshSnapshot(home adapter.Home, res scan.Result, events []event.UsageEvent) (publicprofile.Snapshot, error) {
	return publicprofile.Build(publicprofile.Input{
		Events:       events,
		Turns:        res.Turns,
		Now:          a.Now(),
		Loc:          a.Loc,
		Version:      a.Version,
		PortraitSeed: a.PortraitSeed(home),
		Offline:      res.Offline,
		Errors:       res.Errors,
	})
}

// retainAccountObservations reuses a confirmed account-API observation when
// the fresh fetch failed. remote, when set, can seed that observation from
// the accepted public snapshot. Local token rows for that source are cleared
// so the observation is not added on top of them.
func (a *App) retainAccountObservations(home adapter.Home, events []event.UsageEvent, errs []string, remote *publicprofile.Snapshot) ([]event.UsageEvent, bool) {
	if os.Getenv("WHERETOKEN_NO_LEDGER") == "1" {
		return events, false
	}
	p, err := ledger.PathFor(home)
	if err != nil {
		return events, false
	}
	lg, err := ledger.Open(p)
	if err != nil {
		return events, false
	}
	defer lg.Close()
	changed := false
	if next, err := lg.SupplementNarrowWindow(events); err == nil && len(next) != len(events) {
		events = next
		changed = true
	}
	for _, source := range failedAccountSources(errs) {
		if !publicprofile.AccountFetchFailed(events, errs, source) {
			continue
		}
		got, ok, err := lg.LatestRetainedEvents(source)
		if err == nil && ok {
			events = publicprofile.ApplyRetainedEvents(events, got)
			changed = true
			continue
		}
		if remote == nil {
			continue
		}
		days, ok := publicprofile.DaysFromSnapshot(*remote, source)
		if !ok {
			continue
		}
		observed, err := time.Parse(time.RFC3339, remote.GeneratedAt)
		if err != nil {
			continue
		}
		if err := lg.SaveObservation(source, "verified_snapshot", "retained", observed, publicprofile.EventsFromDays(source, days)); err != nil {
			continue
		}
		got, ok, err = lg.LatestRetainedEvents(source)
		if err != nil || !ok {
			continue
		}
		events = publicprofile.ApplyRetainedEvents(events, got)
		changed = true
	}
	return events, changed
}

// withCorrectionPermit leaves the local ledger permit in the ledger.
// Attaching it would make this refresh try to publish a lower total.
// The public route does not accept that attempt.
func (a *App) withCorrectionPermit(ctx context.Context, _ adapter.Home) context.Context {
	return ctx
}

// clearPermitIfPublished does not consume the local permit. A verified
// upload, a cooldown, or a pending GitHub write is not proof that a
// correction was published, and a failed write must not burn the permit.
func (a *App) clearPermitIfPublished(_ adapter.Home, _ publicprofile.ApplyResult) {
}

func (a *App) openRefreshLedger(home adapter.Home) (*ledger.Ledger, error) {
	p, err := ledger.PathFor(home)
	if err != nil {
		return nil, err
	}
	return ledger.Open(p)
}

func failedAccountSources(errs []string) []string {
	var out []string
	seen := map[string]struct{}{}
	for _, msg := range errs {
		i := strings.IndexByte(msg, ':')
		if i <= 0 {
			continue
		}
		id := msg[:i]
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

func (a *App) refreshFailed() int {
	fmt.Fprintln(a.Stdout, "phase=failed 更新失败，下次再试")
	fmt.Fprintln(a.Stdout, publicprofile.CodeWillRetry)
	fmt.Fprintln(a.Stderr, "public profile refresh failed; will retry")
	return ExitFail
}

func (a *App) writeRefresh(w io.Writer, d publicprofile.Decision, withCode bool) {
	fmt.Fprintf(w, "phase=%s %s\n", d.Phase, d.Label)
	if withCode && d.Code != "" {
		fmt.Fprintln(w, d.Code)
	}
	if refreshDetailSafe(d.Detail) {
		fmt.Fprintf(w, "detail=%s\n", d.Detail)
	}
}

func refreshDetailSafe(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r != ',' && (r < 'a' || r > 'z') && (r < '0' || r > '9') {
			return false
		}
	}
	return true
}

func (a *App) profileRefreshCreds(home adapter.Home) (token, login string, err error) {
	cs := a.credStore(home)
	token, err = cs.Get(credstore.KeyDeviceToken)
	if err != nil || token == "" {
		return "", "", fmt.Errorf("not logged in; run wheretoken login")
	}
	login, err = cs.Get(credstore.KeyLogin)
	if err != nil || !refreshLoginOK(login) {
		return "", "", fmt.Errorf("not logged in; run wheretoken login")
	}
	return token, login, nil
}

func refreshLoginOK(login string) bool {
	if login == "" || len(login) > 39 {
		return false
	}
	for i, c := range login {
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9':
		case c == '-' && i > 0 && i < len(login)-1:
		default:
			return false
		}
	}
	return true
}

func (a *App) scanLockPath(home adapter.Home) string {
	return filepath.Join(filepath.Dir(publicprofile.RefreshConfigPath(home)), "scan.lock")
}

type projectionClient struct {
	app   *App
	token string
	login string
}

func (p projectionClient) Fetch(ctx context.Context) (publicprofile.RemoteView, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.app.hostedBase()+"/api/v1/public-profile/"+url.PathEscape(p.login), nil)
	if err != nil {
		return publicprofile.RemoteView{}, err
	}
	resp, err := p.app.refreshDo(req)
	if err != nil {
		return publicprofile.RemoteView{}, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return publicprofile.RemoteView{}, err
	}
	switch {
	case resp.StatusCode == http.StatusNotFound:
		return publicprofile.RemoteView{}, nil
	case resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden:
		return publicprofile.RemoteView{}, publicprofile.ErrNotSignedIn
	case resp.StatusCode >= 400:
		return publicprofile.RemoteView{}, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return publicprofile.ParseLiveEnvelope(body)
}

func (p projectionClient) Put(ctx context.Context, body []byte) (publicprofile.PutResult, error) {
	return p.app.putSanitizedProjectionDo(ctx, p.token, body, p.app.refreshDo)
}

func FormatProfileRefreshDoctor(home adapter.Home) string {
	on, err := publicprofile.LoadRefreshSwitch(publicprofile.RefreshConfigPath(home))
	if err != nil || !on {
		return "Public profile refresh\n  · Off\n"
	}
	return "Public profile refresh\n  · On · sanitized snapshot only\n"
}
