package publicprofile

import "time"

const (
	// RefreshMinDelta is the smallest all-time token movement that may
	// upload a new hosted projection on the same local calendar day.
	// It is the 0.01 M display step below 1e9, compared as raw tokens.
	// Larger totals display more coarsely; the gate stays 10_000.
	RefreshMinDelta int64 = 10_000
	// RefreshCooldown is the minimum gap between accepted hosted uploads.
	// A local-date change does not skip it. Compare instants with
	// time.Time.Sub so a DST fall-back cannot shorten the wait.
	RefreshCooldown = time.Hour
)

const (
	PhaseIdle      = "idle"
	PhasePublished = "published"
	PhaseSkipped   = "skipped"
)

const (
	CodePublished       = "PUBLISHED"
	CodeDateRollover    = "DATE_ROLLOVER"
	CodeSkippedCooldown = "SKIPPED_COOLDOWN"
	CodeSkippedDelta    = "SKIPPED_DELTA"
	CodeWillRetry       = "WILL_RETRY"
	CodeNotSignedIn     = "NOT_SIGNED_IN"
	CodeOffline         = "OFFLINE"
	CodeBusy            = "BUSY"
)

// Decision is the refresh gate. Publish is true only when a sanitized PUT
// should be attempted. Labels are the fixed bilingual phase text.
type Decision struct {
	Publish bool
	Phase   string
	Label   string
	Code    string
}

// RefreshInput is the pure refresh gate. Remote is nil when the hosted
// envelope has no accepted snapshot. Totals are integer pointers: nil is
// unavailable and is never treated as zero. Offline refuses a publish
// without reading or inventing a total.
type RefreshInput struct {
	Local           Snapshot
	Remote          *Snapshot
	RemoteUpdatedAt time.Time
	Now             time.Time
	Offline         bool
}

// DecideRefresh chooses whether a sanitized snapshot may be PUT.
// The hosted envelope is the watermark. This function does not read a clock
// except the instants the caller passes in.
func DecideRefresh(in RefreshInput) Decision {
	if in.Offline {
		if in.Remote != nil && in.Remote.SnapshotID != "" {
			return skipped("离线不覆盖已发布快照", "")
		}
		return skipped("离线不发布", "")
	}
	localTotal := in.Local.Periods.All.Totals.Total.Value
	noRemote := in.Remote == nil || in.Remote.SnapshotID == ""
	if noRemote {
		if in.Local.DataStatus == StatusUnavailable || localTotal == nil {
			return skipped("用量不可用", "")
		}
		// First accepted snapshot, including a small available total.
		// Partial coverage with a real total is still a first publish;
		// unavailable and a nil total are not.
		if in.Local.DataStatus == StatusAvailable || in.Local.DataStatus == StatusPartial {
			return publishedUsage()
		}
		return skipped("用量不可用", "")
	}
	if in.Local.DataStatus == StatusUnavailable || localTotal == nil {
		if in.Remote != nil {
			return skipped("用量不可用，保留上次", "")
		}
		return skipped("用量不可用", "")
	}
	if !ShouldReplaceProjection(*in.Remote, in.Local) {
		return skipped("覆盖变弱，保留上次", "")
	}
	prevTotal := in.Remote.Periods.All.Totals.Total.Value
	// A nil remote total is not zero, so it cannot prove the new total is
	// at least as large. A smaller total never publishes, including when
	// the local calendar date changed or the scan recorded errors.
	if prevTotal == nil || *localTotal < *prevTotal {
		return skipped("增量未到 0.01 M", CodeSkippedDelta)
	}
	// Only a strictly later local calendar day is a date opportunity.
	// An earlier day (a timezone move west) and an equal day are not.
	dateRollover := laterLocalDate(in.Local.AsOfDate, in.Remote.AsOfDate)
	if !dateRollover && *localTotal-*prevTotal < RefreshMinDelta {
		return skipped("增量未到 0.01 M", CodeSkippedDelta)
	}
	// A missing freshness.updated_at is not "cooldown elapsed".
	// time.Time.Sub is absolute, so a DST fall-back cannot shorten the hour.
	if in.RemoteUpdatedAt.IsZero() || in.Now.Sub(in.RemoteUpdatedAt) < RefreshCooldown {
		return skipped("间隔未满 1 小时", CodeSkippedCooldown)
	}
	if dateRollover {
		return Decision{Publish: true, Phase: PhasePublished, Label: "日期已更新", Code: CodeDateRollover}
	}
	return publishedUsage()
}

// AllowedRefreshCode is the only set persisted in profile-refresh-state.json.
func AllowedRefreshCode(code string) bool {
	switch code {
	case "", CodePublished, CodeDateRollover, CodeSkippedCooldown, CodeSkippedDelta, CodeWillRetry, CodeNotSignedIn, CodeOffline, CodeBusy:
		return true
	default:
		return false
	}
}

func publishedUsage() Decision {
	return Decision{Publish: true, Phase: PhasePublished, Label: "用量已更新", Code: CodePublished}
}

func skipped(label, code string) Decision {
	return Decision{Phase: PhaseSkipped, Label: label, Code: code}
}

// FailedRefresh is the fixed failure phase. The caller retries a transport
// error on a later invocation and does not loop a rejected body.
func FailedRefresh() Decision {
	return Decision{Phase: PhaseFailed, Label: "更新失败，下次再试", Code: CodeWillRetry}
}
