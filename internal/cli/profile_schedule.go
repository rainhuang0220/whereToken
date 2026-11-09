package cli

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/rainhuang0220/whereToken/internal/adapter"
	"github.com/rainhuang0220/whereToken/internal/publicprofile"
)

// The attempt is reserved before starting its child. Reexec and launchd repair
// therefore cannot reset this slot's budget or start an immediate second scan.
// Only calendar times, a count and a completion flag are persisted here.
type profileRefreshSchedule struct {
	Slot     time.Time `json:"slot"`
	Attempts int       `json:"attempts"`
	RetryAt  time.Time `json:"retry_at,omitempty"`
	Done     bool      `json:"done"`
}

var profileRefreshBackoff = [...]time.Duration{30 * time.Second, 2 * time.Minute}

func profileRefreshSchedulePath(home adapter.Home) string {
	return filepath.Join(filepath.Dir(publicprofile.RefreshConfigPath(home)), "profile-refresh-schedule.json")
}

func (a *App) refreshSchedule(home adapter.Home, now time.Time) (profileRefreshSchedule, time.Time, error) {
	slot := publicprofile.ScheduledRefreshSlot(now, a.Loc)
	st := profileRefreshSchedule{Slot: slot}
	raw, err := os.ReadFile(profileRefreshSchedulePath(home))
	if err == nil {
		if err = json.Unmarshal(raw, &st); err != nil {
			return st, time.Time{}, errors.New("refresh schedule unavailable")
		}
		if st.Slot.IsZero() || st.Attempts < 0 || st.Attempts > 3 {
			return st, time.Time{}, errors.New("refresh schedule unavailable")
		}
		if st.Slot.Before(slot) {
			st = profileRefreshSchedule{Slot: slot}
		}
	} else if !os.IsNotExist(err) {
		return st, time.Time{}, errors.New("refresh schedule unavailable")
	} else {
		// Migrate the earlier result-only checkpoint. A BUSY or transient
		// failure is an attempted check, never a completed scan for this slot.
		prev, e := publicprofile.LoadRefreshState(publicprofile.RefreshStatePath(home))
		if e == nil && !prev.CheckedAt.Before(slot) {
			if prev.Code == publicprofile.CodeBusy || prev.Code == publicprofile.CodeWillRetry {
				st.Attempts = 1
				st.RetryAt = prev.CheckedAt.Add(profileRefreshBackoff[0])
			} else {
				st.Done = true
			}
		}
	}
	next := publicprofile.NextScheduledRefreshWake(now, a.Loc)
	if st.Slot.After(slot) || st.Done || st.Attempts >= 3 {
		return st, next, nil
	}
	if st.RetryAt.After(now) {
		if st.RetryAt.Before(next) {
			next = st.RetryAt
		}
		return st, next, nil
	}
	return st, now, nil
}

func saveProfileRefreshSchedule(home adapter.Home, st profileRefreshSchedule) error {
	path := profileRefreshSchedulePath(home)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	raw, err := json.Marshal(st)
	if err != nil {
		return err
	}
	return writeAtomicFile(path, append(raw, '\n'), 0o600)
}

func (a *App) attemptScheduledProfileRefresh(ctx context.Context, flags Flags, home adapter.Home, st profileRefreshSchedule) error {
	st.Attempts++
	st.RetryAt = a.Now().Add(profileRefreshRunTimeout)
	if st.Attempts <= len(profileRefreshBackoff) {
		st.RetryAt = st.RetryAt.Add(profileRefreshBackoff[st.Attempts-1])
	}
	// Reserve before spawning: even a crash cannot replenish the attempt budget.
	if err := saveProfileRefreshSchedule(home, st); err != nil {
		return err
	}
	retry := a.runProfileRefreshChild(ctx, flags, home)
	st.Done = !retry
	if retry && st.Attempts <= len(profileRefreshBackoff) {
		st.RetryAt = a.Now().Add(profileRefreshBackoff[st.Attempts-1])
	}
	return saveProfileRefreshSchedule(home, st)
}
