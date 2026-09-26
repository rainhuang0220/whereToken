package publicprofile

import (
	"context"
	"encoding/json"
	"errors"
	"time"
)

// RemoteView is the hosted live envelope the refresh gate needs:
// the accepted snapshot, its as_of_date and all-time total, and
// freshness.updated_at. The scan index is not a watermark.
type RemoteView struct {
	Found     bool
	Snapshot  Snapshot
	UpdatedAt time.Time
}

// PutResult is one projection upload. KeptPrevious means the server
// left the last good snapshot in place. Status is the HTTP status.
type PutResult struct {
	Status       int
	KeptPrevious bool
}

// RefreshPublisher uploads one sanitized snapshot. Fetch is the public live
// envelope. Put is PUT /api/v1/sync/public-profile and nothing else.
type RefreshPublisher interface {
	Fetch(ctx context.Context) (RemoteView, error)
	Put(ctx context.Context, body []byte) (PutResult, error)
}

// ErrNotSignedIn means the hosted API rejected the device credential.
// Callers print the existing login hint and do not publish.
var ErrNotSignedIn = errors.New("not signed in")

// ApplyResult is one refresh attempt. Rejected means the server refused
// the body (4xx); the same bytes must not be sent again in a loop.
// Transport means the last good remote snapshot was left in place.
// Already means a follow-up read showed this snapshot is stored, so
// no further PUT is required. Auth means the device is not signed in.
type ApplyResult struct {
	Decision  Decision
	Rejected  bool
	Transport bool
	Already   bool
	Auth      bool
}

// ApplyRefresh fetches the hosted envelope, applies DecideRefresh, and
// PUTs at most once. A transport error reads the envelope again: if that
// snapshot id is already the local one, the attempt counts as published.
func ApplyRefresh(ctx context.Context, local Snapshot, now time.Time, offline bool, pub RefreshPublisher) ApplyResult {
	if ctx.Err() != nil {
		return ApplyResult{Decision: FailedRefresh(), Transport: true}
	}
	if offline {
		return ApplyResult{Decision: DecideRefresh(RefreshInput{Local: local, Now: now, Offline: true})}
	}
	if pub == nil {
		return ApplyResult{Decision: FailedRefresh(), Transport: true}
	}
	view, err := pub.Fetch(ctx)
	if errors.Is(err, ErrNotSignedIn) {
		return ApplyResult{Auth: true}
	}
	if err != nil {
		return ApplyResult{Decision: FailedRefresh(), Transport: true}
	}
	in := RefreshInput{Local: local, Now: now, RemoteUpdatedAt: view.UpdatedAt}
	if view.Found && view.Snapshot.SnapshotID != "" {
		remote := view.Snapshot
		in.Remote = &remote
	}
	d := DecideRefresh(in)
	if !d.Publish {
		return ApplyResult{Decision: d}
	}
	if view.Found && view.Snapshot.SnapshotID != "" && view.Snapshot.SnapshotID == local.SnapshotID {
		return ApplyResult{Decision: d, Already: true}
	}
	if err := Validate(local); err != nil {
		return ApplyResult{Decision: FailedRefresh(), Rejected: true}
	}
	raw, err := Marshal(local)
	if err != nil || Sensitive(string(raw)) {
		return ApplyResult{Decision: FailedRefresh(), Rejected: true}
	}
	res, err := pub.Put(ctx, raw)
	if err != nil || res.Status == 429 || res.Status >= 500 {
		if stored(ctx, pub, local.SnapshotID) {
			return ApplyResult{Decision: d, Already: true}
		}
		return ApplyResult{Decision: FailedRefresh(), Transport: true}
	}
	if res.Status == 401 || res.Status == 403 {
		return ApplyResult{Auth: true}
	}
	if res.Status >= 400 {
		return ApplyResult{Decision: FailedRefresh(), Rejected: true}
	}
	if res.KeptPrevious {
		return ApplyResult{Decision: skipped("覆盖变弱，保留上次", "")}
	}
	return ApplyResult{Decision: d}
}

func stored(ctx context.Context, pub RefreshPublisher, id string) bool {
	if id == "" || ctx.Err() != nil {
		return false
	}
	view, err := pub.Fetch(ctx)
	return err == nil && view.Found && view.Snapshot.SnapshotID == id
}

// ParseLiveEnvelope reads the public profile GET body. Unknown fields are
// ignored. Found is false when the snapshot id is empty.
func ParseLiveEnvelope(raw []byte) (RemoteView, error) {
	var env struct {
		Freshness struct {
			UpdatedAt string `json:"updated_at"`
		} `json:"freshness"`
		DataRevision string          `json:"data_revision"`
		Snapshot     json.RawMessage `json:"snapshot"`
	}
	if err := json.Unmarshal(raw, &env); err != nil {
		return RemoteView{}, err
	}
	var snap Snapshot
	if len(env.Snapshot) > 0 && string(env.Snapshot) != "null" {
		if err := json.Unmarshal(env.Snapshot, &snap); err != nil {
			return RemoteView{}, err
		}
	}
	if snap.SnapshotID == "" {
		snap.SnapshotID = env.DataRevision
	}
	view := RemoteView{Found: snap.SnapshotID != "", Snapshot: snap}
	if env.Freshness.UpdatedAt != "" {
		t, err := time.Parse(time.RFC3339, env.Freshness.UpdatedAt)
		if err != nil {
			return RemoteView{}, err
		}
		view.UpdatedAt = t
	}
	return view, nil
}
