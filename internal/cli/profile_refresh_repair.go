package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/rainhuang0220/whereToken/internal/adapter"
	"github.com/rainhuang0220/whereToken/internal/publicprofile"
)

func (a *App) profileRefreshPrepareDowngrade(home adapter.Home) int {
	raw, readErr := os.ReadFile(profileRefreshSchedulePath(home))
	if readErr == nil {
		backup := profileRefreshSchedulePath(home) + ".pre-downgrade"
		if _, err := os.Stat(backup); os.IsNotExist(err) {
			if err := writeAtomicFile(backup, raw, 0o600); err != nil {
				fmt.Fprintln(a.Stderr, "public profile refresh failed; will retry")
				return ExitFail
			}
		}
	} else if !os.IsNotExist(readErr) {
		fmt.Fprintln(a.Stderr, "public profile refresh failed; will retry")
		return ExitFail
	}
	st, _, err := a.refreshSchedule(home, a.Now())
	if err != nil {
		fmt.Fprintln(a.Stderr, "public profile refresh failed; will retry")
		return ExitFail
	}
	if err := saveProfileRefreshSchedule(home, st); err != nil {
		fmt.Fprintln(a.Stderr, "public profile refresh failed; will retry")
		return ExitFail
	}
	fmt.Fprintf(a.Stdout, "schedule_attempts=%d\nschedule_failures=%d\nschedule_schema=2\n", st.Attempts, st.Failures)
	fmt.Fprintf(a.Stdout, "evidence=%s\n", profileRefreshEvidencePath(home))
	fmt.Fprintln(a.Stdout, "ledger_untouched=yes")
	fmt.Fprintln(a.Stdout, "downgrade_ready=yes")
	return ExitOK
}

func (a *App) profileRefreshRepair(home adapter.Home) int {
	if a.GOOS != "darwin" {
		fmt.Fprintln(a.Stdout, "scheduler=unsupported")
		fmt.Fprintln(a.Stdout, "scheduler_healthy=unavailable")
		return ExitOK
	}
	on, err := publicprofile.LoadRefreshSwitch(publicprofile.RefreshConfigPath(home))
	if err != nil && !os.IsNotExist(err) {
		fmt.Fprintln(a.Stderr, "public profile refresh failed; will retry")
		return ExitFail
	}
	if err != nil || !on {
		fmt.Fprintln(a.Stdout, "scheduler=off")
		fmt.Fprintln(a.Stdout, "scheduler_healthy=off")
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
		fmt.Fprintln(a.Stdout, "scheduler_healthy=no")
		fmt.Fprintln(a.Stdout, "data_healthy=unknown")
		fmt.Fprintln(a.Stdout, "healthy=no")
		return ExitFail
	}
	fmt.Fprintln(a.Stdout, "scheduler_healthy=yes")
	data, healthy, fail := a.refreshDataHealth(home)
	fmt.Fprintf(a.Stdout, "data_healthy=%s\nhealthy=%s\n", data, healthy)
	if fail {
		return ExitFail
	}
	return ExitOK
}

// refreshDataHealth reads the local checkpoint only. A live scheduler is not
// evidence that collection or publication succeeded. In-budget retries stay
// exit 0 so the hourly job does not flap, and they still print healthy=no.
func (a *App) refreshDataHealth(home adapter.Home) (data, healthy string, fail bool) {
	now := a.Now()
	st, err := publicprofile.LoadRefreshState(publicprofile.RefreshStatePath(home))
	if err != nil {
		return "unknown", "no", false
	}
	schedule, _, schedErr := a.refreshSchedule(home, now)
	if schedErr != nil {
		return "unknown", "no", true
	}
	degraded := false
	if st.Collection != nil {
		_, ok := st.Collection.At(now)
		degraded = !ok
	}
	age := time.Duration(0)
	if !st.CheckedAt.IsZero() {
		age = now.Sub(st.CheckedAt)
	}
	switch st.Code {
	case "", publicprofile.CodeVerified, publicprofile.CodePublished, publicprofile.CodeDateRollover,
		publicprofile.CodeSkippedCooldown, publicprofile.CodeSkippedDelta, publicprofile.CodeOffline:
		if st.Code == "" || degraded {
			if st.Code == "" {
				return "unknown", "no", false
			}
			return "no", "no", true
		}
		return "yes", "yes", false
	case publicprofile.CodeWillRetry, publicprofile.CodePendingGitHub, publicprofile.CodeBusy:
		if schedule.Failures >= 3 || age > 4*time.Hour || degraded && age > 4*time.Hour {
			return "no", "no", true
		}
		return "recovering", "no", false
	default:
		return "no", "no", true
	}
}

// repairProfileRefreshAgents bootstraps a job only when it is missing or its
// program path changed. A running job with the current path is left alone.
func (a *App) repairProfileRefreshAgents(program string) error {
	home, err := a.userHome()
	if err != nil || home == "" {
		return errors.New("profile refresh agent is incomplete")
	}
	watchPlist, err := a.profileRefreshPlistPath(profileRefreshLabel)
	if err != nil {
		return err
	}
	healthPlist, err := a.profileRefreshPlistPath(profileRefreshHealthLabel)
	if err != nil {
		return err
	}
	watchBody, err := profileRefreshPlist(program, home)
	if err != nil {
		return err
	}
	healthLog := filepath.Join(home, "Library", "Logs", "wheretoken", "profile-refresh-health.log")
	if err := os.MkdirAll(filepath.Dir(healthLog), 0o755); err != nil {
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
	// The hourly repair is one-shot and normally has no PID between runs.
	return err != nil || (label == profileRefreshLabel && !launchctlHasPID(out))
}

func (a *App) refreshPlistState(enabled bool, program string) string {
	if a.GOOS != "darwin" {
		return "unsupported"
	}
	plistPath, err := a.profileRefreshPlistPath(profileRefreshLabel)
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
