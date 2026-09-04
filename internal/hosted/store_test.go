package hosted

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"testing"
	"time"

	"github.com/rainhuang0220/whereToken/internal/syncagg"
)

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
