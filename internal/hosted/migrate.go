package hosted

import "context"

var migrations = []string{
	`CREATE TABLE IF NOT EXISTS users (
  id BIGINT NOT NULL AUTO_INCREMENT,
  public_id CHAR(36) NOT NULL,
  github_id BIGINT NOT NULL,
  github_login VARCHAR(255) NOT NULL,
  avatar_url VARCHAR(1024) NOT NULL DEFAULT '',
  profile_seed CHAR(36) NOT NULL,
  source_hmac_key BINARY(32) NOT NULL,
  timezone VARCHAR(64) NOT NULL DEFAULT 'UTC',
  created_at DATETIME NOT NULL,
  updated_at DATETIME NOT NULL,
  deleted_at DATETIME NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uq_users_public_id (public_id),
  UNIQUE KEY uq_users_github_id (github_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
	`CREATE TABLE IF NOT EXISTS web_sessions (
  id BIGINT NOT NULL AUTO_INCREMENT,
  user_id BIGINT NOT NULL,
  token_hash BINARY(32) NOT NULL,
  csrf_hash BINARY(32) NOT NULL,
  expires_at DATETIME NOT NULL,
  rotated_from BIGINT NULL,
  created_at DATETIME NOT NULL,
  last_seen_at DATETIME NOT NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uq_sessions_token (token_hash),
  KEY idx_sessions_user (user_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
	`CREATE TABLE IF NOT EXISTS oauth_states (
  state_hash BINARY(32) NOT NULL,
  code_verifier_hash BINARY(32) NOT NULL,
  expires_at DATETIME NOT NULL,
  created_at DATETIME NOT NULL,
  PRIMARY KEY (state_hash)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
	`CREATE TABLE IF NOT EXISTS devices (
  id BIGINT NOT NULL AUTO_INCREMENT,
  user_id BIGINT NOT NULL,
  public_id CHAR(36) NOT NULL,
  label VARCHAR(128) NOT NULL,
  os VARCHAR(32) NOT NULL,
  arch VARCHAR(32) NOT NULL,
  client_version VARCHAR(64) NOT NULL,
  token_hash BINARY(32) NOT NULL,
  token_version INT NOT NULL DEFAULT 1,
  created_at DATETIME NOT NULL,
  last_seen_at DATETIME NOT NULL,
  last_sync_at DATETIME NULL,
  revoked_at DATETIME NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uq_devices_public_id (public_id),
  UNIQUE KEY uq_devices_token (token_hash),
  KEY idx_devices_user (user_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
	`CREATE TABLE IF NOT EXISTS pairing_challenges (
  id BIGINT NOT NULL AUTO_INCREMENT,
  display_code CHAR(8) NOT NULL,
  secret_hash BINARY(32) NOT NULL,
  user_id BIGINT NULL,
  device_meta JSON NOT NULL,
  expires_at DATETIME NOT NULL,
  consumed_at DATETIME NULL,
  denied_at DATETIME NULL,
  created_ip_hash BINARY(32) NULL,
  created_at DATETIME NOT NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uq_pair_code (display_code)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
	`CREATE TABLE IF NOT EXISTS usage_daily_model (
  user_id BIGINT NOT NULL,
  device_id BIGINT NOT NULL,
  source_scope VARCHAR(32) NOT NULL,
  tool VARCHAR(32) NOT NULL,
  source_key_hash CHAR(64) NOT NULL,
  date DATE NOT NULL,
  vendor VARCHAR(32) NOT NULL,
  model VARCHAR(128) NOT NULL,
  miss BIGINT NOT NULL DEFAULT 0,
  cache_read BIGINT NOT NULL DEFAULT 0,
  cache_create BIGINT NOT NULL DEFAULT 0,
  output BIGINT NOT NULL DEFAULT 0,
  requests BIGINT NOT NULL DEFAULT 0,
  user_turns BIGINT NOT NULL DEFAULT 0,
  quality VARCHAR(32) NOT NULL DEFAULT '',
  derivation VARCHAR(64) NOT NULL DEFAULT '',
  revision BIGINT NOT NULL,
  writer_device_id BIGINT NULL,
  updated_at DATETIME NOT NULL,
  PRIMARY KEY (user_id, source_scope, device_id, tool, source_key_hash, date, vendor, model)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
	`CREATE TABLE IF NOT EXISTS source_states (
  user_id BIGINT NOT NULL,
  device_id BIGINT NOT NULL,
  tool VARCHAR(32) NOT NULL,
  source_scope VARCHAR(32) NOT NULL,
  source_key_hash CHAR(64) NOT NULL,
  status VARCHAR(32) NOT NULL,
  quality VARCHAR(32) NOT NULL DEFAULT '',
  detected TINYINT(1) NOT NULL DEFAULT 0,
  updated_at DATETIME NOT NULL,
  PRIMARY KEY (user_id, device_id, tool, source_scope, source_key_hash)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
	`CREATE TABLE IF NOT EXISTS sync_revisions (
  user_id BIGINT NOT NULL,
  device_id BIGINT NOT NULL,
  idempotency_key CHAR(64) NOT NULL,
  body_hash BINARY(32) NOT NULL,
  schema_version INT NOT NULL,
  created_at DATETIME NOT NULL,
  PRIMARY KEY (device_id, idempotency_key)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
	`ALTER TABLE devices MODIFY client_version VARCHAR(64) NOT NULL`,
	`CREATE TABLE IF NOT EXISTS public_projections (
  user_id BIGINT NOT NULL,
  snapshot_json MEDIUMTEXT NOT NULL,
  snapshot_id VARCHAR(80) NOT NULL,
  total_tokens BIGINT NOT NULL DEFAULT 0,
  updated_at DATETIME NOT NULL,
  desired_snapshot_id VARCHAR(80) NOT NULL DEFAULT '',
  readme_status VARCHAR(32) NOT NULL DEFAULT '',
  readme_last_error VARCHAR(40) NOT NULL DEFAULT '',
  pending_due_at DATETIME NULL,
  pending_reason VARCHAR(32) NOT NULL DEFAULT '',
  publish_lease_until DATETIME NULL,
  publish_lease_token CHAR(36) NULL,
  publish_retry_count INT NOT NULL DEFAULT 0,
  PRIMARY KEY (user_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
	`CREATE TABLE IF NOT EXISTS public_presentations (
  user_id BIGINT NOT NULL,
  palette VARCHAR(32) NOT NULL,
  revision VARCHAR(80) NOT NULL,
  asset_revision VARCHAR(80) NOT NULL,
  preview_light MEDIUMBLOB NOT NULL,
  preview_dark MEDIUMBLOB NOT NULL,
  readme_cache_key VARCHAR(160) NOT NULL DEFAULT '',
  readme_snapshot_id VARCHAR(80) NOT NULL DEFAULT '',
  readme_materialized_at DATETIME NULL,
  verified_snapshot_id VARCHAR(80) NULL,
  verified_total_tokens BIGINT NULL,
  verified_as_of_date VARCHAR(10) NULL,
  updated_at DATETIME NOT NULL,
  PRIMARY KEY (user_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
	`CREATE TABLE IF NOT EXISTS publish_jobs (
  id CHAR(36) NOT NULL,
  user_id BIGINT NOT NULL,
  kind VARCHAR(16) NOT NULL,
  palette VARCHAR(32) NOT NULL,
  phase VARCHAR(32) NOT NULL,
  snapshot_id VARCHAR(80) NOT NULL,
  asset_revision VARCHAR(80) NOT NULL DEFAULT '',
  cache_key VARCHAR(160) NOT NULL DEFAULT '',
  error_text VARCHAR(400) NOT NULL DEFAULT '',
  result_code VARCHAR(40) NOT NULL DEFAULT '',
  created_at DATETIME NOT NULL,
  updated_at DATETIME NOT NULL,
  PRIMARY KEY (id),
  KEY idx_publish_jobs_user (user_id, created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
	`CREATE TABLE IF NOT EXISTS profile_sessions (
  id BIGINT NOT NULL AUTO_INCREMENT,
  user_id BIGINT NOT NULL,
  token_hash BINARY(32) NOT NULL,
  csrf_hash BINARY(32) NOT NULL,
  expires_at DATETIME NOT NULL,
  created_at DATETIME NOT NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uq_profile_sessions_token (token_hash),
  KEY idx_profile_sessions_user (user_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
	`CREATE TABLE IF NOT EXISTS profile_exchange_codes (
  code_hash BINARY(32) NOT NULL,
  user_id BIGINT NOT NULL,
  expires_at DATETIME NOT NULL,
  consumed_at DATETIME NULL,
  created_at DATETIME NOT NULL,
  PRIMARY KEY (code_hash)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
}

func (s *Store) Migrate(ctx context.Context) error {
	for _, q := range migrations {
		if _, err := s.db.ExecContext(ctx, q); err != nil {
			return err
		}
	}
	if err := s.ensureColumns(ctx); err != nil {
		return err
	}
	return s.backfillVerifiedIdentity(ctx)
}

// ensureColumns adds the README cursor to databases created before it.
// CREATE TABLE IF NOT EXISTS does not alter an existing table.
func (s *Store) ensureColumns(ctx context.Context) error {
	cols := []struct{ table, name, def string }{
		{"public_projections", "desired_snapshot_id", "VARCHAR(80) NOT NULL DEFAULT ''"},
		{"public_projections", "readme_status", "VARCHAR(32) NOT NULL DEFAULT ''"},
		{"public_projections", "readme_last_error", "VARCHAR(40) NOT NULL DEFAULT ''"},
		{"public_projections", "pending_due_at", "DATETIME NULL"},
		{"public_projections", "pending_reason", "VARCHAR(32) NOT NULL DEFAULT ''"},
		{"public_projections", "publish_lease_until", "DATETIME NULL"},
		{"public_projections", "publish_lease_token", "CHAR(36) NULL"},
		{"public_projections", "publish_retry_count", "INT NOT NULL DEFAULT 0"},
		{"public_presentations", "verified_snapshot_id", "VARCHAR(80) NULL"},
		{"public_presentations", "verified_total_tokens", "BIGINT NULL"},
		{"public_presentations", "verified_as_of_date", "VARCHAR(10) NULL"},
	}
	for _, c := range cols {
		var n int
		err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ? AND COLUMN_NAME = ?`, c.table, c.name).Scan(&n)
		if err != nil {
			return err
		}
		if n > 0 {
			continue
		}
		if _, err := s.db.ExecContext(ctx, "ALTER TABLE "+c.table+" ADD COLUMN "+c.name+" "+c.def); err != nil {
			return err
		}
	}
	return nil
}
