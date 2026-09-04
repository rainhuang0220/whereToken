package hosted

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
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
	err := s.db.QueryRowContext(ctx, `SELECT id, user_id, expires_at FROM web_sessions WHERE token_hash=?`, hash).Scan(&sess.ID, &sess.UserID, &sess.ExpiresAt)
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
	if meta.Label == "" {
		meta.Label = meta.OS + " " + meta.Arch
	}
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

func (s *Store) ApplyUsage(ctx context.Context, userID, deviceID int64, rows []syncagg.DailyModel) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	now := s.clock()
	for _, row := range rows {
		canonDevice := deviceID
		if row.SourceScope == syncagg.ScopeAccountGlobal {
			canonDevice = 0
		}
		var storedRev, storedMiss sql.NullInt64
		err := tx.QueryRowContext(ctx, `SELECT revision, miss FROM usage_daily_model WHERE user_id=? AND source_scope=? AND device_id=? AND tool=? AND source_key_hash=? AND date=? AND vendor=? AND model=?`,
			userID, string(row.SourceScope), canonDevice, row.Tool, row.SourceKeyHash, row.Date, row.Vendor, row.Model,
		).Scan(&storedRev, &storedMiss)
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
			if row.Revision < storedRev.Int64 {
				return ErrStaleRevision
			}
			if row.Revision == storedRev.Int64 {
				if row.Miss != storedMiss.Int64 {
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
	return tx.Commit()
}

func (s *Store) SyncBatch(ctx context.Context, userID, deviceID int64, batch syncagg.Batch) error {
	body, err := json.Marshal(batch.DailyModelUsage)
	if err != nil {
		return err
	}
	sum := sha256.Sum256(body)
	var existing []byte
	err = s.db.QueryRowContext(ctx, `SELECT body_hash FROM sync_revisions WHERE device_id=? AND idempotency_key=?`, deviceID, batch.IdempotencyKey).Scan(&existing)
	if err == nil {
		if string(existing) != string(sum[:]) {
			return ErrIdempotency
		}
		return nil
	}
	if err != sql.ErrNoRows {
		return err
	}
	if err := s.ApplyUsage(ctx, userID, deviceID, batch.DailyModelUsage); err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `INSERT INTO sync_revisions (user_id, device_id, idempotency_key, body_hash, schema_version, created_at) VALUES (?,?,?,?,?,?)`,
		userID, deviceID, batch.IdempotencyKey, sum[:], batch.SchemaVersion, s.clock())
	return err
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

func (s *Store) Ping(ctx context.Context) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("store unavailable")
	}
	return s.db.PingContext(ctx)
}
