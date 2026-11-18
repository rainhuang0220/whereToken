package cli

import (
	"context"
	"encoding/json"
	"errors"
	"math/rand/v2"
	"os"
	"path/filepath"
	"time"

	"github.com/rainhuang0220/whereToken/internal/adapter"
	"github.com/rainhuang0220/whereToken/internal/publicprofile"
)

// The reservation is durable before starting a child. A restart or calendar
// slot change keeps the deadline and escalation for an unresolved outage.
type profileRefreshSchedule struct {
	SchemaVersion      int       `json:"schema_version,omitempty"`
	Slot               time.Time `json:"slot"`
	Attempts           int       `json:"attempts"`
	RetryAt            time.Time `json:"retry_at,omitzero"`
	Done               bool      `json:"done"`
	Failures           int       `json:"failures,omitempty"`
	LastAttemptAt      time.Time `json:"last_attempt_at,omitzero"`
	LastDurationMS     int64     `json:"last_duration_ms"`
	LastPhase          string    `json:"last_phase,omitempty"`
	LastCode           string    `json:"last_code,omitempty"`
	CursorAuthNoticeAt time.Time `json:"cursor_auth_notice_at,omitzero"`
	LastRetryReason    string    `json:"last_retry_reason,omitempty"`
}

var profileRefreshBackoff = [...]time.Duration{30 * time.Second, 2 * time.Minute, 2 * time.Hour, 4 * time.Hour}

// v0.7.19 rejects a schedule whose attempts field is greater than 3 and
// then exits. KeepAlive would restart that exit every minute. The candidate
// keeps the outage in failures and never stores attempts above this cap.
const legacyProfileRefreshAttempts = 3

func profileRefreshSchedulePath(home adapter.Home) string {
	return filepath.Join(filepath.Dir(publicprofile.RefreshConfigPath(home)), "profile-refresh-schedule.json")
}

func (a *App) refreshSchedule(home adapter.Home, now time.Time) (profileRefreshSchedule, time.Time, error) {
	slot := publicprofile.ScheduledRefreshSlot(now, a.Loc)
	st := profileRefreshSchedule{Slot: slot}
	raw, err := os.ReadFile(profileRefreshSchedulePath(home))
	switch {
	case err == nil:
		if err := json.Unmarshal(raw, &st); err != nil || st.Slot.IsZero() || st.Attempts < 0 ||
			st.Failures < 0 || st.Failures > len(profileRefreshBackoff) {
			return st, time.Time{}, errors.New("refresh schedule unavailable")
		}
		if st.Attempts > legacyProfileRefreshAttempts {
			if err := backupProfileRefreshSchedule(home, raw); err != nil {
				return st, time.Time{}, err
			}
			if st.Failures == 0 {
				st.Failures = min(st.Attempts, len(profileRefreshBackoff))
			}
			st.Attempts = legacyProfileRefreshAttempts
			if err := saveProfileRefreshSchedule(home, st); err != nil {
				return st, time.Time{}, err
			}
		}
		if !st.Done && st.Failures == 0 && st.Attempts > 0 {
			// The previous schedule persisted only the per-slot count. Its
			// final reservation has no outage delay, so extend it once.
			st.Failures = min(st.Attempts, 3)
			if st.Attempts >= 3 {
				st.RetryAt = st.RetryAt.Add(profileRefreshBackoff[2])
			}
		}
		if st.Slot.Before(slot) {
			if st.Done {
				st.RetryAt = time.Time{}
			}
			st.Slot, st.Attempts, st.Done = slot, 0, false
		}
	case os.IsNotExist(err):
		// Migrate the earlier result-only checkpoint.
		prev, err := publicprofile.LoadRefreshState(publicprofile.RefreshStatePath(home))
		if err == nil && !prev.CheckedAt.Before(slot) {
			if prev.Code == publicprofile.CodeBusy || prev.Code == publicprofile.CodeWillRetry {
				st.Attempts, st.Failures = 1, 1
				st.RetryAt = prev.CheckedAt.Add(profileRefreshBackoff[0])
			} else {
				st.Done = true
			}
		}
	default:
		return st, time.Time{}, errors.New("refresh schedule unavailable")
	}
	if st.Slot.After(slot) || st.Done {
		return st, publicprofile.NextScheduledRefreshWake(now, a.Loc), nil
	}
	if st.RetryAt.After(now) {
		return st, st.RetryAt, nil
	}
	return st, now, nil
}

func saveProfileRefreshSchedule(home adapter.Home, st profileRefreshSchedule) error {
	if st.Attempts > legacyProfileRefreshAttempts {
		if st.Failures == 0 {
			st.Failures = min(st.Attempts, len(profileRefreshBackoff))
		}
		st.Attempts = legacyProfileRefreshAttempts
	}
	st.SchemaVersion = 2
	path := profileRefreshSchedulePath(home)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	raw, err := json.Marshal(st)
	if err != nil {
		return err
	}
	if err := writeAtomicFile(path, append(raw, '\n'), 0o600); err != nil {
		return err
	}
	return writeRefreshEvidence(home, st)
}

func profileRefreshEvidencePath(home adapter.Home) string {
	return filepath.Join(filepath.Dir(profileRefreshSchedulePath(home)), "profile-refresh-evidence.json")
}

func backupProfileRefreshSchedule(home adapter.Home, raw []byte) error {
	path := profileRefreshSchedulePath(home) + ".pre-downgrade"
	if _, err := os.Stat(path); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	return writeAtomicFile(path, raw, 0o600)
}

func writeRefreshEvidence(home adapter.Home, st profileRefreshSchedule) error {
	raw, err := json.Marshal(st)
	if err != nil {
		return err
	}
	return writeAtomicFile(profileRefreshEvidencePath(home), append(raw, '\n'), 0o600)
}

func (a *App) attemptScheduledProfileRefresh(ctx context.Context, flags Flags, home adapter.Home, st profileRefreshSchedule) error {
	start := a.Now()
	timerStart := time.Now()
	st.Attempts++
	st.Failures = min(st.Failures+1, len(profileRefreshBackoff))
	delay := profileRefreshBackoff[st.Failures-1]
	delay += time.Duration(rand.Int64N(int64(delay/10) + 1))
	st.RetryAt = start.Add(profileRefreshRunTimeout + delay)
	st.LastAttemptAt = start
	st.LastDurationMS = 0
	st.LastPhase = "started"
	st.LastCode = ""
	st.LastRetryReason = "INTERRUPTED"
	if err := saveProfileRefreshSchedule(home, st); err != nil {
		return err
	}
	outcome := a.runProfileRefreshChild(ctx, flags, home)
	st.LastPhase = outcome.phase
	st.LastCode = outcome.code
	st.LastDurationMS = time.Since(timerStart).Milliseconds()
	st.LastRetryReason = outcome.reason
	st.Done = !outcome.retry
	if outcome.retry {
		st.RetryAt = a.Now().Add(delay)
	} else {
		st.Failures = 0
		st.RetryAt = time.Time{}
	}
	if err := saveProfileRefreshSchedule(home, st); err != nil {
		return err
	}
	appendProfileRefreshScheduleLog(home, st)
	now := a.Now()
	if a.GOOS == "darwin" && outcome.cursorAuthDue &&
		(st.CursorAuthNoticeAt.IsZero() || now.Sub(st.CursorAuthNoticeAt) >= publicprofile.CollectionMaxAge) {
		if a.notifyCursorAuth(ctx) == nil {
			st.CursorAuthNoticeAt = now
			return saveProfileRefreshSchedule(home, st)
		}
	}
	return nil
}

const maxProfileRefreshLogBytes = 64 << 10

func appendProfileRefreshScheduleLog(home adapter.Home, st profileRefreshSchedule) {
	entry := struct {
		At       time.Time `json:"attempted_at"`
		Duration int64     `json:"duration_ms"`
		Phase    string    `json:"phase"`
		Code     string    `json:"code,omitempty"`
		Reason   string    `json:"retry_reason,omitempty"`
		Attempt  int       `json:"attempt"`
		RetryAt  time.Time `json:"retry_at,omitzero"`
	}{st.LastAttemptAt, st.LastDurationMS, st.LastPhase, st.LastCode, st.LastRetryReason, st.Attempts, st.RetryAt}
	line, err := json.Marshal(entry)
	if err != nil {
		return
	}
	line = append(line, '\n')
	path := filepath.Join(filepath.Dir(profileRefreshSchedulePath(home)), "profile-refresh-schedule.log")
	if fi, err := os.Lstat(path); err == nil {
		if !fi.Mode().IsRegular() {
			return
		}
		if fi.Size() > maxProfileRefreshLogBytes {
			if err := os.Truncate(path, 0); err != nil {
				return
			}
		} else if fi.Size()+int64(len(line)) > maxProfileRefreshLogBytes {
			if err := os.Rename(path, path+".1"); err != nil {
				return
			}
		}
	} else if !os.IsNotExist(err) {
		return
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o600)
	if err == nil {
		_, _ = file.Write(line)
		_ = file.Close()
	}
}
