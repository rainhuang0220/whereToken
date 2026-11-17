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

type profileRefreshOutcome struct {
	phase  string
	code   string
	reason string
	retry  bool
}

// runProfileRefreshChild bounds the complete automatic scan and upload. A
// goroutine timeout cannot safely cancel an adapter or release its scan lock;
// the watch instead owns one one-shot process and waits for it to exit. The
// kernel releases that child's file lock on exit. It never signals an existing
// process, a process group, or a service. Manual refresh remains in-process.
func (a *App) runProfileRefreshChild(ctx context.Context, flags Flags, home adapter.Home) profileRefreshOutcome {
	failure := func(reason string) profileRefreshOutcome {
		return profileRefreshOutcome{phase: publicprofile.PhaseFailed, code: publicprofile.CodeWillRetry, reason: reason, retry: true}
	}
	program, err := a.refreshProgramPath()
	if err != nil {
		a.noteRefresh(home, publicprofile.PhaseFailed, publicprofile.CodeWillRetry)
		return failure("BINARY_UNAVAILABLE")
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
	path := publicprofile.RefreshStatePath(home)
	before := refreshResultFileInfo(path)
	start := a.Now().Truncate(time.Second)
	ctx, cancel := context.WithTimeout(ctx, profileRefreshRunTimeout)
	defer cancel()
	if a.RunRefreshChild != nil {
		err = a.RunRefreshChild(ctx, program, args)
	} else {
		err = a.execProfileRefreshChild(ctx, program, args, !flags.Quiet)
	}
	if ctx.Err() != nil {
		a.noteRefresh(home, publicprofile.PhaseFailed, publicprofile.CodeWillRetry)
		if errors.Is(ctx.Err(), context.Canceled) {
			return failure("CANCELED")
		}
		return failure("TIMED_OUT")
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && exitErr.ExitCode() == ExitUsage {
		// A rejected upload is permanent for these bytes. It must not enter
		// the transient-error retry loop. Only owned children use this exit code.
		return profileRefreshOutcome{phase: publicprofile.PhaseFailed, reason: "PUBLISH_REJECTED"}
	}
	st, stateErr := publicprofile.LoadRefreshState(path)
	after := refreshResultFileInfo(path)
	freshResult := stateErr == nil && after != nil && !st.CheckedAt.Before(start) &&
		(before == nil || !os.SameFile(before, after))
	if !freshResult {
		a.noteRefresh(home, publicprofile.PhaseFailed, publicprofile.CodeWillRetry)
		if err != nil {
			return failure("PROCESS_FAILED")
		}
		return failure("RESULT_MISSING")
	}
	outcome := profileRefreshOutcome{phase: st.Phase, code: st.Code}
	switch st.Code {
	case publicprofile.CodeBusy:
		outcome.reason, outcome.retry = "LOCK_BUSY", true
	case publicprofile.CodeWillRetry:
		outcome.reason, outcome.retry = "REFRESH_RETRY", true
		if st.Publication.Category != "" {
			outcome.reason = st.Publication.Category
		}
	default:
		if err != nil {
			outcome.reason, outcome.retry = "PROCESS_FAILED", true
		} else if category := st.Collection.RetryCategory(); category != "" {
			outcome.reason, outcome.retry = category, true
		} else if st.Publication.Category != "" {
			outcome.reason = st.Publication.Category
		} else if st.Collection != nil {
			for _, check := range st.Collection.Sources {
				if check.Category != "" {
					outcome.reason = check.Category
					break
				}
			}
		}
	}
	return outcome
}

func (a *App) execProfileRefreshChild(ctx context.Context, program string, args []string, output bool) error {
	cmd := exec.CommandContext(ctx, program, args...)
	cmd.Env = append(os.Environ(), profileRefreshChildEnv+"=1")
	if output {
		cmd.Stdout, cmd.Stderr = a.Stdout, a.Stderr
	}
	cmd.WaitDelay = time.Second
	return cmd.Run()
}

func refreshResultFileInfo(path string) os.FileInfo {
	file, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer file.Close()
	// Use the open file's identity. On Windows os.Stat may resolve the
	// identity lazily by path, after the worker's atomic replacement.
	st, _ := file.Stat()
	return st
}
