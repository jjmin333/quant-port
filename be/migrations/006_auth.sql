-- Auth, per-user Toss credentials, refresh tokens

USE quantport;

CREATE TABLE IF NOT EXISTS user_broker_credentials (
  id                  BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
  user_id             BIGINT UNSIGNED NOT NULL,
  broker_code         VARCHAR(32) NOT NULL DEFAULT 'toss',
  client_id_enc       VARBINARY(1024) NOT NULL,
  client_secret_enc   VARBINARY(1024) NOT NULL,
  client_id_hint      VARCHAR(32) NOT NULL DEFAULT '',
  is_active           TINYINT(1) NOT NULL DEFAULT 1,
  linked_at           TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at          TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  UNIQUE KEY uk_user_broker (user_id, broker_code),
  INDEX idx_user_broker_user (user_id),
  CONSTRAINT fk_user_broker_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
) ENGINE=InnoDB;

CREATE TABLE IF NOT EXISTS user_refresh_tokens (
  id          BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
  user_id     BIGINT UNSIGNED NOT NULL,
  token_hash  CHAR(64) NOT NULL,
  expires_at  TIMESTAMP NOT NULL,
  created_at  TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  revoked_at  TIMESTAMP NULL,
  user_agent  VARCHAR(255) NULL,
  UNIQUE KEY uk_refresh_token_hash (token_hash),
  INDEX idx_refresh_user (user_id),
  INDEX idx_refresh_expires (expires_at),
  CONSTRAINT fk_refresh_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
) ENGINE=InnoDB;
