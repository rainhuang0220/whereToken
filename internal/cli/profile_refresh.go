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
	"syscall"
	"time"

	"github.com/rainhuang0220/whereToken/internal/adapter"
	"github.com/rainhuang0220/whereToken/internal/credstore"
	"github.com/rainhuang0220/whereToken/internal/proclock"
	"github.com/rainhuang0220/whereToken/internal/publicprofile"
	"github.com/rainhuang0220/whereToken/internal/scan"
)

const profileRefreshInterval = 15 * time.Minute

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
	case "":
		return a.executeProfileRefresh(context.Background(), flags, home, nil, false, nil)
	default:
		fmt.Fprintf(a.Stderr, "unknown profile refresh action %q\ntry `wheretoken --help`\n", flags.ProfileRefreshAction)
		return ExitUsage
	}
}

func (a *App) profileRefreshStatus(home adapter.Home) int {
	on, err := publicprofile.LoadRefreshSwitch(publicprofile.RefreshConfigPath(home))
	if err != nil && !os.IsNotExist(err) {
		fmt.Fprintln(a.Stderr, "public profile refresh failed; will retry")
		return ExitFail
	}
	if err != nil || !on {
		fmt.Fprintln(a.Stdout, "phase=idle 未开启")
		return ExitOK
	}
	fmt.Fprintln(a.Stdout, "phase=idle 等待下一次")
	return ExitOK
}

func (a *App) profileRefreshSet(home adapter.Home, on bool) int {
	if err := publicprofile.SaveRefreshSwitch(publicprofile.RefreshConfigPath(home), on); err != nil {
		fmt.Fprintln(a.Stderr, "public profile refresh failed; will retry")
		return ExitFail
	}
	if on {
		fmt.Fprintln(a.Stdout, "public profile refresh on (sanitized snapshot only — prompts and paths stay local)")
		return ExitOK
	}
	fmt.Fprintln(a.Stdout, "public profile refresh off (GitHub profile stays as last published)")
	return ExitOK
}

func (a *App) profileRefreshWatch(flags Flags, home adapter.Home) int {
	on, err := publicprofile.LoadRefreshSwitch(publicprofile.RefreshConfigPath(home))
	if err != nil && !os.IsNotExist(err) {
		fmt.Fprintln(a.Stderr, "public profile refresh failed; will retry")
		return ExitFail
	}
	if err != nil || !on {
		fmt.Fprintln(a.Stdout, "phase=idle 未开启")
		return ExitOK
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	rejected := map[string]struct{}{}
	for {
		if ctx.Err() != nil {
			return ExitOK
		}
		a.executeProfileRefresh(ctx, flags, home, nil, false, rejected)
		timer := time.NewTimer(profileRefreshInterval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ExitOK
		case <-timer.C:
		}
	}
}

func (a *App) maybeProfileRefresh(flags Flags, home adapter.Home, res scan.Result) {
	on, err := publicprofile.LoadRefreshSwitch(publicprofile.RefreshConfigPath(home))
	if err != nil || !on {
		return
	}
	a.executeProfileRefresh(context.Background(), flags, home, &res, true, nil)
}

func (a *App) executeProfileRefresh(ctx context.Context, flags Flags, home adapter.Home, preset *scan.Result, fromReport bool, rejected map[string]struct{}) int {
	out := io.Writer(a.Stdout)
	if fromReport {
		out = a.Stderr
	}
	if a.wantOffline(flags) {
		fmt.Fprintln(out, "phase=skipped 离线不覆盖已发布快照")
		return ExitOK
	}
	token, login, err := a.profileRefreshCreds(home)
	if err != nil {
		fmt.Fprintln(a.Stderr, "not logged in; run wheretoken login")
		return ExitOK
	}
	if preset == nil {
		release, ok, lockErr := proclock.TryLock(a.scanLockPath(home))
		if lockErr != nil {
			return a.refreshFailed(out, fromReport)
		}
		if !ok {
			fmt.Fprintln(out, "phase=skipped 已有扫描，跳过本次")
			return ExitOK
		}
		defer release()
	}
	res := scan.Result{}
	if preset != nil {
		res = *preset
	} else {
		res = a.doScanOpt(home, flags.Quiet, flags.Offline, flags.ASCII, false)
	}
	snap, err := publicprofile.Build(publicprofile.Input{
		Events:       res.Events,
		Turns:        res.Turns,
		Now:          a.Now(),
		Loc:          a.Loc,
		Version:      a.Version,
		PortraitSeed: a.PortraitSeed(home),
		Offline:      res.Offline,
		Errors:       res.Errors,
	})
	if err != nil {
		return a.refreshFailed(out, fromReport)
	}
	if snap.SnapshotID != "" && rejected != nil {
		if _, seen := rejected[snap.SnapshotID]; seen {
			fmt.Fprintln(out, "phase=idle 等待下一次")
			return ExitOK
		}
	}
	applied := publicprofile.ApplyRefresh(ctx, snap, a.Now(), false, projectionClient{app: a, token: token, login: login})
	if applied.Auth {
		fmt.Fprintln(a.Stderr, "not logged in; run wheretoken login")
		return ExitOK
	}
	if applied.Rejected && rejected != nil && snap.SnapshotID != "" {
		rejected[snap.SnapshotID] = struct{}{}
	}
	if applied.Transport || applied.Rejected {
		return a.refreshFailed(out, fromReport)
	}
	a.writeRefresh(out, applied.Decision, !fromReport)
	return ExitOK
}

func (a *App) refreshFailed(out io.Writer, fromReport bool) int {
	if fromReport {
		fmt.Fprintln(a.Stderr, "public profile refresh failed; will retry")
		return ExitOK
	}
	fmt.Fprintln(out, "phase=failed 更新失败，下次再试")
	fmt.Fprintln(out, publicprofile.CodeWillRetry)
	fmt.Fprintln(a.Stderr, "public profile refresh failed; will retry")
	return ExitFail
}

func (a *App) writeRefresh(w io.Writer, d publicprofile.Decision, withCode bool) {
	fmt.Fprintf(w, "phase=%s %s\n", d.Phase, d.Label)
	if withCode && d.Code != "" {
		fmt.Fprintln(w, d.Code)
	}
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
	resp, err := p.app.doHTTP(req)
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
	return p.app.putSanitizedProjection(ctx, p.token, body)
}

func FormatProfileRefreshDoctor(home adapter.Home) string {
	on, err := publicprofile.LoadRefreshSwitch(publicprofile.RefreshConfigPath(home))
	if err != nil || !on {
		return "Public profile refresh\n  · Off\n"
	}
	return "Public profile refresh\n  · On · sanitized snapshot only\n"
}
