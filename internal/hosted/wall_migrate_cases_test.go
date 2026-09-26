package hosted

import (
	"context"
	"database/sql"
	"strings"
	"testing"
	"time"
)

// v0.7.6 public tables, taken from the schema before the verified-wall columns.
// CREATE TABLE IF NOT EXISTS in Migrate does not alter these once they exist.
const (
	v076Projections = `CREATE TABLE public_projections (
  user_id BIGINT NOT NULL,
  snapshot_json MEDIUMTEXT NOT NULL,
  snapshot_id VARCHAR(80) NOT NULL,
  total_tokens BIGINT NOT NULL DEFAULT 0,
  updated_at DATETIME NOT NULL,
  desired_snapshot_id VARCHAR(80) NOT NULL DEFAULT '',
  readme_status VARCHAR(32) NOT NULL DEFAULT '',
  readme_last_error VARCHAR(40) NOT NULL DEFAULT '',
  PRIMARY KEY (user_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`

	v076Presentations = `CREATE TABLE public_presentations (
  user_id BIGINT NOT NULL,
  palette VARCHAR(32) NOT NULL,
  revision VARCHAR(80) NOT NULL,
  asset_revision VARCHAR(80) NOT NULL,
  preview_light MEDIUMBLOB NOT NULL,
  preview_dark MEDIUMBLOB NOT NULL,
  readme_cache_key VARCHAR(160) NOT NULL DEFAULT '',
  readme_snapshot_id VARCHAR(80) NOT NULL DEFAULT '',
  readme_materialized_at DATETIME NULL,
  updated_at DATETIME NOT NULL,
  PRIMARY KEY (user_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`
)

func TestWallPublicationMigrationCases(t *testing.T) {
	t.Run("fresh", testFreshPublicationMigration)
	t.Run("v076", testV076PublicationMigration)
	t.Run("accepted_newer_than_verified", testAcceptedNewerThanVerified)
	t.Run("missing_verified_watermark", testMissingVerifiedWatermark)
	t.Run("pending_failed", testPendingFailedPublication)
}

func testFreshPublicationMigration(t *testing.T) {
	st := openCaseDatabase(t, "wt_case_fresh")
	ctx := context.Background()
	if err := st.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	assertVerifiedColumnNullable(t, st.db)
	if n := countRows(t, st.db, `SELECT COUNT(*) FROM public_presentations`); n != 0 {
		t.Fatalf("fresh presentations = %d", n)
	}
	if n := countRows(t, st.db, `SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'schema_migrations'`); n != 0 {
		t.Fatal("migration invented a schema_migrations table")
	}
	if err := st.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	if n := countRows(t, st.db, `SELECT COUNT(*) FROM public_presentations`); n != 0 {
		t.Fatalf("second migrate created presentations = %d", n)
	}
}

func testV076PublicationMigration(t *testing.T) {
	st := openCaseDatabase(t, "wt_case_v076")
	ctx := context.Background()
	installV076Tables(t, st.db)
	when := time.Date(2026, 9, 25, 12, 0, 0, 0, time.UTC)
	snap := gateProjection(t, "2026-09-25", 42_000)
	insertV076Pair(t, st.db, 1, snap.raw, snap.id, 42_000, snap.id, "applied", "", when)
	if columnExists(t, st.db, "public_presentations", "verified_total_tokens") {
		t.Fatal("v0.7.6 shape already had verified_total_tokens")
	}
	if err := st.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	if !columnExists(t, st.db, "public_projections", "snapshot_id") || !columnExists(t, st.db, "public_projections", "pending_due_at") {
		t.Fatal("migration did not keep the old projection columns and add the pending cursor")
	}
	got := readVerified(t, st.db, 1)
	if !got.total.Valid || got.total.Int64 != 42_000 || got.snap.String != snap.id || got.asOf.String != "2026-09-25" {
		t.Fatalf("proven backfill = %+v", got)
	}
	if err := st.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	again := readVerified(t, st.db, 1)
	if again != got {
		t.Fatalf("second migrate changed verified from %+v to %+v", got, again)
	}

	if _, err := st.db.ExecContext(ctx, `ALTER TABLE public_projections
		DROP COLUMN pending_due_at,
		DROP COLUMN pending_reason,
		DROP COLUMN publish_lease_until,
		DROP COLUMN publish_lease_token,
		DROP COLUMN publish_retry_count`); err != nil {
		t.Fatal(err)
	}
	if _, err := st.db.ExecContext(ctx, `ALTER TABLE public_presentations
		DROP COLUMN verified_snapshot_id,
		DROP COLUMN verified_total_tokens,
		DROP COLUMN verified_as_of_date`); err != nil {
		t.Fatal(err)
	}
	var keptID string
	var keptTotal int64
	if err := st.db.QueryRowContext(ctx, `SELECT snapshot_id, total_tokens FROM public_projections WHERE user_id=1`).Scan(&keptID, &keptTotal); err != nil {
		t.Fatal(err)
	}
	if keptID != snap.id || keptTotal != 42_000 {
		t.Fatalf("rollback changed accepted row id=%s total=%d", keptID, keptTotal)
	}
	if err := st.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	restored := readVerified(t, st.db, 1)
	if !restored.total.Valid || restored.total.Int64 != 42_000 || restored.snap.String != snap.id {
		t.Fatalf("re-migrate after rollback = %+v", restored)
	}
}

func testAcceptedNewerThanVerified(t *testing.T) {
	st := openCaseDatabase(t, "wt_case_newer")
	ctx := context.Background()
	installV076Tables(t, st.db)
	when := time.Date(2026, 9, 25, 12, 0, 0, 0, time.UTC)
	oldSnap := gateProjection(t, "2026-09-25", 42_000)
	insertV076Pair(t, st.db, 1, oldSnap.raw, oldSnap.id, 42_000, oldSnap.id, "applied", "", when)
	if err := st.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	newer := gateProjection(t, "2026-09-26", 80_000)
	if _, err := st.db.ExecContext(ctx, `UPDATE public_projections SET snapshot_json=?, snapshot_id=?, total_tokens=?, desired_snapshot_id=?, updated_at=? WHERE user_id=1`,
		newer.raw, newer.id, 80_000, newer.id, when.Add(24*time.Hour)); err != nil {
		t.Fatal(err)
	}
	if err := st.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	got := readVerified(t, st.db, 1)
	if got.snap.String != oldSnap.id || got.total.Int64 != 42_000 || got.asOf.String != "2026-09-25" {
		t.Fatalf("accepted snapshot replaced verified state: %+v", got)
	}
	if _, err := st.db.ExecContext(ctx, `UPDATE public_presentations SET readme_snapshot_id=? WHERE user_id=1`, newer.id); err != nil {
		t.Fatal(err)
	}
	if err := st.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	still := readVerified(t, st.db, 1)
	if still != got {
		t.Fatalf("backfill overwrote an existing verified wall: %+v", still)
	}
}

func testMissingVerifiedWatermark(t *testing.T) {
	st := openCaseDatabase(t, "wt_case_missing")
	ctx := context.Background()
	installV076Tables(t, st.db)
	when := time.Date(2026, 9, 25, 12, 0, 0, 0, time.UTC)
	known := gateProjection(t, "2026-09-25", 42_000)
	insertV076Pair(t, st.db, 1, known.raw, known.id, 42_000, "", "applied", "", when)
	newer := gateProjection(t, "2026-09-26", 80_000)
	insertV076Pair(t, st.db, 2, newer.raw, newer.id, 80_000, "sha256:applied-older", "failed", "conflict", when)
	nilSnap := []byte(`{"snapshot_id":"sha256:nil-total","as_of_date":"2026-09-25","periods":{"all":{"totals":{"total":{"value":null,"display":"—","status":"unavailable"}}}}}`)
	insertV076Pair(t, st.db, 3, nilSnap, "sha256:nil-total", 0, "sha256:nil-total", "applied", "", when)

	if err := st.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	for _, userID := range []int64{1, 2, 3} {
		got := readVerified(t, st.db, userID)
		if got.snap.Valid || got.total.Valid || got.asOf.Valid {
			t.Fatalf("user %d missing watermark became %+v", userID, got)
		}
	}
	var stored sql.NullInt64
	if err := st.db.QueryRowContext(ctx, `SELECT verified_total_tokens FROM public_presentations WHERE user_id=3`).Scan(&stored); err != nil {
		t.Fatal(err)
	}
	if stored.Valid {
		t.Fatalf("nil JSON total stored as %d", stored.Int64)
	}
}

func testPendingFailedPublication(t *testing.T) {
	st := openCaseDatabase(t, "wt_case_failed")
	ctx := context.Background()
	installV076Tables(t, st.db)
	when := time.Date(2026, 9, 26, 8, 0, 0, 0, time.UTC)
	newer := gateProjection(t, "2026-09-26", 80_000)
	insertV076Pair(t, st.db, 1, newer.raw, newer.id, 80_000, "sha256:applied-older", "failed", "conflict", when)
	if err := st.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	status, last, due, reason, retries := readPending(t, st.db, 1)
	if status != "failed" || last != "conflict" || due.Valid || reason != "" || retries != 0 {
		t.Fatalf("migration rewrote failed publication status=%s err=%s due=%v reason=%s retries=%d", status, last, due, reason, retries)
	}
	assertVerifiedNull(t, readVerified(t, st.db, 1))

	dueAt := when.Add(time.Hour)
	if _, err := st.db.ExecContext(ctx, `UPDATE public_projections SET pending_due_at=?, pending_reason='usage', publish_retry_count=2, publish_lease_token='11111111-1111-1111-1111-111111111111' WHERE user_id=1`, dueAt); err != nil {
		t.Fatal(err)
	}
	if err := st.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	status, last, due, reason, retries = readPending(t, st.db, 1)
	if status != "failed" || last != "conflict" || !due.Valid || !due.Time.Equal(dueAt) || reason != "usage" || retries != 2 {
		t.Fatalf("restart changed pending failure status=%s err=%s due=%v reason=%s retries=%d", status, last, due.Time, reason, retries)
	}
	assertVerifiedNull(t, readVerified(t, st.db, 1))
}

func openCaseDatabase(t *testing.T, name string) *Store {
	t.Helper()
	if strings.ContainsAny(name, "`;'\"") || name == "" {
		t.Fatalf("bad database name %q", name)
	}
	base := readyStore(t)
	if sharedTestDSN == "" {
		t.Fatal("test DSN was not recorded")
	}
	ctx := context.Background()
	if _, err := base.db.ExecContext(ctx, "DROP DATABASE IF EXISTS `"+name+"`"); err != nil {
		t.Fatal(err)
	}
	if _, err := base.db.ExecContext(ctx, "CREATE DATABASE `"+name+"` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = base.db.ExecContext(context.Background(), "DROP DATABASE IF EXISTS `"+name+"`")
	})
	st, err := Open(swapTestDatabase(sharedTestDSN, name))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return st
}

func swapTestDatabase(dsn, name string) string {
	query := ""
	base := dsn
	if i := strings.Index(dsn, "?"); i >= 0 {
		query = dsn[i:]
		base = dsn[:i]
	}
	i := strings.LastIndex(base, "/")
	if i < 0 {
		return dsn + name
	}
	return base[:i+1] + name + query
}

func installV076Tables(t *testing.T, db *sql.DB) {
	t.Helper()
	if _, err := db.Exec(v076Projections); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(v076Presentations); err != nil {
		t.Fatal(err)
	}
}

func insertV076Pair(t *testing.T, db *sql.DB, userID int64, raw []byte, snapshotID string, total int64, readmeID, status, errCode string, when time.Time) {
	t.Helper()
	if _, err := db.Exec(`INSERT INTO public_projections (user_id, snapshot_json, snapshot_id, total_tokens, updated_at, desired_snapshot_id, readme_status, readme_last_error) VALUES (?,?,?,?,?,?,?,?)`,
		userID, raw, snapshotID, total, when, snapshotID, status, errCode); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO public_presentations (user_id, palette, revision, asset_revision, preview_light, preview_dark, readme_cache_key, readme_snapshot_id, readme_materialized_at, updated_at) VALUES (?,?,?,?,?,?,?,?,?,?)`,
		userID, "newsprint", "rev", "sha256:asset", []byte("light"), []byte("dark"), "cache-key", readmeID, when, when); err != nil {
		t.Fatal(err)
	}
}

type verifiedCols struct {
	snap  sql.NullString
	total sql.NullInt64
	asOf  sql.NullString
}

func readVerified(t *testing.T, db *sql.DB, userID int64) verifiedCols {
	t.Helper()
	var got verifiedCols
	if err := db.QueryRow(`SELECT verified_snapshot_id, verified_total_tokens, verified_as_of_date FROM public_presentations WHERE user_id=?`, userID).Scan(&got.snap, &got.total, &got.asOf); err != nil {
		t.Fatal(err)
	}
	return got
}

func assertVerifiedNull(t *testing.T, got verifiedCols) {
	t.Helper()
	if got.snap.Valid || got.total.Valid || got.asOf.Valid {
		t.Fatalf("verified identity was invented: %+v", got)
	}
}

func readPending(t *testing.T, db *sql.DB, userID int64) (status, last string, due sql.NullTime, reason string, retries int) {
	t.Helper()
	if err := db.QueryRow(`SELECT readme_status, readme_last_error, pending_due_at, pending_reason, publish_retry_count FROM public_projections WHERE user_id=?`, userID).Scan(&status, &last, &due, &reason, &retries); err != nil {
		t.Fatal(err)
	}
	return status, last, due, reason, retries
}

func columnExists(t *testing.T, db *sql.DB, table, column string) bool {
	t.Helper()
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ? AND COLUMN_NAME = ?`, table, column).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n == 1
}

func assertVerifiedColumnNullable(t *testing.T, db *sql.DB) {
	t.Helper()
	var nullable string
	var def sql.NullString
	if err := db.QueryRow(`SELECT IS_NULLABLE, COLUMN_DEFAULT FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'public_presentations' AND COLUMN_NAME = 'verified_total_tokens'`).Scan(&nullable, &def); err != nil {
		t.Fatal(err)
	}
	if nullable != "YES" || (def.Valid && def.String == "0") {
		t.Fatalf("verified_total_tokens nullable=%s default=%v %q", nullable, def.Valid, def.String)
	}
}

func countRows(t *testing.T, db *sql.DB, query string) int {
	t.Helper()
	var n int
	if err := db.QueryRow(query).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}
