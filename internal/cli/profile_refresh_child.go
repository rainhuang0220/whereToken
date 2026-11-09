package cli

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"time"

	"github.com/rainhuang0220/whereToken/internal/adapter"
	"github.com/rainhuang0220/whereToken/internal/publicprofile"
)

var profileRefreshRunTimeout = 5 * time.Minute

const profileRefreshChildEnv = "WHERETOKEN_PROFILE_REFRESH_CHILD"

// runProfileRefreshChild bounds the complete automatic scan and upload. A
// goroutine timeout cannot safely cancel an adapter or release its scan lock;
// the watch instead owns one one-shot process and waits for it to exit. The
// kernel releases that child's file lock on exit. It never signals an existing
// process, a process group, or a service. Manual refresh remains in-process.
func (a *App) runProfileRefreshChild(ctx context.Context, flags Flags, home adapter.Home) (retry bool) {
	program, err := a.refreshProgramPath()
	if err != nil {
		a.noteRefresh(home, publicprofile.PhaseFailed, publicprofile.CodeWillRetry)
		return true
	}
	args := []string{"profile", "refresh", "--quiet"}
	if flags.Home != "" {
		args = append(args, "--home", flags.Home)
	}
	if flags.Offline {
		args = append(args, "--offline")
	}
	if flags.ASCII {
		args = append(args, "--ascii")
	}
	ctx, cancel := context.WithTimeout(ctx, profileRefreshRunTimeout)
	defer cancel()
	if a.RunRefreshChild != nil {
		err = a.RunRefreshChild(ctx, program, args)
	} else {
		err = a.execProfileRefreshChild(ctx, program, args)
	}
	if ctx.Err() != nil {
		a.noteRefresh(home, publicprofile.PhaseFailed, publicprofile.CodeWillRetry)
		if !errors.Is(ctx.Err(), context.Canceled) {
			a.refreshFailed()
		}
		// Replacing a watch can cancel a partially completed scan. Keep
		// its bounded retry reservation; cancellation is not completion.
		return true
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && exitErr.ExitCode() == ExitUsage {
		// A rejected upload is permanent for these bytes. It must not enter
		// the transient-error retry loop. Only owned children use this exit code.
		return false
	}
	if err != nil {
		a.noteRefresh(home, publicprofile.PhaseFailed, publicprofile.CodeWillRetry)
		return true
	}
	st, err := publicprofile.LoadRefreshState(publicprofile.RefreshStatePath(home))
	return err == nil && (st.Code == publicprofile.CodeBusy || st.Code == publicprofile.CodeWillRetry)
}

func (a *App) execProfileRefreshChild(ctx context.Context, program string, args []string) error {
	cmd := exec.CommandContext(ctx, program, args...)
	cmd.Env = append(os.Environ(), profileRefreshChildEnv+"=1")
	cmd.Stdout, cmd.Stderr = a.Stdout, a.Stderr
	cmd.WaitDelay = time.Second
	return cmd.Run()
}
