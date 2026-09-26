package publicprofile

import "time"

const (
	// RefreshMinDelta is the smallest all-time raw token movement since the
	// last verified GitHub wall that may publish on the same local day.
	// It is the 0.01 M display step below 1e9. Comparison uses integers.
	RefreshMinDelta int64 = 10_000
	// RefreshCooldown is the minimum absolute gap since the last verified
	// GitHub wall publication. A strictly later owner-local date does not
	// wait for it. time.Time.Sub keeps a DST fall-back from shortening it.
	RefreshCooldown = time.Hour
)

const (
	PhaseIdle      = "idle"
	PhasePublished = "published"
	PhaseSkipped   = "skipped"
	PhaseWaiting   = "waiting"
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
	CodeWaitingCooldown = "WAITING_COOLDOWN"
	CodePendingGitHub   = "PENDING_GITHUB"
	CodeBlockedData     = "BLOCKED_DATA"
	CodeBlockedAuth     = "BLOCKED_AUTH"
	CodeVerified        = "VERIFIED"
)

// Decision is the client upload gate. Publish is true when a sanitized PUT
// should be attempted. It is not a claim that GitHub has verified the wall.
// Action is the shared publication result. Labels are fixed phase text.
type Decision struct {
	Publish bool
	Phase   string
	Label   string
	Code    string
	Action  string
	DueAt   time.Time
}

// RefreshInput is the client view of the shared wall gate. Remote is the
// accepted hosted snapshot. Verified is the GitHub wall. When WallKnown is
// false, an old envelope has no wall object: the accepted snapshot is the
// only watermark the client can see, and the hosted process remains the
// authority once it sends wall. Totals are integer pointers. Nil is
// unavailable and is never treated as zero.
type RefreshInput struct {
	Local           Snapshot
	Remote          *Snapshot
	RemoteUpdatedAt time.Time
	Verified        VerifiedWall
	Pending         PendingIntent
	WallKnown       bool
	Now             time.Time
	Offline         bool
}

// DecideRefresh chooses whether the client should PUT a sanitized snapshot.
// GitHub publication is DecidePublication. A growth that is still inside the
// verified cooldown is uploaded once so the host can publish it later
// without another PUT. A strictly later owner-local date does not wait.
func DecideRefresh(in RefreshInput) Decision {
	if in.Offline {
		if in.Remote != nil && in.Remote.SnapshotID != "" {
			return skipped("离线不覆盖已发布快照", CodeOffline)
		}
		return skipped("离线不发布", CodeOffline)
	}
	verified := in.Verified
	if !in.WallKnown && in.Remote != nil && in.Remote.SnapshotID != "" {
		verified = WallFromSnapshot(*in.Remote, in.RemoteUpdatedAt)
	}
	pub := DecidePublication(PublicationInput{
		Candidate: in.Local,
		Accepted:  in.Remote,
		Verified:  verified,
		Pending:   in.Pending,
		Now:       in.Now,
		Offline:   false,
	})
	return refreshFromPublication(pub, in)
}

func refreshFromPublication(pub PublicationDecision, in RefreshInput) Decision {
	switch pub.Action {
	case ActionPublishNow:
		if pub.Reason == ReasonDate {
			return Decision{Publish: true, Phase: PhasePublished, Label: "日期已更新", Code: CodeDateRollover, Action: pub.Action}
		}
		d := publishedUsage()
		d.Action = pub.Action
		return d
	case ActionWaitUntil:
		already := in.Remote != nil && in.Remote.SnapshotID != "" && in.Remote.SnapshotID == in.Local.SnapshotID
		return Decision{
			Publish: !already,
			Phase:   PhaseWaiting,
			Label:   "间隔未满 1 小时",
			Code:    CodeWaitingCooldown,
			Action:  pub.Action,
			DueAt:   pub.DueAt,
		}
	case ActionRetryPending:
		return Decision{Publish: false, Phase: PhaseWaiting, Label: "主页更新将重试", Code: CodePendingGitHub, Action: pub.Action}
	case ActionBlocked:
		return blockedDecision(pub, in)
	default:
		return noChangeDecision(pub, in)
	}
}

func blockedDecision(pub PublicationDecision, in RefreshInput) Decision {
	switch pub.Reason {
	case ReasonUnavailable:
		if in.Remote != nil && in.Remote.SnapshotID != "" {
			return skipped("用量不可用，保留上次", CodeBlockedData)
		}
		return skipped("用量不可用", CodeBlockedData)
	case ReasonCoverage:
		return skipped("覆盖变弱，保留上次", CodeBlockedData)
	case ReasonNegative:
		return skipped("增量未到 0.01 M", CodeSkippedDelta)
	case ReasonUnknownVerified:
		return skipped("已发布用量未知，保留上次", CodeBlockedData)
	case ReasonCooldown:
		return skipped("间隔未满 1 小时", CodeSkippedCooldown)
	default:
		return skipped("用量不可用，保留上次", CodeBlockedData)
	}
}

func noChangeDecision(pub PublicationDecision, in RefreshInput) Decision {
	switch pub.Reason {
	case ReasonEarlierDate, ReasonDelta, ReasonSame:
		return skipped("增量未到 0.01 M", CodeSkippedDelta)
	default:
		if in.Remote != nil {
			return skipped("增量未到 0.01 M", CodeSkippedDelta)
		}
		return skipped("用量不可用", "")
	}
}

// AllowedRefreshCode is the only set persisted in profile-refresh-state.json.
func AllowedRefreshCode(code string) bool {
	switch code {
	case "", CodePublished, CodeDateRollover, CodeSkippedCooldown, CodeSkippedDelta, CodeWillRetry, CodeNotSignedIn, CodeOffline, CodeBusy, CodeWaitingCooldown, CodePendingGitHub, CodeBlockedData, CodeBlockedAuth, CodeVerified:
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
