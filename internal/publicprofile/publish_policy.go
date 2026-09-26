package publicprofile

import "time"

// Publication actions are the only outputs of DecidePublication.
const (
	ActionPublishNow   = "PUBLISH_NOW"
	ActionWaitUntil    = "WAIT_UNTIL"
	ActionNoChange     = "NO_CHANGE"
	ActionRetryPending = "RETRY_PENDING"
	ActionBlocked      = "BLOCKED"
)

// Publication reasons are stable machine labels. They are not log payloads.
const (
	ReasonUsage           = "usage"
	ReasonDate            = "date"
	ReasonTheme           = "theme"
	ReasonFirst           = "first"
	ReasonRetry           = "retry"
	ReasonCooldown        = "cooldown"
	ReasonDelta           = "delta"
	ReasonCoverage        = "coverage"
	ReasonUnavailable     = "unavailable"
	ReasonEarlierDate     = "earlier_date"
	ReasonUnknownVerified = "unknown_verified"
	ReasonNegative        = "negative"
	ReasonOffline         = "offline"
	ReasonSame            = "same"
)

// VerifiedWall is the last GitHub wall publication that was independently
// checked against the remote README and both SVGs. Proven means a snapshot
// id was verified. Total and AsOfDate stay unset when that identity cannot
// be tied to an exact raw total or calendar date. A nil total is unknown,
// never zero.
type VerifiedWall struct {
	Proven        bool
	SnapshotID    string
	Total         *int64
	AsOfDate      string
	PublishedAt   time.Time
	CacheKey      string
	AssetRevision string
}

// PendingIntent is the one desired GitHub publication for an owner.
// DueAt is the absolute instant the publication may run. Failed means a
// previous attempt did not verify, so a later snapshot coalesces onto this
// intent instead of starting a second commit.
type PendingIntent struct {
	SnapshotID string
	DueAt      time.Time
	Reason     string
	Failed     bool
}

// PublicationInput is the pure wall gate. Candidate is the snapshot that
// would be written. Accepted is the latest hosted projection, used for
// coverage. Verified is the GitHub watermark. Now and PublishedAt are
// absolute instants. AsOfDate values are owner-local calendar dates.
type PublicationInput struct {
	Candidate   Snapshot
	Accepted    *Snapshot
	Verified    VerifiedWall
	Pending     PendingIntent
	Now         time.Time
	ThemeChange bool
	Offline     bool
	// FromWorker is the hosted maintenance path. A failed intent then
	// publishes the latest candidate. A coalescing client PUT leaves the
	// same intent as RETRY_PENDING so intermediate snapshots do not each
	// open a GitHub commit.
	FromWorker bool
}

// PublicationDecision is one policy result. DueAt is set for WAIT_UNTIL.
type PublicationDecision struct {
	Action string
	DueAt  time.Time
	Reason string
	Code   string
}

// DecidePublication chooses whether the candidate may update the GitHub
// activity wall. Daily rollover wins over the usage cooldown. Usage growth
// is a raw integer delta from the verified total. Theme publication is
// explicit and immediate. Missing totals stay missing.
func DecidePublication(in PublicationInput) PublicationDecision {
	if in.Offline {
		return PublicationDecision{Action: ActionBlocked, Reason: ReasonOffline, Code: CodeOffline}
	}
	total := tokenValue(in.Candidate)
	if (in.Candidate.DataStatus != StatusAvailable && in.Candidate.DataStatus != StatusPartial) || total == nil {
		return PublicationDecision{Action: ActionBlocked, Reason: ReasonUnavailable, Code: CodeBlockedData}
	}
	if in.Accepted != nil && in.Accepted.SnapshotID != "" && in.Accepted.SnapshotID != in.Candidate.SnapshotID {
		if !ShouldReplaceProjection(*in.Accepted, in.Candidate) {
			return PublicationDecision{Action: ActionBlocked, Reason: ReasonCoverage, Code: CodeBlockedData}
		}
	}
	if in.Verified.Proven && in.Verified.Total != nil && *total < *in.Verified.Total {
		return PublicationDecision{Action: ActionBlocked, Reason: ReasonNegative, Code: CodeSkippedDelta}
	}
	if in.Verified.Proven && validLocalDate(in.Verified.AsOfDate) && validLocalDate(in.Candidate.AsOfDate) && earlierLocalDate(in.Candidate.AsOfDate, in.Verified.AsOfDate) {
		return PublicationDecision{Action: ActionNoChange, Reason: ReasonEarlierDate, Code: CodeSkippedDelta}
	}
	if in.ThemeChange {
		return PublicationDecision{Action: ActionPublishNow, Reason: ReasonTheme, Code: CodePublished}
	}
	if dateRollover(in.Verified, in.Candidate) {
		if in.Verified.Total == nil {
			return PublicationDecision{Action: ActionBlocked, Reason: ReasonUnknownVerified, Code: CodeBlockedData}
		}
		return PublicationDecision{Action: ActionPublishNow, Reason: ReasonDate, Code: CodeDateRollover}
	}
	if retryable(in) {
		if in.FromWorker {
			return PublicationDecision{Action: ActionPublishNow, Reason: ReasonRetry, Code: CodePendingGitHub}
		}
		return PublicationDecision{Action: ActionRetryPending, Reason: ReasonRetry, Code: CodePendingGitHub}
	}
	if !in.Verified.Proven || in.Verified.SnapshotID == "" {
		return PublicationDecision{Action: ActionPublishNow, Reason: ReasonFirst, Code: CodePublished}
	}
	if in.Candidate.SnapshotID != "" && in.Candidate.SnapshotID == in.Verified.SnapshotID {
		return PublicationDecision{Action: ActionNoChange, Reason: ReasonSame, Code: CodeSkippedDelta}
	}
	if in.Verified.Total == nil {
		return PublicationDecision{Action: ActionBlocked, Reason: ReasonUnknownVerified, Code: CodeBlockedData}
	}
	delta := *total - *in.Verified.Total
	if delta < RefreshMinDelta {
		return PublicationDecision{Action: ActionNoChange, Reason: ReasonDelta, Code: CodeSkippedDelta}
	}
	if in.Verified.PublishedAt.IsZero() {
		if !in.Pending.DueAt.IsZero() && !in.Now.Before(in.Pending.DueAt) {
			return PublicationDecision{Action: ActionPublishNow, Reason: ReasonUsage, Code: CodePublished}
		}
		return PublicationDecision{Action: ActionBlocked, Reason: ReasonCooldown, Code: CodeSkippedCooldown}
	}
	due := in.Verified.PublishedAt.Add(RefreshCooldown)
	if in.Now.Sub(in.Verified.PublishedAt) < RefreshCooldown {
		if !in.Pending.DueAt.IsZero() && !in.Now.Before(in.Pending.DueAt) {
			return PublicationDecision{Action: ActionPublishNow, Reason: ReasonUsage, Code: CodePublished}
		}
		return PublicationDecision{Action: ActionWaitUntil, DueAt: due, Reason: ReasonCooldown, Code: CodeWaitingCooldown}
	}
	return PublicationDecision{Action: ActionPublishNow, Reason: ReasonUsage, Code: CodePublished}
}

func dateRollover(v VerifiedWall, cand Snapshot) bool {
	return v.Proven && laterLocalDate(cand.AsOfDate, v.AsOfDate)
}

func retryable(in PublicationInput) bool {
	if !in.Pending.Failed || in.Pending.SnapshotID == "" {
		return false
	}
	if in.Verified.Proven && in.Candidate.SnapshotID != "" && in.Candidate.SnapshotID == in.Verified.SnapshotID {
		return false
	}
	return true
}

func tokenValue(s Snapshot) *int64 {
	return s.Periods.All.Totals.Total.Value
}

func validLocalDate(s string) bool {
	_, err := time.Parse("2006-01-02", s)
	return err == nil
}

func earlierLocalDate(next, prev string) bool {
	n, errN := time.Parse("2006-01-02", next)
	p, errP := time.Parse("2006-01-02", prev)
	if errN != nil || errP != nil {
		return false
	}
	return n.Before(p)
}

// WallFromSnapshot builds a verified wall when the snapshot id is the one
// that was published. The caller must already have proved that identity.
func WallFromSnapshot(s Snapshot, publishedAt time.Time) VerifiedWall {
	return VerifiedWall{
		Proven:      s.SnapshotID != "",
		SnapshotID:  s.SnapshotID,
		Total:       tokenValue(s),
		AsOfDate:    s.AsOfDate,
		PublishedAt: publishedAt,
	}
}
