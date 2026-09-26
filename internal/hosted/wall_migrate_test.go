package hosted

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/rainhuang0220/whereToken/internal/event"
	"github.com/rainhuang0220/whereToken/internal/publicprofile"
)

func TestBackfillVerifiedIdentityDoesNotInventATotal(t *testing.T) {
	st := readyStore(t)
	ctx := context.Background()
	user, err := st.UpsertGitHubUser(ctx, GitHubIdentity{ID: 909090, Login: "wall-migrate", AvatarURL: "https://avatars.githubusercontent.com/u/1"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = st.db.ExecContext(context.Background(), `DELETE FROM public_projections WHERE user_id=?`, user.ID)
		_, _ = st.db.ExecContext(context.Background(), `DELETE FROM public_presentations WHERE user_id=?`, user.ID)
	})
	snap := gateProjection(t, "2026-09-25", 42_000)
	if _, err := st.db.ExecContext(ctx, `INSERT INTO public_projections (user_id, snapshot_json, snapshot_id, total_tokens, updated_at, desired_snapshot_id) VALUES (?,?,?,?,?,?)`,
		user.ID, snap.raw, snap.id, 42_000, time.Date(2026, 9, 25, 12, 0, 0, 0, time.UTC), snap.id); err != nil {
		t.Fatal(err)
	}
	if _, err := st.db.ExecContext(ctx, `INSERT INTO public_presentations (user_id, palette, revision, asset_revision, preview_light, preview_dark, readme_cache_key, readme_snapshot_id, readme_materialized_at, updated_at) VALUES (?,?,?,?,?,?,?,?,?,?)`,
		user.ID, "newsprint", "rev", "sha256:asset", []byte("l"), []byte("d"), "cache", snap.id, time.Date(2026, 9, 25, 12, 0, 0, 0, time.UTC), time.Date(2026, 9, 25, 12, 0, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}
	if err := st.backfillVerifiedIdentity(ctx); err != nil {
		t.Fatal(err)
	}
	pres, err := st.Presentation(ctx, user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !pres.VerifiedTotal.Valid || pres.VerifiedTotal.Int64 != 42_000 || pres.VerifiedAsOf.String != "2026-09-25" || pres.VerifiedSnapshotID.String != snap.id {
		t.Fatalf("proven identity %+v", pres)
	}
	if err := st.backfillVerifiedIdentity(ctx); err != nil {
		t.Fatal(err)
	}
	again, err := st.Presentation(ctx, user.ID)
	if err != nil || again.VerifiedTotal.Int64 != 42_000 {
		t.Fatalf("repeat %+v %v", again.VerifiedTotal, err)
	}

	other, err := st.UpsertGitHubUser(ctx, GitHubIdentity{ID: 909091, Login: "wall-unknown", AvatarURL: "https://avatars.githubusercontent.com/u/2"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = st.db.ExecContext(context.Background(), `DELETE FROM public_projections WHERE user_id=?`, other.ID)
		_, _ = st.db.ExecContext(context.Background(), `DELETE FROM public_presentations WHERE user_id=?`, other.ID)
	})
	newer := gateProjection(t, "2026-09-26", 80_000)
	if _, err := st.db.ExecContext(ctx, `INSERT INTO public_projections (user_id, snapshot_json, snapshot_id, total_tokens, updated_at, desired_snapshot_id, readme_status) VALUES (?,?,?,?,?,?,?)`,
		other.ID, newer.raw, newer.id, 80_000, time.Date(2026, 9, 26, 8, 0, 0, 0, time.UTC), newer.id, "failed"); err != nil {
		t.Fatal(err)
	}
	if _, err := st.db.ExecContext(ctx, `INSERT INTO public_presentations (user_id, palette, revision, asset_revision, preview_light, preview_dark, readme_snapshot_id, readme_materialized_at, updated_at) VALUES (?,?,?,?,?,?,?,?,?)`,
		other.ID, "newsprint", "rev", "sha256:asset", []byte("l"), []byte("d"), "sha256:applied-older", time.Date(2026, 9, 25, 12, 0, 0, 0, time.UTC), time.Date(2026, 9, 25, 12, 0, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}
	if err := st.backfillVerifiedIdentity(ctx); err != nil {
		t.Fatal(err)
	}
	unknown, err := st.Presentation(ctx, other.ID)
	if err != nil {
		t.Fatal(err)
	}
	if unknown.VerifiedTotal.Valid || unknown.VerifiedSnapshotID.Valid || unknown.VerifiedAsOf.Valid {
		t.Fatalf("unknown legacy total was invented: %+v", unknown)
	}
	var stored sql.NullInt64
	if err := st.db.QueryRowContext(ctx, `SELECT verified_total_tokens FROM public_presentations WHERE user_id=?`, other.ID).Scan(&stored); err != nil {
		t.Fatal(err)
	}
	if stored.Valid {
		t.Fatal("NULL verified total became a number")
	}
}

type gateBuilt struct {
	raw []byte
	id  string
}

func gateProjection(t *testing.T, date string, total int64) gateBuilt {
	t.Helper()
	when, err := time.Parse("2006-01-02", date)
	if err != nil {
		t.Fatal(err)
	}
	when = time.Date(when.Year(), when.Month(), when.Day(), 12, 0, 0, 0, time.UTC)
	built := mustProjection(t, publicprofile.Input{
		Now: when, Loc: time.UTC, Version: "test",
		Events: []event.UsageEvent{authEvent("claude", "anthropic", when.Add(-time.Hour), total)},
	})
	return gateBuilt{raw: built.raw, id: built.snap.SnapshotID}
}
