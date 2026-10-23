package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/rainhuang0220/whereToken/internal/adapter"
	"github.com/rainhuang0220/whereToken/internal/publicprofile"
)

func (a *App) profileRefreshRepair(home adapter.Home) int {
	if a.GOOS != "darwin" {
		fmt.Fprintln(a.Stdout, "scheduler=unsupported")
		fmt.Fprintln(a.Stdout, "healthy=yes")
		return ExitOK
	}
	on, err := publicprofile.LoadRefreshSwitch(publicprofile.RefreshConfigPath(home))
	if err != nil && !os.IsNotExist(err) {
		fmt.Fprintln(a.Stderr, "public profile refresh failed; will retry")
		return ExitFail
	}
	if err != nil || !on {
		fmt.Fprintln(a.Stdout, "scheduler=off")
		fmt.Fprintln(a.Stdout, "healthy=yes")
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
	if err := a.repairProfileRefreshAgents(program); err != nil {
		fmt.Fprintln(a.Stderr, "public profile refresh failed; will retry")
		return ExitFail
	}
	scheduler := a.schedulerStatus(true)
	fmt.Fprintf(a.Stdout, "scheduler=%s\n", scheduler)
	if scheduler != "running" {
		fmt.Fprintln(a.Stdout, "healthy=no")
		return ExitFail
	}
	fmt.Fprintln(a.Stdout, "healthy=yes")
	return ExitOK
}

// repairProfileRefreshAgents bootstraps a job only when it is missing or its
// program path changed. A running job with the current path is left alone.
func (a *App) repairProfileRefreshAgents(program string) error {
	home, err := a.userHome()
	if err != nil || home == "" {
		return errors.New("profile refresh agent is incomplete")
	}
	watchPlist, watchLog, err := a.profileRefreshPaths()
	if err != nil {
		return err
	}
	healthPlist, healthLog, err := a.profileRefreshHealthPaths()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(watchLog), 0o755); err != nil {
		return err
	}
	watchBody, err := profileRefreshPlist(program, home, watchLog)
	if err != nil {
		return err
	}
	healthBody, err := profileRefreshHealthPlist(program, healthLog)
	if err != nil {
		return err
	}
	if err := a.repairOne(profileRefreshLabel, watchPlist, watchBody, program); err != nil {
		return err
	}
	return a.repairOne(profileRefreshHealthLabel, healthPlist, healthBody, program)
}

func (a *App) repairOne(label, plistPath string, body []byte, program string) error {
	if a.agentNeedsBootstrap(label, plistPath, program) {
		return a.bootstrapPlist(plistPath, body)
	}
	return nil
}

func (a *App) agentNeedsBootstrap(label, plistPath, program string) bool {
	if _, err := os.Stat(plistPath); err != nil {
		return true
	}
	got, err := plistProgram(plistPath)
	if err != nil || got != program {
		return true
	}
	out, err := a.launchctl("print", fmt.Sprintf("gui/%d/%s", a.uid(), label))
	if err != nil || !launchctlHasPID(out) {
		return true
	}
	return false
}

func (a *App) refreshPlistState(enabled bool, program string) string {
	if a.GOOS != "darwin" {
		return "unsupported"
	}
	plistPath, _, err := a.profileRefreshPaths()
	if err != nil {
		return "missing"
	}
	if _, err := os.Stat(plistPath); err != nil {
		if !enabled {
			return "ok"
		}
		return "missing"
	}
	if !enabled {
		return "stale"
	}
	got, err := plistProgram(plistPath)
	if err != nil || got == "" || (program != "" && got != program) {
		return "stale"
	}
	return "ok"
}
