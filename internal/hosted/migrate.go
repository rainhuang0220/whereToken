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
  client_version VARCHAR(32) NOT NULL,
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
}

func (s *Store) Migrate(ctx context.Context) error {
	for _, q := range migrations {
		if _, err := s.db.ExecContext(ctx, q); err != nil {
			return err
		}
	}
	return nil
}
