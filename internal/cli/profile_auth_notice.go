package cli

import (
	"context"
	"os/exec"
	"time"

	"github.com/rainhuang0220/whereToken/internal/publicprofile"
)

func cursorAuthReminderDue(collection *publicprofile.CollectionEvidence, now time.Time) bool {
	if collection == nil {
		return false
	}
	for _, check := range collection.Sources {
		if check.Source == "cursor" && check.Status == publicprofile.SourceAuthRequired {
			return check.LastObservationAt == nil || check.LastObservationAt.After(now) ||
				now.Sub(*check.LastObservationAt) >= publicprofile.CollectionMaxAge
		}
	}
	return false
}

const cursorAuthNotification = `display notification "Cursor usage needs attention. Open Terminal and run wheretoken cursor connect." with title "whereToken"`

func (a *App) notifyCursorAuth(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if a.RunCmd != nil {
		return a.RunCmd("/usr/bin/osascript", "-e", cursorAuthNotification)
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "/usr/bin/osascript", "-e", cursorAuthNotification)
	cmd.WaitDelay = time.Second
	return cmd.Run()
}
