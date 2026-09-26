package hosted

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"time"

	"github.com/rainhuang0220/whereToken/internal/publicprofile"
)

const profileSessionTTL = 2 * time.Hour

type projectionRow struct {
	SnapshotJSON      []byte
	SnapshotID        string
	TotalTokens       int64
	UpdatedAt         time.Time
	DesiredSnapshotID string
	ReadmeStatus      string
	ReadmeLastError   string
	PendingDueAt      sql.NullTime
	PendingReason     string
	PublishRetryCount int
}

type presentationRow struct {
	Palette              string
	Revision             string
	AssetRevision        string
	PreviewLight         []byte
	PreviewDark          []byte
	ReadmeCacheKey       string
	ReadmeSnapshotID     string
	ReadmeMaterializedAt sql.NullTime
	VerifiedSnapshotID   sql.NullString
	VerifiedTotal        sql.NullInt64
	VerifiedAsOf         sql.NullString
	UpdatedAt            time.Time
}

type publishJob struct {
	ID            string
	UserID        int64
	Kind          string
	Palette       string
	Phase         string
	SnapshotID    string
	AssetRevision string
	CacheKey      string
	ErrorText     string
	ResultCode    string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

func (s *Store) SaveProjection(ctx context.Context, userID int64, canonical []byte, snap publicprofile.Snapshot) error {
	now := s.clock()
	_, err := s.db.ExecContext(ctx, `
INSERT INTO public_projections (user_id, snapshot_json, snapshot_id, total_tokens, updated_at, desired_snapshot_id)
VALUES (?,?,?,?,?,?)
ON DUPLICATE KEY UPDATE snapshot_json=VALUES(snapshot_json), snapshot_id=VALUES(snapshot_id), total_tokens=VALUES(total_tokens), updated_at=VALUES(updated_at), desired_snapshot_id=VALUES(desired_snapshot_id)`,
		userID, canonical, snap.SnapshotID, publicprofile.TotalTokens(snap), now, snap.SnapshotID)
	return err
}

func (s *Store) Projection(ctx context.Context, userID int64) (projectionRow, error) {
	var row projectionRow
	err := s.db.QueryRowContext(ctx, `SELECT snapshot_json, snapshot_id, total_tokens, updated_at, desired_snapshot_id, readme_status, readme_last_error, pending_due_at, pending_reason, publish_retry_count FROM public_projections WHERE user_id=?`, userID).Scan(
		&row.SnapshotJSON, &row.SnapshotID, &row.TotalTokens, &row.UpdatedAt, &row.DesiredSnapshotID, &row.ReadmeStatus, &row.ReadmeLastError, &row.PendingDueAt, &row.PendingReason, &row.PublishRetryCount)
	if err == sql.ErrNoRows {
		return projectionRow{}, ErrNotFound
	}
	return row, err
}

func (s *Store) SavePresentation(ctx context.Context, userID int64, palette, revision, assetRevision string, light, dark []byte) error {
	now := s.clock()
	_, err := s.db.ExecContext(ctx, `
INSERT INTO public_presentations (user_id, palette, revision, asset_revision, preview_light, preview_dark, updated_at)
VALUES (?,?,?,?,?,?,?)
ON DUPLICATE KEY UPDATE palette=VALUES(palette), revision=VALUES(revision), asset_revision=VALUES(asset_revision), preview_light=VALUES(preview_light), preview_dark=VALUES(preview_dark), updated_at=VALUES(updated_at)`,
		userID, palette, revision, assetRevision, light, dark, now)
	return err
}

func (s *Store) SetReadmeStatus(ctx context.Context, userID int64, status, code string) error {
	if len(code) > 40 {
		code = code[:40]
	}
	_, err := s.db.ExecContext(ctx, `UPDATE public_projections SET readme_status=?, readme_last_error=? WHERE user_id=?`, status, code, userID)
	return err
}

func (s *Store) DuePublicationUserIDs(ctx context.Context, now time.Time) ([]int64, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT p.user_id
FROM public_projections p
LEFT JOIN public_presentations pr ON pr.user_id = p.user_id
WHERE p.desired_snapshot_id <> ''
  AND p.desired_snapshot_id <> COALESCE(pr.readme_snapshot_id, '')
  AND p.readme_status IN ('failed', 'pending', 'blocked_auth')
  AND (p.pending_due_at IS NULL OR p.pending_due_at <= ?)
  AND (p.publish_lease_until IS NULL OR p.publish_lease_until < ?)`, now, now)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// MarkReadmeVerified records a GitHub wall only after the caller has read
// the remote README and both SVGs. A stale lease or a newer verification
// updates nothing. A nil total stays NULL.
func (s *Store) MarkReadmeVerified(ctx context.Context, userID int64, cacheKey, snapshotID, asOf string, total *int64, leaseToken string, notAfter time.Time) error {
	now := s.clock()
	var totalArg any
	if total != nil {
		totalArg = *total
	}
	var asOfArg any
	if strings.TrimSpace(asOf) != "" {
		asOfArg = asOf
	}
	var notAfterArg any
	if !notAfter.IsZero() {
		notAfterArg = notAfter
	}
	res, err := s.db.ExecContext(ctx, `
UPDATE public_presentations
SET readme_cache_key=?, readme_snapshot_id=?, readme_materialized_at=?,
    verified_snapshot_id=?, verified_total_tokens=?, verified_as_of_date=?, updated_at=?
WHERE user_id=?
  AND (
    readme_snapshot_id=''
    OR readme_snapshot_id=?
    OR (readme_materialized_at IS NOT NULL AND ? IS NOT NULL AND readme_materialized_at <= ?)
  )
  AND EXISTS (
    SELECT 1 FROM public_projections WHERE user_id=? AND publish_lease_token=?
  )`, cacheKey, snapshotID, now, snapshotID, totalArg, asOfArg, now, userID, snapshotID, notAfterArg, notAfterArg, userID, leaseToken)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 1 {
		return nil
	}
	pres, readErr := s.Presentation(ctx, userID)
	if readErr == nil && pres.ReadmeSnapshotID == snapshotID && pres.VerifiedSnapshotID.Valid && pres.VerifiedSnapshotID.String == snapshotID {
		return nil
	}
	return errVerifiedMoved
}

func (s *Store) SetPublicationIntent(ctx context.Context, userID int64, desired, status, reason, errCode string, due sql.NullTime, retry int) error {
	if len(errCode) > 40 {
		errCode = errCode[:40]
	}
	if len(reason) > 32 {
		reason = reason[:32]
	}
	_, err := s.db.ExecContext(ctx, `UPDATE public_projections SET desired_snapshot_id=?, readme_status=?, readme_last_error=?, pending_reason=?, pending_due_at=?, publish_retry_count=? WHERE user_id=?`,
		desired, status, errCode, reason, due, retry, userID)
	return err
}

func (s *Store) SetReadmeApplied(ctx context.Context, userID int64, snapshotID string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE public_projections SET readme_status='applied', readme_last_error='', pending_reason='', pending_due_at=NULL, publish_retry_count=0 WHERE user_id=? AND desired_snapshot_id=?`, userID, snapshotID)
	return err
}

func (s *Store) AcquirePublishLease(ctx context.Context, userID int64, token string, until time.Time) (bool, error) {
	now := s.clock()
	res, err := s.db.ExecContext(ctx, `
UPDATE public_projections
SET publish_lease_until=?, publish_lease_token=?
WHERE user_id=? AND (publish_lease_until IS NULL OR publish_lease_until < ? OR publish_lease_token=?)`,
		until, token, userID, now, token)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n == 1, err
}

func (s *Store) ReleasePublishLease(ctx context.Context, userID int64, token string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE public_projections SET publish_lease_until=NULL, publish_lease_token=NULL WHERE user_id=? AND publish_lease_token=?`, userID, token)
	return err
}

// backfillVerifiedIdentity copies the exact total and local date only when
// the applied README snapshot is still the accepted projection. A newer
// accepted snapshot leaves the verified total unknown. Repeated runs do not
// overwrite a value that is already stored.
func (s *Store) backfillVerifiedIdentity(ctx context.Context) error {
	rows, err := s.db.QueryContext(ctx, `
SELECT pr.user_id, pr.readme_snapshot_id, p.snapshot_json
FROM public_presentations pr
JOIN public_projections p ON p.user_id = pr.user_id
WHERE pr.readme_snapshot_id <> ''
  AND pr.readme_snapshot_id = p.snapshot_id
  AND pr.verified_snapshot_id IS NULL
  AND pr.verified_total_tokens IS NULL
  AND (pr.verified_as_of_date IS NULL OR pr.verified_as_of_date = '')`)
	if err != nil {
		return err
	}
	defer rows.Close()
	type row struct {
		userID int64
		id     string
		raw    []byte
	}
	var pending []row
	for rows.Next() {
		var item row
		if err := rows.Scan(&item.userID, &item.id, &item.raw); err != nil {
			return err
		}
		pending = append(pending, item)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for _, item := range pending {
		var snap publicprofile.Snapshot
		if json.Unmarshal(item.raw, &snap) != nil || snap.SnapshotID != item.id {
			continue
		}
		total := snap.Periods.All.Totals.Total.Value
		if total == nil || !validDate(snap.AsOfDate) {
			continue
		}
		if _, err := s.db.ExecContext(ctx, `
UPDATE public_presentations
SET verified_snapshot_id=?, verified_total_tokens=?, verified_as_of_date=?
WHERE user_id=? AND readme_snapshot_id=? AND verified_snapshot_id IS NULL AND verified_total_tokens IS NULL`,
			item.id, *total, snap.AsOfDate, item.userID, item.id); err != nil {
			return err
		}
	}
	return nil
}

func validDate(s string) bool {
	_, err := time.Parse("2006-01-02", strings.TrimSpace(s))
	return err == nil
}

func (s *Store) Presentation(ctx context.Context, userID int64) (presentationRow, error) {
	var row presentationRow
	err := s.db.QueryRowContext(ctx, `SELECT palette, revision, asset_revision, preview_light, preview_dark, readme_cache_key, readme_snapshot_id, readme_materialized_at, verified_snapshot_id, verified_total_tokens, verified_as_of_date, updated_at FROM public_presentations WHERE user_id=?`, userID).Scan(
		&row.Palette, &row.Revision, &row.AssetRevision, &row.PreviewLight, &row.PreviewDark, &row.ReadmeCacheKey, &row.ReadmeSnapshotID, &row.ReadmeMaterializedAt, &row.VerifiedSnapshotID, &row.VerifiedTotal, &row.VerifiedAsOf, &row.UpdatedAt)
	if err == sql.ErrNoRows {
		return presentationRow{}, ErrNotFound
	}
	return row, err
}

func (s *Store) InsertJob(ctx context.Context, job publishJob) error {
	now := s.clock()
	_, err := s.db.ExecContext(ctx, `INSERT INTO publish_jobs (id, user_id, kind, palette, phase, snapshot_id, asset_revision, cache_key, error_text, result_code, created_at, updated_at) VALUES (?,?,?,?,?,?,?,?,?,?,?,?)`,
		job.ID, job.UserID, job.Kind, job.Palette, job.Phase, job.SnapshotID, job.AssetRevision, job.CacheKey, job.ErrorText, job.ResultCode, now, now)
	return err
}

func (s *Store) UpdateJob(ctx context.Context, job publishJob) error {
	now := s.clock()
	_, err := s.db.ExecContext(ctx, `UPDATE publish_jobs SET phase=?, snapshot_id=?, asset_revision=?, cache_key=?, error_text=?, result_code=?, updated_at=? WHERE id=? AND user_id=?`,
		job.Phase, job.SnapshotID, job.AssetRevision, job.CacheKey, trimJobError(job.ErrorText), job.ResultCode, now, job.ID, job.UserID)
	return err
}

func (s *Store) Job(ctx context.Context, userID int64, id string) (publishJob, error) {
	var job publishJob
	err := s.db.QueryRowContext(ctx, `SELECT id, user_id, kind, palette, phase, snapshot_id, asset_revision, cache_key, error_text, result_code, created_at, updated_at FROM publish_jobs WHERE id=? AND user_id=?`, id, userID).Scan(
		&job.ID, &job.UserID, &job.Kind, &job.Palette, &job.Phase, &job.SnapshotID, &job.AssetRevision, &job.CacheKey, &job.ErrorText, &job.ResultCode, &job.CreatedAt, &job.UpdatedAt)
	if err == sql.ErrNoRows {
		return publishJob{}, ErrNotFound
	}
	return job, err
}

func trimJobError(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > 400 {
		s = s[:400]
	}
	return s
}

func (s *Store) UserByLogin(ctx context.Context, login string) (User, error) {
	var u User
	err := s.db.QueryRowContext(ctx, `SELECT id, public_id, github_id, github_login, avatar_url, profile_seed, source_hmac_key FROM users WHERE LOWER(github_login)=LOWER(?) AND deleted_at IS NULL`, login).Scan(
		&u.ID, &u.PublicID, &u.GitHubID, &u.Login, &u.AvatarURL, &u.ProfileSeed, &u.SourceHMACKey)
	if err == sql.ErrNoRows {
		return User{}, ErrNotFound
	}
	return u, err
}

func (s *Store) CreateProfileSession(ctx context.Context, userID int64) (token, csrf string, exp time.Time, err error) {
	raw, hash, err := newOpaqueToken()
	if err != nil {
		return "", "", time.Time{}, err
	}
	csrfRaw, csrfHash, err := newOpaqueToken()
	if err != nil {
		return "", "", time.Time{}, err
	}
	now := s.clock()
	exp = now.Add(profileSessionTTL)
	if _, err := s.db.ExecContext(ctx, `INSERT INTO profile_sessions (user_id, token_hash, csrf_hash, expires_at, created_at) VALUES (?,?,?,?,?)`,
		userID, hash, csrfHash, exp, now); err != nil {
		return "", "", time.Time{}, err
	}
	return "wtp_1." + raw, csrfRaw, exp, nil
}

func (s *Store) LookupProfileSession(ctx context.Context, raw string) (Session, error) {
	raw = strings.TrimPrefix(strings.TrimSpace(raw), "wtp_1.")
	if raw == "" {
		return Session{}, ErrUnauthorized
	}
	var sess Session
	err := s.db.QueryRowContext(ctx, `SELECT id, user_id, csrf_hash, expires_at FROM profile_sessions WHERE token_hash=?`, hashBytes([]byte(raw))).Scan(
		&sess.ID, &sess.UserID, &sess.CSRFHash, &sess.ExpiresAt)
	if err == sql.ErrNoRows {
		return Session{}, ErrUnauthorized
	}
	if err != nil {
		return Session{}, err
	}
	if !sess.ExpiresAt.After(s.clock()) {
		return Session{}, ErrUnauthorized
	}
	return sess, nil
}

func (s *Store) CreateExchangeCode(ctx context.Context, userID int64) (string, error) {
	raw, hash, err := newOpaqueToken()
	if err != nil {
		return "", err
	}
	now := s.clock()
	if _, err := s.db.ExecContext(ctx, `INSERT INTO profile_exchange_codes (code_hash, user_id, expires_at, created_at) VALUES (?,?,?,?)`,
		hash, userID, now.Add(2*time.Minute), now); err != nil {
		return "", err
	}
	return raw, nil
}

func (s *Store) ConsumeExchangeCode(ctx context.Context, raw string) (int64, error) {
	if strings.TrimSpace(raw) == "" {
		return 0, ErrNotFound
	}
	hash := hashBytes([]byte(raw))
	var userID int64
	var exp time.Time
	var consumed sql.NullTime
	err := s.db.QueryRowContext(ctx, `SELECT user_id, expires_at, consumed_at FROM profile_exchange_codes WHERE code_hash=?`, hash).Scan(&userID, &exp, &consumed)
	if err == sql.ErrNoRows {
		return 0, ErrNotFound
	}
	if err != nil {
		return 0, err
	}
	if consumed.Valid || !exp.After(s.clock()) {
		return 0, ErrNotFound
	}
	res, err := s.db.ExecContext(ctx, `UPDATE profile_exchange_codes SET consumed_at=? WHERE code_hash=? AND consumed_at IS NULL`, s.clock(), hash)
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	if n != 1 {
		return 0, ErrNotFound
	}
	return userID, nil
}

func (s *Store) deletePublicProfile(ctx context.Context, tx *sql.Tx, userID int64) error {
	for _, q := range []string{
		`DELETE FROM public_projections WHERE user_id=?`,
		`DELETE FROM public_presentations WHERE user_id=?`,
		`DELETE FROM publish_jobs WHERE user_id=?`,
		`DELETE FROM profile_sessions WHERE user_id=?`,
		`DELETE FROM profile_exchange_codes WHERE user_id=?`,
	} {
		if _, err := tx.ExecContext(ctx, q, userID); err != nil {
			return err
		}
	}
	return nil
}
