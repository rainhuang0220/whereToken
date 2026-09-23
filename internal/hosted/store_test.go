package hosted

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"sync"
	"testing"
	"time"

	"github.com/rainhuang0220/whereToken/internal/syncagg"
)

// TestMigrateAddsPairDeviceLinkIdempotently proves that Migrate can run
// repeatedly against a database that already has the pairing_challenges ->
// devices link (the normal case: every process calls Migrate at startup)
// without erroring on the already-present column or unique index.
func TestMigrateAddsPairDeviceLinkIdempotently(t *testing.T) {
	st := readyStore(t)
	ctx := context.Background()
	if err := st.Migrate(ctx); err != nil {
		t.Fatalf("second Migrate call: %v", err)
	}
	if err := st.Migrate(ctx); err != nil {
		t.Fatalf("third Migrate call: %v", err)
	}
	var columns int
	if err := st.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='pairing_challenges' AND COLUMN_NAME='device_id'`).Scan(&columns); err != nil {
		t.Fatal(err)
	}
	if columns != 1 {
		t.Fatalf("device_id column count = %d", columns)
	}
	var indexes int
	if err := st.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM information_schema.STATISTICS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='pairing_challenges' AND INDEX_NAME='uq_pair_device_id'`).Scan(&indexes); err != nil {
		t.Fatal(err)
	}
	if indexes != 1 {
		t.Fatalf("uq_pair_device_id index count = %d", indexes)
	}
}

func TestMigrateAndUserGithubIdentity(t *testing.T) {
	st := readyStore(t)
	ctx := context.Background()
	gh := uniqueGitHubID(t)
	u1, err := st.UpsertGitHubUser(ctx, GitHubIdentity{ID: gh, Login: "alice", AvatarURL: "https://avatars.githubusercontent.com/u/1"})
	if err != nil {
		t.Fatal(err)
	}
	if u1.GitHubID != gh || u1.Login != "alice" || len(u1.SourceHMACKey) != 32 || u1.ProfileSeed == "" {
		t.Fatalf("%+v", u1)
	}
	u2, err := st.UpsertGitHubUser(ctx, GitHubIdentity{ID: gh, Login: "alice-renamed", AvatarURL: "https://avatars.githubusercontent.com/u/1?v=2"})
	if err != nil {
		t.Fatal(err)
	}
	if u2.ID != u1.ID {
		t.Fatal("github_id must be the stable user key")
	}
	if u2.Login != "alice-renamed" {
		t.Fatalf("login should update, got %q", u2.Login)
	}
	if u2.ProfileSeed != u1.ProfileSeed {
		t.Fatal("profile_seed must stay stable")
	}
	if hex.EncodeToString(u2.SourceHMACKey) != hex.EncodeToString(u1.SourceHMACKey) {
		t.Fatal("source_hmac_key must stay stable")
	}
}

func TestSessionHashRoundTripAndRevoke(t *testing.T) {
	st := readyStore(t)
	ctx := context.Background()
	u, err := st.UpsertGitHubUser(ctx, GitHubIdentity{ID: uniqueGitHubID(t), Login: "bob"})
	if err != nil {
		t.Fatal(err)
	}
	raw, sess, err := st.CreateSession(ctx, u.ID, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if raw == "" || sess.UserID != u.ID {
		t.Fatalf("session %+v raw %q", sess, raw)
	}
	got, err := st.LookupSession(ctx, raw)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != sess.ID {
		t.Fatalf("lookup %+v", got)
	}
	if err := st.DeleteSession(ctx, raw); err != nil {
		t.Fatal(err)
	}
	if _, err := st.LookupSession(ctx, raw); err == nil {
		t.Fatal("revoked session still valid")
	}
}

func TestInsertDeviceAcceptsGoPseudoVersion(t *testing.T) {
	st := readyStore(t)
	ctx := context.Background()
	u, err := st.UpsertGitHubUser(ctx, GitHubIdentity{ID: uniqueGitHubID(t), Login: "ver"})
	if err != nil {
		t.Fatal(err)
	}
	ver := "v0.6.5-0.20260906020645-723001220861"
	if len(ver) <= 32 {
		t.Fatalf("fixture too short: %d", len(ver))
	}
	d, tok, err := st.InsertDevice(ctx, u.ID, DeviceMeta{Label: "Mac", OS: "darwin", Arch: "arm64", ClientVersion: ver})
	if err != nil {
		t.Fatal(err)
	}
	if d.ClientVersion != ver || tok == "" {
		t.Fatalf("%+v", d)
	}
}

func TestAccountGlobalReplaceAndDeviceLocalSum(t *testing.T) {
	st := readyStore(t)
	ctx := context.Background()
	u, err := st.UpsertGitHubUser(ctx, GitHubIdentity{ID: uniqueGitHubID(t), Login: "carol"})
	if err != nil {
		t.Fatal(err)
	}
	d1, tok1, err := st.InsertDevice(ctx, u.ID, DeviceMeta{Label: "Mac", OS: "darwin", Arch: "arm64", ClientVersion: "0.7.0"})
	if err != nil {
		t.Fatal(err)
	}
	d2, _, err := st.InsertDevice(ctx, u.ID, DeviceMeta{Label: "Win", OS: "windows", Arch: "amd64", ClientVersion: "0.7.0"})
	if err != nil {
		t.Fatal(err)
	}
	_ = tok1
	hash, err := syncagg.SourceKeyHash(u.SourceHMACKey, "cursor", syncagg.ScopeAccountGlobal, "cursor-sub")
	if err != nil {
		t.Fatal(err)
	}
	claudeHash, err := syncagg.SourceKeyHash(u.SourceHMACKey, "claude", syncagg.ScopeDeviceLocal, d1.PublicID)
	if err != nil {
		t.Fatal(err)
	}
	claudeHashB, err := syncagg.SourceKeyHash(u.SourceHMACKey, "claude", syncagg.ScopeDeviceLocal, d2.PublicID)
	if err != nil {
		t.Fatal(err)
	}
	row := func(dev int64, scope syncagg.SourceScope, tool, h string, miss int64, rev int64) syncagg.DailyModel {
		return syncagg.DailyModel{
			Date: "2026-09-03", Tool: tool, SourceScope: scope, SourceKeyHash: h,
			Vendor: "anthropic", Model: "claude-opus-4.6",
			Miss: miss, Quality: "authoritative", Derivation: "raw", Revision: rev,
		}
	}
	if err := st.ApplyUsage(ctx, u.ID, d1.ID, []syncagg.DailyModel{
		row(d1.ID, syncagg.ScopeAccountGlobal, "cursor", hash, 100_000_000, 1),
		row(d1.ID, syncagg.ScopeDeviceLocal, "claude", claudeHash, 50_000_000, 1),
	}); err != nil {
		t.Fatal(err)
	}
	if err := st.ApplyUsage(ctx, u.ID, d2.ID, []syncagg.DailyModel{
		row(d2.ID, syncagg.ScopeAccountGlobal, "cursor", hash, 100_000_000, 1),
		row(d2.ID, syncagg.ScopeDeviceLocal, "claude", claudeHashB, 30_000_000, 1),
	}); err != nil {
		t.Fatal(err)
	}
	sum, err := st.SumUsage(ctx, u.ID, "2026-09-03", "2026-09-03")
	if err != nil {
		t.Fatal(err)
	}
	if sum["cursor"] != 100_000_000 {
		t.Fatalf("cursor account_global must REPLACE, got %d", sum["cursor"])
	}
	if sum["claude"] != 80_000_000 {
		t.Fatalf("claude device_local must SUM, got %d", sum["claude"])
	}
}

func TestApplyUsageRejectsStaleRevision(t *testing.T) {
	st := readyStore(t)
	ctx := context.Background()
	u, err := st.UpsertGitHubUser(ctx, GitHubIdentity{ID: uniqueGitHubID(t), Login: "dave"})
	if err != nil {
		t.Fatal(err)
	}
	d, _, err := st.InsertDevice(ctx, u.ID, DeviceMeta{Label: "Mac", OS: "darwin", Arch: "arm64", ClientVersion: "0.7.0"})
	if err != nil {
		t.Fatal(err)
	}
	h, err := syncagg.SourceKeyHash(u.SourceHMACKey, "claude", syncagg.ScopeDeviceLocal, d.PublicID)
	if err != nil {
		t.Fatal(err)
	}
	row := syncagg.DailyModel{
		Date: "2026-09-01", Tool: "claude", SourceScope: syncagg.ScopeDeviceLocal, SourceKeyHash: h,
		Vendor: "anthropic", Model: "claude-opus-4.6", Miss: 10, Revision: 5,
	}
	if err := st.ApplyUsage(ctx, u.ID, d.ID, []syncagg.DailyModel{row}); err != nil {
		t.Fatal(err)
	}
	row.Revision = 4
	row.Miss = 99
	if err := st.ApplyUsage(ctx, u.ID, d.ID, []syncagg.DailyModel{row}); err == nil {
		t.Fatal("stale revision accepted")
	}
}

// TestApplyUsageEqualRevisionRejectsAnyChangedMeasure closes the gap the
// architecture review found: comparing only `miss` on an equal revision let
// a change to cache_read, cache_create, output, requests, user_turns,
// quality, or derivation through unnoticed as long as miss happened to
// match. Every persisted measure must participate in the comparison.
func TestApplyUsageEqualRevisionRejectsAnyChangedMeasure(t *testing.T) {
	st := readyStore(t)
	ctx := context.Background()
	u, err := st.UpsertGitHubUser(ctx, GitHubIdentity{ID: uniqueGitHubID(t), Login: "frank"})
	if err != nil {
		t.Fatal(err)
	}
	d, _, err := st.InsertDevice(ctx, u.ID, DeviceMeta{Label: "Mac", OS: "darwin", Arch: "arm64", ClientVersion: "0.7.0"})
	if err != nil {
		t.Fatal(err)
	}
	h, err := syncagg.SourceKeyHash(u.SourceHMACKey, "claude", syncagg.ScopeDeviceLocal, d.PublicID)
	if err != nil {
		t.Fatal(err)
	}
	base := syncagg.DailyModel{
		Date: "2026-09-05", Tool: "claude", SourceScope: syncagg.ScopeDeviceLocal, SourceKeyHash: h,
		Vendor: "anthropic", Model: "claude-opus-4.6",
		Miss: 10, CacheRead: 1, CacheCreate: 1, Output: 1, Requests: 1, UserTurns: 1,
		Quality: "authoritative", Derivation: "raw", Revision: 9,
	}
	if err := st.ApplyUsage(ctx, u.ID, d.ID, []syncagg.DailyModel{base}); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name string
		row  syncagg.DailyModel
	}{
		{"cache_read", func() syncagg.DailyModel { r := base; r.CacheRead = 99; return r }()},
		{"cache_create", func() syncagg.DailyModel { r := base; r.CacheCreate = 99; return r }()},
		{"output", func() syncagg.DailyModel { r := base; r.Output = 99; return r }()},
		{"requests", func() syncagg.DailyModel { r := base; r.Requests = 99; return r }()},
		{"user_turns", func() syncagg.DailyModel { r := base; r.UserTurns = 99; return r }()},
		{"quality", func() syncagg.DailyModel { r := base; r.Quality = "degraded"; return r }()},
		{"derivation", func() syncagg.DailyModel { r := base; r.Derivation = "deduplicated"; return r }()},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if err := st.ApplyUsage(ctx, u.ID, d.ID, []syncagg.DailyModel{c.row}); err != ErrRevisionClash {
				t.Fatalf("changed %s at equal revision: got err=%v, want ErrRevisionClash", c.name, err)
			}
		})
	}

	// The exact same row at the exact same revision remains a safe no-op —
	// this is the retry path a client actually relies on.
	if err := st.ApplyUsage(ctx, u.ID, d.ID, []syncagg.DailyModel{base}); err != nil {
		t.Fatalf("identical retry at equal revision must not error: %v", err)
	}
}

func TestIdempotentBatchRetry(t *testing.T) {
	st := readyStore(t)
	ctx := context.Background()
	u, err := st.UpsertGitHubUser(ctx, GitHubIdentity{ID: uniqueGitHubID(t), Login: "erin"})
	if err != nil {
		t.Fatal(err)
	}
	d, _, err := st.InsertDevice(ctx, u.ID, DeviceMeta{Label: "Mac", OS: "darwin", Arch: "arm64", ClientVersion: "0.7.0"})
	if err != nil {
		t.Fatal(err)
	}
	h, err := syncagg.SourceKeyHash(u.SourceHMACKey, "claude", syncagg.ScopeDeviceLocal, d.PublicID)
	if err != nil {
		t.Fatal(err)
	}
	batch := syncagg.Batch{
		SchemaVersion:  syncagg.SchemaVersion,
		IdempotencyKey: uniqueID(t),
		DeviceID:       d.PublicID,
		DailyModelUsage: []syncagg.DailyModel{{
			Date: "2026-09-02", Tool: "claude", SourceScope: syncagg.ScopeDeviceLocal, SourceKeyHash: h,
			Vendor: "anthropic", Model: "claude-opus-4.6", Miss: 7, Revision: 1,
		}},
	}
	if err := st.SyncBatch(ctx, u.ID, d.ID, batch); err != nil {
		t.Fatal(err)
	}
	if err := st.SyncBatch(ctx, u.ID, d.ID, batch); err != nil {
		t.Fatal(err)
	}
	sum, err := st.SumUsage(ctx, u.ID, "2026-09-02", "2026-09-02")
	if err != nil {
		t.Fatal(err)
	}
	if sum["claude"] != 7 {
		t.Fatalf("retry doubled usage: %d", sum["claude"])
	}
}

// TestSyncBatchIdempotencyCoversFullEnvelope proves the idempotency key is
// bound to the whole request, not only DailyModelUsage. Reusing a key with
// identical usage rows but a changed source-status list must be rejected as
// a body mismatch rather than silently accepted as a no-op that discards
// the status change.
func TestSyncBatchIdempotencyCoversFullEnvelope(t *testing.T) {
	st := readyStore(t)
	ctx := context.Background()
	u, err := st.UpsertGitHubUser(ctx, GitHubIdentity{ID: uniqueGitHubID(t), Login: "grace"})
	if err != nil {
		t.Fatal(err)
	}
	d, _, err := st.InsertDevice(ctx, u.ID, DeviceMeta{Label: "Mac", OS: "darwin", Arch: "arm64", ClientVersion: "0.7.0"})
	if err != nil {
		t.Fatal(err)
	}
	h, err := syncagg.SourceKeyHash(u.SourceHMACKey, "claude", syncagg.ScopeDeviceLocal, d.PublicID)
	if err != nil {
		t.Fatal(err)
	}
	key := uniqueID(t)
	usage := []syncagg.DailyModel{{
		Date: "2026-09-06", Tool: "claude", SourceScope: syncagg.ScopeDeviceLocal, SourceKeyHash: h,
		Vendor: "anthropic", Model: "claude-opus-4.6", Miss: 3, Revision: 1,
	}}
	first := syncagg.Batch{
		SchemaVersion: syncagg.SchemaVersion, IdempotencyKey: key, DeviceID: d.PublicID,
		DailyModelUsage: usage,
	}
	if err := st.SyncBatch(ctx, u.ID, d.ID, first); err != nil {
		t.Fatal(err)
	}

	changed := syncagg.Batch{
		SchemaVersion: syncagg.SchemaVersion, IdempotencyKey: key, DeviceID: d.PublicID,
		DailyModelUsage: usage,
		Sources: []syncagg.Source{
			{Tool: "claude", Detected: true, Status: "ok", Quality: "authoritative", SourceScope: syncagg.ScopeDeviceLocal, SourceKeyHash: h},
		},
	}
	if err := st.SyncBatch(ctx, u.ID, d.ID, changed); err != ErrIdempotency {
		t.Fatalf("reused key with a changed envelope: got err=%v, want ErrIdempotency", err)
	}
}

// TestSyncBatchAppliesTimezoneAndSourceStatesAtomicallyWithUsage guards the
// atomicity fix: a successful sync must leave usage, timezone, and source
// status consistent together, not usage-only with the rest silently
// discarded on a later statement failure.
func TestSyncBatchAppliesTimezoneAndSourceStatesAtomicallyWithUsage(t *testing.T) {
	st := readyStore(t)
	ctx := context.Background()
	u, err := st.UpsertGitHubUser(ctx, GitHubIdentity{ID: uniqueGitHubID(t), Login: "henry"})
	if err != nil {
		t.Fatal(err)
	}
	d, _, err := st.InsertDevice(ctx, u.ID, DeviceMeta{Label: "Mac", OS: "darwin", Arch: "arm64", ClientVersion: "0.7.0"})
	if err != nil {
		t.Fatal(err)
	}
	h, err := syncagg.SourceKeyHash(u.SourceHMACKey, "claude", syncagg.ScopeDeviceLocal, d.PublicID)
	if err != nil {
		t.Fatal(err)
	}
	batch := syncagg.Batch{
		SchemaVersion: syncagg.SchemaVersion, IdempotencyKey: uniqueID(t), DeviceID: d.PublicID,
		Timezone: "Asia/Shanghai",
		DailyModelUsage: []syncagg.DailyModel{{
			Date: "2026-09-07", Tool: "claude", SourceScope: syncagg.ScopeDeviceLocal, SourceKeyHash: h,
			Vendor: "anthropic", Model: "claude-opus-4.6", Miss: 4, Revision: 1,
		}},
		Sources: []syncagg.Source{
			{Tool: "claude", Detected: true, Status: "ok", Quality: "authoritative", SourceScope: syncagg.ScopeDeviceLocal, SourceKeyHash: h},
		},
	}
	if err := st.SyncBatch(ctx, u.ID, d.ID, batch); err != nil {
		t.Fatal(err)
	}
	if got := st.UserTimezone(ctx, u.ID); got != "Asia/Shanghai" {
		t.Fatalf("timezone not applied with usage: %q", got)
	}
	var status string
	if err := st.db.QueryRowContext(ctx, `SELECT status FROM source_states WHERE user_id=? AND device_id=? AND tool=?`, u.ID, d.ID, "claude").Scan(&status); err != nil {
		t.Fatalf("source_states not applied with usage: %v", err)
	}
	if status != "ok" {
		t.Fatalf("source_states status = %q", status)
	}
}

// TestSyncBatchConcurrentIdenticalRetriesApplyOnce exercises the race the
// idempotency claim-then-apply ordering is meant to close: two callers
// submitting the exact same batch and idempotency key at the same time must
// not double-apply usage, and neither call may fail outright.
func TestSyncBatchConcurrentIdenticalRetriesApplyOnce(t *testing.T) {
	st := readyStore(t)
	ctx := context.Background()
	u, err := st.UpsertGitHubUser(ctx, GitHubIdentity{ID: uniqueGitHubID(t), Login: "iris"})
	if err != nil {
		t.Fatal(err)
	}
	d, _, err := st.InsertDevice(ctx, u.ID, DeviceMeta{Label: "Mac", OS: "darwin", Arch: "arm64", ClientVersion: "0.7.0"})
	if err != nil {
		t.Fatal(err)
	}
	h, err := syncagg.SourceKeyHash(u.SourceHMACKey, "claude", syncagg.ScopeDeviceLocal, d.PublicID)
	if err != nil {
		t.Fatal(err)
	}
	batch := syncagg.Batch{
		SchemaVersion: syncagg.SchemaVersion, IdempotencyKey: uniqueID(t), DeviceID: d.PublicID,
		DailyModelUsage: []syncagg.DailyModel{{
			Date: "2026-09-08", Tool: "claude", SourceScope: syncagg.ScopeDeviceLocal, SourceKeyHash: h,
			Vendor: "anthropic", Model: "claude-opus-4.6", Miss: 11, Revision: 1,
		}},
	}
	const n = 6
	errs := make([]error, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			errs[i] = st.SyncBatch(ctx, u.ID, d.ID, batch)
		}(i)
	}
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Fatalf("concurrent identical retry %d failed: %v", i, err)
		}
	}
	sum, err := st.SumUsage(ctx, u.ID, "2026-09-08", "2026-09-08")
	if err != nil {
		t.Fatal(err)
	}
	if sum["claude"] != 11 {
		t.Fatalf("concurrent identical retries double-applied usage: %d", sum["claude"])
	}
}

func uniqueGitHubID(t *testing.T) int64 {
	t.Helper()
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		t.Fatal(err)
	}
	n := int64(b[0])<<16 | int64(b[1])<<8 | int64(b[2])
	return 1_000_000_000 + n
}

func uniqueID(t *testing.T) string {
	t.Helper()
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		t.Fatal(err)
	}
	return hex.EncodeToString(b[:])
}
