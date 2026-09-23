package hosted

import (
	"context"
	"database/sql"
	"strings"
	"time"

	"github.com/rainhuang0220/whereToken/internal/publicprofile"
)

const profileSessionTTL = 2 * time.Hour

type projectionRow struct {
	SnapshotJSON []byte
	SnapshotID   string
	TotalTokens  int64
	UpdatedAt    time.Time
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
INSERT INTO public_projections (user_id, snapshot_json, snapshot_id, total_tokens, updated_at)
VALUES (?,?,?,?,?)
ON DUPLICATE KEY UPDATE snapshot_json=VALUES(snapshot_json), snapshot_id=VALUES(snapshot_id), total_tokens=VALUES(total_tokens), updated_at=VALUES(updated_at)`,
		userID, canonical, snap.SnapshotID, publicprofile.TotalTokens(snap), now)
	return err
}

func (s *Store) Projection(ctx context.Context, userID int64) (projectionRow, error) {
	var row projectionRow
	err := s.db.QueryRowContext(ctx, `SELECT snapshot_json, snapshot_id, total_tokens, updated_at FROM public_projections WHERE user_id=?`, userID).Scan(
		&row.SnapshotJSON, &row.SnapshotID, &row.TotalTokens, &row.UpdatedAt)
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

func (s *Store) MarkReadmeMaterialized(ctx context.Context, userID int64, cacheKey, snapshotID string) error {
	now := s.clock()
	_, err := s.db.ExecContext(ctx, `UPDATE public_presentations SET readme_cache_key=?, readme_snapshot_id=?, readme_materialized_at=?, updated_at=? WHERE user_id=?`,
		cacheKey, snapshotID, now, now, userID)
	return err
}

func (s *Store) Presentation(ctx context.Context, userID int64) (presentationRow, error) {
	var row presentationRow
	err := s.db.QueryRowContext(ctx, `SELECT palette, revision, asset_revision, preview_light, preview_dark, readme_cache_key, readme_snapshot_id, readme_materialized_at, updated_at FROM public_presentations WHERE user_id=?`, userID).Scan(
		&row.Palette, &row.Revision, &row.AssetRevision, &row.PreviewLight, &row.PreviewDark, &row.ReadmeCacheKey, &row.ReadmeSnapshotID, &row.ReadmeMaterializedAt, &row.UpdatedAt)
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
