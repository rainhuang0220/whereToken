package hosted

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/rainhuang0220/whereToken/internal/syncagg"

	_ "github.com/go-sql-driver/mysql"
)

var (
	ErrNotFound      = errors.New("not found")
	ErrStaleRevision = errors.New("stale revision")
	ErrRevisionClash = errors.New("revision conflict")
	ErrIdempotency   = errors.New("idempotency key reused with different body")
	ErrUnauthorized  = errors.New("unauthorized")
)

type GitHubIdentity struct {
	ID        int64
	Login     string
	AvatarURL string
}

type User struct {
	ID            int64
	PublicID      string
	GitHubID      int64
	Login         string
	AvatarURL     string
	ProfileSeed   string
	SourceHMACKey []byte
}

type Session struct {
	ID        int64
	UserID    int64
	CSRFToken string
	CSRFHash  []byte
	ExpiresAt time.Time
}

type DeviceMeta struct {
	Label         string
	OS            string
	Arch          string
	ClientVersion string
}

type Device struct {
	ID            int64
	UserID        int64
	PublicID      string
	Label         string
	OS            string
	Arch          string
	ClientVersion string
	LastSeenAt    time.Time
	LastSyncAt    time.Time
	Revoked       bool
}

type Store struct {
	db  *sql.DB
	now func() time.Time
}

func Open(dsn string) (*Store, error) {
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, err
	}
	return &Store{db: db, now: time.Now}, nil
}

func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *Store) clock() time.Time {
	if s.now != nil {
		return s.now()
	}
	return time.Now()
}

func (s *Store) UpsertGitHubUser(ctx context.Context, id GitHubIdentity) (User, error) {
	var u User
	err := s.db.QueryRowContext(ctx, `SELECT id, public_id, github_id, github_login, avatar_url, profile_seed, source_hmac_key FROM users WHERE github_id=? AND deleted_at IS NULL`, id.ID).Scan(
		&u.ID, &u.PublicID, &u.GitHubID, &u.Login, &u.AvatarURL, &u.ProfileSeed, &u.SourceHMACKey,
	)
	now := s.clock()
	if err == sql.ErrNoRows {
		publicID, err := newUUID4()
		if err != nil {
			return User{}, err
		}
		seed, err := newUUID4()
		if err != nil {
			return User{}, err
		}
		key, err := randomBytes(32)
		if err != nil {
			return User{}, err
		}
		res, err := s.db.ExecContext(ctx, `INSERT INTO users (public_id, github_id, github_login, avatar_url, profile_seed, source_hmac_key, created_at, updated_at) VALUES (?,?,?,?,?,?,?,?)`,
			publicID, id.ID, id.Login, id.AvatarURL, seed, key, now, now)
		if err != nil {
			return User{}, err
		}
		pk, err := res.LastInsertId()
		if err != nil {
			return User{}, err
		}
		return User{ID: pk, PublicID: publicID, GitHubID: id.ID, Login: id.Login, AvatarURL: id.AvatarURL, ProfileSeed: seed, SourceHMACKey: key}, nil
	}
	if err != nil {
		return User{}, err
	}
	if _, err := s.db.ExecContext(ctx, `UPDATE users SET github_login=?, avatar_url=?, updated_at=? WHERE id=?`, id.Login, id.AvatarURL, now, u.ID); err != nil {
		return User{}, err
	}
	u.Login = id.Login
	u.AvatarURL = id.AvatarURL
	return u, nil
}

func (s *Store) CreateSession(ctx context.Context, userID int64, ttl time.Duration) (string, Session, error) {
	raw, hash, err := newOpaqueToken()
	if err != nil {
		return "", Session{}, err
	}
	csrfRaw, csrfHash, err := newOpaqueToken()
	if err != nil {
		return "", Session{}, err
	}
	now := s.clock()
	exp := now.Add(ttl)
	res, err := s.db.ExecContext(ctx, `INSERT INTO web_sessions (user_id, token_hash, csrf_hash, expires_at, created_at, last_seen_at) VALUES (?,?,?,?,?,?)`,
		userID, hash, csrfHash, exp, now, now)
	if err != nil {
		return "", Session{}, err
	}
	pk, err := res.LastInsertId()
	if err != nil {
		return "", Session{}, err
	}
	return raw, Session{ID: pk, UserID: userID, CSRFToken: csrfRaw, ExpiresAt: exp}, nil
}

func (s *Store) LookupSession(ctx context.Context, raw string) (Session, error) {
	if raw == "" {
		return Session{}, ErrNotFound
	}
	hash := hashBytes([]byte(raw))
	var sess Session
	err := s.db.QueryRowContext(ctx, `SELECT id, user_id, expires_at, csrf_hash FROM web_sessions WHERE token_hash=?`, hash).Scan(&sess.ID, &sess.UserID, &sess.ExpiresAt, &sess.CSRFHash)
	if err == sql.ErrNoRows {
		return Session{}, ErrNotFound
	}
	if err != nil {
		return Session{}, err
	}
	if !sess.ExpiresAt.After(s.clock()) {
		return Session{}, ErrNotFound
	}
	return sess, nil
}

func (s *Store) DeleteSession(ctx context.Context, raw string) error {
	hash := hashBytes([]byte(raw))
	_, err := s.db.ExecContext(ctx, `DELETE FROM web_sessions WHERE token_hash=?`, hash)
	return err
}

func (s *Store) InsertDevice(ctx context.Context, userID int64, meta DeviceMeta) (Device, string, error) {
	publicID, err := newUUID4()
	if err != nil {
		return Device{}, "", err
	}
	raw, hash, err := newDeviceToken()
	if err != nil {
		return Device{}, "", err
	}
	now := s.clock()
	meta = normalizeDeviceMeta(meta)
	res, err := s.db.ExecContext(ctx, `INSERT INTO devices (user_id, public_id, label, os, arch, client_version, token_hash, token_version, created_at, last_seen_at) VALUES (?,?,?,?,?,?,?,1,?,?)`,
		userID, publicID, meta.Label, meta.OS, meta.Arch, meta.ClientVersion, hash, now, now)
	if err != nil {
		return Device{}, "", err
	}
	pk, err := res.LastInsertId()
	if err != nil {
		return Device{}, "", err
	}
	return Device{ID: pk, UserID: userID, PublicID: publicID, Label: meta.Label, OS: meta.OS, Arch: meta.Arch, ClientVersion: meta.ClientVersion}, raw, nil
}

func normalizeDeviceMeta(meta DeviceMeta) DeviceMeta {
	if meta.Label == "" {
		meta.Label = strings.TrimSpace(meta.OS + " " + meta.Arch)
	}
	if len(meta.Label) > 128 {
		meta.Label = meta.Label[:128]
	}
	if len(meta.OS) > 32 {
		meta.OS = meta.OS[:32]
	}
	if len(meta.Arch) > 32 {
		meta.Arch = meta.Arch[:32]
	}
	if len(meta.ClientVersion) > 64 {
		meta.ClientVersion = meta.ClientVersion[:64]
	}
	return meta
}

// storedRow mirrors the persisted measures of one usage_daily_model row that
// participate in equal-revision comparison. Anything not listed here
// (writer_device_id, updated_at) is bookkeeping, not a measure, and does not
// gate the comparison.
type storedRow struct {
	Revision                             int64
	Miss, CacheRead, CacheCreate, Output int64
	Requests, UserTurns                  int64
	Quality, Derivation                  string
}

func (a storedRow) equalMeasures(b syncagg.DailyModel) bool {
	return a.Miss == b.Miss && a.CacheRead == b.CacheRead && a.CacheCreate == b.CacheCreate &&
		a.Output == b.Output && a.Requests == b.Requests && a.UserTurns == b.UserTurns &&
		a.Quality == b.Quality && a.Derivation == b.Derivation
}

func (s *Store) ApplyUsage(ctx context.Context, userID, deviceID int64, rows []syncagg.DailyModel) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := applyUsageTx(ctx, tx, s.clock(), userID, deviceID, rows); err != nil {
		return err
	}
	return tx.Commit()
}

// applyUsageTx is the transaction body shared by ApplyUsage (used directly
// by tests and by any future non-idempotency-tracked caller) and SyncBatch,
// which must apply usage, timezone, source status, and the idempotency
// record as one atomic unit rather than as separate best-effort writes.
func applyUsageTx(ctx context.Context, tx *sql.Tx, now time.Time, userID, deviceID int64, rows []syncagg.DailyModel) error {
	for _, row := range rows {
		canonDevice := deviceID
		if row.SourceScope == syncagg.ScopeAccountGlobal {
			canonDevice = 0
		}
		var stored storedRow
		err := tx.QueryRowContext(ctx, `SELECT revision, miss, cache_read, cache_create, output, requests, user_turns, quality, derivation FROM usage_daily_model WHERE user_id=? AND source_scope=? AND device_id=? AND tool=? AND source_key_hash=? AND date=? AND vendor=? AND model=? FOR UPDATE`,
			userID, string(row.SourceScope), canonDevice, row.Tool, row.SourceKeyHash, row.Date, row.Vendor, row.Model,
		).Scan(&stored.Revision, &stored.Miss, &stored.CacheRead, &stored.CacheCreate, &stored.Output, &stored.Requests, &stored.UserTurns, &stored.Quality, &stored.Derivation)
		switch {
		case err == sql.ErrNoRows:
			if _, err := tx.ExecContext(ctx, `INSERT INTO usage_daily_model (user_id, device_id, source_scope, tool, source_key_hash, date, vendor, model, miss, cache_read, cache_create, output, requests, user_turns, quality, derivation, revision, writer_device_id, updated_at) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
				userID, canonDevice, string(row.SourceScope), row.Tool, row.SourceKeyHash, row.Date, row.Vendor, row.Model,
				row.Miss, row.CacheRead, row.CacheCreate, row.Output, row.Requests, row.UserTurns, row.Quality, row.Derivation, row.Revision, deviceID, now,
			); err != nil {
				return err
			}
		case err != nil:
			return err
		default:
			if row.Revision < stored.Revision {
				return ErrStaleRevision
			}
			if row.Revision == stored.Revision {
				// Equal revision must mean an identical retry (safe no-op)
				// or a genuine conflict — never a silent partial update.
				// Comparing only one column (historically `miss`) let a
				// same-revision retry with different cache/output/request/
				// quality/derivation values through unnoticed.
				if !stored.equalMeasures(row) {
					return ErrRevisionClash
				}
				continue
			}
			if _, err := tx.ExecContext(ctx, `UPDATE usage_daily_model SET miss=?, cache_read=?, cache_create=?, output=?, requests=?, user_turns=?, quality=?, derivation=?, revision=?, writer_device_id=?, updated_at=? WHERE user_id=? AND source_scope=? AND device_id=? AND tool=? AND source_key_hash=? AND date=? AND vendor=? AND model=?`,
				row.Miss, row.CacheRead, row.CacheCreate, row.Output, row.Requests, row.UserTurns, row.Quality, row.Derivation, row.Revision, deviceID, now,
				userID, string(row.SourceScope), canonDevice, row.Tool, row.SourceKeyHash, row.Date, row.Vendor, row.Model,
			); err != nil {
				return err
			}
		}
	}
	return nil
}

// SyncBatch applies one sync request as a single atomic unit: usage rows,
// the caller's timezone, per-source status, and the idempotency record
// either all commit together or none do. A prior version wrote these as
// separate best-effort statements outside any shared transaction and
// discarded the timezone/source-state errors, so a client could see HTTP 200
// while accounting rows committed but status metadata silently did not.
//
// Idempotency is keyed on the full request envelope (schema version, device,
// timezone, price catalog version, client version, sources, and usage rows),
// not only the usage rows, so replaying an identical retry after a changed
// source-status list is correctly treated as a *different* body rather than
// a no-op.
func (s *Store) SyncBatch(ctx context.Context, userID, deviceID int64, batch syncagg.Batch) error {
	body, err := json.Marshal(batch)
	if err != nil {
		return err
	}
	sum := sha256.Sum256(body)
	now := s.clock()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Claim the idempotency key before doing any accounting work. On MySQL/
	// InnoDB, a duplicate-key error from one statement does not abort the
	// surrounding transaction, so the losing side of a race can safely fall
	// through to compare hashes instead of double-applying usage.
	_, err = tx.ExecContext(ctx, `INSERT INTO sync_revisions (user_id, device_id, idempotency_key, body_hash, schema_version, created_at) VALUES (?,?,?,?,?,?)`,
		userID, deviceID, batch.IdempotencyKey, sum[:], batch.SchemaVersion, now)
	if isDuplicateKeyErr(err) {
		var existing []byte
		if scanErr := tx.QueryRowContext(ctx, `SELECT body_hash FROM sync_revisions WHERE device_id=? AND idempotency_key=?`, deviceID, batch.IdempotencyKey).Scan(&existing); scanErr != nil {
			return scanErr
		}
		if string(existing) != string(sum[:]) {
			return ErrIdempotency
		}
		return tx.Commit()
	}
	if err != nil {
		return err
	}

	if err := applyUsageTx(ctx, tx, now, userID, deviceID, batch.DailyModelUsage); err != nil {
		return err
	}
	if tz := strings.TrimSpace(batch.Timezone); tz != "" {
		if _, err := tx.ExecContext(ctx, `UPDATE users SET timezone=? WHERE id=?`, tz, userID); err != nil {
			return err
		}
	}
	for _, src := range batch.Sources {
		if _, err := tx.ExecContext(ctx, `INSERT INTO source_states (user_id, device_id, tool, source_scope, source_key_hash, status, quality, detected, updated_at) VALUES (?,?,?,?,?,?,?,?,?) ON DUPLICATE KEY UPDATE status=VALUES(status), quality=VALUES(quality), detected=VALUES(detected), updated_at=VALUES(updated_at)`,
			userID, deviceID, src.Tool, string(src.SourceScope), src.SourceKeyHash, src.Status, src.Quality, boolToInt(src.Detected), now); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func isDuplicateKeyErr(err error) bool {
	return mysqlError(err, 1062)
}

func boolToInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

func (s *Store) ListUsage(ctx context.Context, userID int64, from, to string) ([]syncagg.DailyModel, error) {
	q := `SELECT device_id, source_scope, tool, source_key_hash, date, vendor, model, miss, cache_read, cache_create, output, requests, user_turns, quality, derivation, revision FROM usage_daily_model WHERE user_id=? AND date>=? AND date<=?`
	rows, err := s.db.QueryContext(ctx, q, userID, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []syncagg.DailyModel
	for rows.Next() {
		var deviceID int64
		var date time.Time
		var row syncagg.DailyModel
		var scope string
		if err := rows.Scan(&deviceID, &scope, &row.Tool, &row.SourceKeyHash, &date, &row.Vendor, &row.Model, &row.Miss, &row.CacheRead, &row.CacheCreate, &row.Output, &row.Requests, &row.UserTurns, &row.Quality, &row.Derivation, &row.Revision); err != nil {
			return nil, err
		}
		row.SourceScope = syncagg.SourceScope(scope)
		row.Date = date.Format("2006-01-02")
		out = append(out, row)
	}
	return out, rows.Err()
}

func (s *Store) HasUsage(ctx context.Context, userID int64) (bool, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM usage_daily_model WHERE user_id=?`, userID).Scan(&n)
	return n > 0, err
}

func (s *Store) UserTimezone(ctx context.Context, userID int64) string {
	var tz string
	if err := s.db.QueryRowContext(ctx, `SELECT timezone FROM users WHERE id=?`, userID).Scan(&tz); err != nil || tz == "" {
		return "UTC"
	}
	return tz
}

func (s *Store) ListDevices(ctx context.Context, userID int64) ([]Device, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, user_id, public_id, label, os, arch, client_version, last_seen_at, last_sync_at, revoked_at FROM devices WHERE user_id=? ORDER BY created_at`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Device
	for rows.Next() {
		var d Device
		var seen, syncAt, rev sql.NullTime
		if err := rows.Scan(&d.ID, &d.UserID, &d.PublicID, &d.Label, &d.OS, &d.Arch, &d.ClientVersion, &seen, &syncAt, &rev); err != nil {
			return nil, err
		}
		d.LastSeenAt = seen.Time
		d.LastSyncAt = syncAt.Time
		d.Revoked = rev.Valid
		out = append(out, d)
	}
	return out, rows.Err()
}

func (s *Store) SumUsage(ctx context.Context, userID int64, from, to string) (map[string]int64, error) {
	q := `SELECT tool, SUM(miss+cache_read+cache_create+output) FROM usage_daily_model WHERE user_id=? AND date>=? AND date<=? GROUP BY tool`
	rows, err := s.db.QueryContext(ctx, q, userID, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]int64{}
	for rows.Next() {
		var tool string
		var total int64
		if err := rows.Scan(&tool, &total); err != nil {
			return nil, err
		}
		out[tool] = total
	}
	return out, rows.Err()
}

func (s *Store) DeviceByToken(ctx context.Context, raw string) (Device, User, error) {
	if raw == "" {
		return Device{}, User{}, ErrUnauthorized
	}
	hash := hashBytes([]byte(raw))
	var d Device
	var revoked sql.NullTime
	err := s.db.QueryRowContext(ctx, `SELECT id, user_id, public_id, label, os, arch, client_version, revoked_at FROM devices WHERE token_hash=?`, hash).Scan(
		&d.ID, &d.UserID, &d.PublicID, &d.Label, &d.OS, &d.Arch, &d.ClientVersion, &revoked,
	)
	if err == sql.ErrNoRows {
		return Device{}, User{}, ErrUnauthorized
	}
	if err != nil {
		return Device{}, User{}, err
	}
	if revoked.Valid {
		return Device{}, User{}, ErrUnauthorized
	}
	u, err := s.UserByID(ctx, d.UserID)
	if err != nil {
		return Device{}, User{}, ErrUnauthorized
	}
	_, _ = s.db.ExecContext(ctx, `UPDATE devices SET last_seen_at=? WHERE id=?`, s.clock(), d.ID)
	return d, u, nil
}

func (s *Store) TouchSync(ctx context.Context, deviceID int64) error {
	_, err := s.db.ExecContext(ctx, `UPDATE devices SET last_sync_at=?, last_seen_at=? WHERE id=?`, s.clock(), s.clock(), deviceID)
	return err
}

func (s *Store) RevokeDevice(ctx context.Context, userID int64, publicID string) error {
	res, err := s.db.ExecContext(ctx, `UPDATE devices SET revoked_at=? WHERE user_id=? AND public_id=? AND revoked_at IS NULL`, s.clock(), userID, publicID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) UserByID(ctx context.Context, id int64) (User, error) {
	var u User
	err := s.db.QueryRowContext(ctx, `SELECT id, public_id, github_id, github_login, avatar_url, profile_seed, source_hmac_key FROM users WHERE id=? AND deleted_at IS NULL`, id).Scan(
		&u.ID, &u.PublicID, &u.GitHubID, &u.Login, &u.AvatarURL, &u.ProfileSeed, &u.SourceHMACKey,
	)
	if err == sql.ErrNoRows {
		return User{}, ErrNotFound
	}
	return u, err
}

func (s *Store) PutOAuthState(ctx context.Context, stateHash, verifierHash []byte, exp time.Time) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO oauth_states (state_hash, code_verifier_hash, expires_at, created_at) VALUES (?,?,?,?)`,
		stateHash, verifierHash, exp, s.clock())
	return err
}

func (s *Store) TakeOAuthState(ctx context.Context, stateHash, verifierHash []byte) error {
	var stored []byte
	var exp time.Time
	err := s.db.QueryRowContext(ctx, `SELECT code_verifier_hash, expires_at FROM oauth_states WHERE state_hash=?`, stateHash).Scan(&stored, &exp)
	if err == sql.ErrNoRows {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if _, err := s.db.ExecContext(ctx, `DELETE FROM oauth_states WHERE state_hash=?`, stateHash); err != nil {
		return err
	}
	if !exp.After(s.clock()) || !hmacEqual(stored, verifierHash) {
		return ErrNotFound
	}
	return nil
}

type pairRow struct {
	SecretHash []byte
	Meta       DeviceMeta
	ExpiresAt  time.Time
	UserID     sql.NullInt64
	DeviceID   sql.NullInt64
	ConsumedAt sql.NullTime
	DeniedAt   sql.NullTime
}

type PairApproval struct {
	Device        Device
	UserLogin     string
	SourceHMACKey []byte
}

func (s *Store) CreatePairChallenge(ctx context.Context, code string, secretHash []byte, meta DeviceMeta, exp time.Time) error {
	raw, err := json.Marshal(meta)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `INSERT INTO pairing_challenges (display_code, secret_hash, device_meta, expires_at, created_at) VALUES (?,?,?,?,?)`,
		code, secretHash, raw, exp, s.clock())
	return err
}

func (s *Store) GetPairChallenge(ctx context.Context, code string) (pairRow, error) {
	var row pairRow
	var meta []byte
	err := s.db.QueryRowContext(ctx, `SELECT secret_hash, device_meta, expires_at, user_id, device_id, consumed_at, denied_at FROM pairing_challenges WHERE display_code=?`, code).Scan(
		&row.SecretHash, &meta, &row.ExpiresAt, &row.UserID, &row.DeviceID, &row.ConsumedAt, &row.DeniedAt,
	)
	if err == sql.ErrNoRows {
		return pairRow{}, ErrNotFound
	}
	if err != nil {
		return pairRow{}, err
	}
	_ = json.Unmarshal(meta, &row.Meta)
	return row, nil
}

func (s *Store) DenyPair(ctx context.Context, code string, userID int64) error {
	res, err := s.db.ExecContext(ctx, `UPDATE pairing_challenges SET denied_at=?, user_id=? WHERE display_code=? AND consumed_at IS NULL AND denied_at IS NULL`,
		s.clock(), userID, code)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) ApprovePair(ctx context.Context, code string, userID int64) (Device, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Device{}, err
	}
	defer tx.Rollback()

	var row pairRow
	var meta []byte
	err = tx.QueryRowContext(ctx, `SELECT secret_hash, device_meta, expires_at, user_id, device_id, consumed_at, denied_at FROM pairing_challenges WHERE display_code=? FOR UPDATE`, code).Scan(
		&row.SecretHash, &meta, &row.ExpiresAt, &row.UserID, &row.DeviceID, &row.ConsumedAt, &row.DeniedAt,
	)
	if err == sql.ErrNoRows {
		return Device{}, ErrNotFound
	}
	if err != nil {
		return Device{}, err
	}
	if err := json.Unmarshal(meta, &row.Meta); err != nil {
		return Device{}, err
	}
	if row.DeniedAt.Valid {
		return Device{}, ErrNotFound
	}
	// Retrieving an already-approved device must survive a dropped response
	// or restart even after the original 10-minute pairing window elapses;
	// only a *new* approval is gated by expiry.
	if row.ConsumedAt.Valid {
		if !row.UserID.Valid || row.UserID.Int64 != userID || !row.DeviceID.Valid {
			return Device{}, ErrUnauthorized
		}
		dev, err := deviceByIDTx(ctx, tx, row.DeviceID.Int64, userID)
		if err != nil {
			return Device{}, err
		}
		if err := tx.Commit(); err != nil {
			return Device{}, err
		}
		return dev, nil
	}
	if !row.ExpiresAt.After(s.clock()) {
		return Device{}, ErrNotFound
	}

	publicID, err := newUUID4()
	if err != nil {
		return Device{}, err
	}
	row.Meta = normalizeDeviceMeta(row.Meta)
	now := s.clock()
	res, err := tx.ExecContext(ctx, `INSERT INTO devices (user_id, public_id, label, os, arch, client_version, token_hash, token_version, created_at, last_seen_at) VALUES (?,?,?,?,?,?,?,1,?,?)`,
		userID, publicID, row.Meta.Label, row.Meta.OS, row.Meta.Arch, row.Meta.ClientVersion, row.SecretHash, now, now)
	if err != nil {
		return Device{}, err
	}
	deviceID, err := res.LastInsertId()
	if err != nil {
		return Device{}, err
	}
	updated, err := tx.ExecContext(ctx, `UPDATE pairing_challenges SET consumed_at=?, user_id=?, device_id=? WHERE display_code=? AND consumed_at IS NULL AND denied_at IS NULL`,
		now, userID, deviceID, code)
	if err != nil {
		return Device{}, err
	}
	n, err := updated.RowsAffected()
	if err != nil {
		return Device{}, err
	}
	if n != 1 {
		return Device{}, ErrNotFound
	}
	if err := tx.Commit(); err != nil {
		return Device{}, err
	}
	return Device{
		ID: deviceID, UserID: userID, PublicID: publicID, Label: row.Meta.Label,
		OS: row.Meta.OS, Arch: row.Meta.Arch, ClientVersion: row.Meta.ClientVersion,
	}, nil
}

func deviceByIDTx(ctx context.Context, tx *sql.Tx, deviceID, userID int64) (Device, error) {
	var d Device
	var lastSync, revoked sql.NullTime
	err := tx.QueryRowContext(ctx, `SELECT id, user_id, public_id, label, os, arch, client_version, last_seen_at, last_sync_at, revoked_at FROM devices WHERE id=? AND user_id=?`,
		deviceID, userID).Scan(
		&d.ID, &d.UserID, &d.PublicID, &d.Label, &d.OS, &d.Arch, &d.ClientVersion, &d.LastSeenAt, &lastSync, &revoked,
	)
	if err == sql.ErrNoRows {
		return Device{}, ErrNotFound
	}
	if err != nil {
		return Device{}, err
	}
	if lastSync.Valid {
		d.LastSyncAt = lastSync.Time
	}
	d.Revoked = revoked.Valid
	if d.Revoked {
		return Device{}, ErrUnauthorized
	}
	return d, nil
}

func (s *Store) GetPairApproval(ctx context.Context, code string) (PairApproval, error) {
	var out PairApproval
	var lastSync, revoked sql.NullTime
	err := s.db.QueryRowContext(ctx, `SELECT d.id, d.user_id, d.public_id, d.label, d.os, d.arch, d.client_version, d.last_seen_at, d.last_sync_at, d.revoked_at, u.github_login, u.source_hmac_key
FROM pairing_challenges p
JOIN devices d ON d.id=p.device_id
JOIN users u ON u.id=p.user_id AND u.deleted_at IS NULL
WHERE p.display_code=? AND p.consumed_at IS NOT NULL AND p.denied_at IS NULL`, code).Scan(
		&out.Device.ID, &out.Device.UserID, &out.Device.PublicID, &out.Device.Label, &out.Device.OS, &out.Device.Arch,
		&out.Device.ClientVersion, &out.Device.LastSeenAt, &lastSync, &revoked, &out.UserLogin, &out.SourceHMACKey,
	)
	if err == sql.ErrNoRows {
		return PairApproval{}, ErrNotFound
	}
	if err != nil {
		return PairApproval{}, err
	}
	if lastSync.Valid {
		out.Device.LastSyncAt = lastSync.Time
	}
	out.Device.Revoked = revoked.Valid
	if out.Device.Revoked {
		return PairApproval{}, ErrUnauthorized
	}
	return out, nil
}

func (s *Store) newDisplayCode(ctx context.Context) (string, error) {
	for i := 0; i < 16; i++ {
		b, err := randomBytes(8)
		if err != nil {
			return "", err
		}
		var code [8]byte
		alpha := pairAlphabet
		for j := 0; j < 8; j++ {
			code[j] = alpha[int(b[j])%len(alpha)]
		}
		cs := string(code[:])
		var n int
		if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM pairing_challenges WHERE display_code=?`, cs).Scan(&n); err != nil {
			return "", err
		}
		if n == 0 {
			return cs, nil
		}
	}
	return "", fmt.Errorf("display code exhausted")
}

func hmacEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	var v byte
	for i := range a {
		v |= a[i] ^ b[i]
	}
	return v == 0
}

func (s *Store) DeleteUsage(ctx context.Context, userID int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `DELETE FROM usage_daily_model WHERE user_id=?`, userID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM source_states WHERE user_id=?`, userID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM sync_revisions WHERE user_id=?`, userID); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) DeleteAccount(ctx context.Context, userID int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, q := range []string{
		`DELETE FROM usage_daily_model WHERE user_id=?`,
		`DELETE FROM source_states WHERE user_id=?`,
		`DELETE FROM sync_revisions WHERE user_id=?`,
		`DELETE FROM pairing_challenges WHERE user_id=?`,
		`DELETE FROM web_sessions WHERE user_id=?`,
		`DELETE FROM devices WHERE user_id=?`,
		`DELETE FROM users WHERE id=?`,
	} {
		if _, err := tx.ExecContext(ctx, q, userID); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) Ping(ctx context.Context) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("store unavailable")
	}
	return s.db.PingContext(ctx)
}
