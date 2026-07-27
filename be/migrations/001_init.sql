-- Quant Port initial schema
-- Applied automatically by BE when required tables are missing (see repository/migrate.go).
-- First deploy: CREATE DATABASE runs here if the database does not exist yet.

CREATE DATABASE IF NOT EXISTS quantport
  CHARACTER SET utf8mb4
  COLLATE utf8mb4_unicode_ci;

USE quantport;

CREATE TABLE IF NOT EXISTS users (
  id            BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
  email         VARCHAR(255) NOT NULL UNIQUE,
  display_name  VARCHAR(100) NOT NULL,
  created_at    TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at    TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
) ENGINE=InnoDB;

CREATE TABLE IF NOT EXISTS broker_accounts (
  id            BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
  user_id       BIGINT UNSIGNED NOT NULL,
  broker_code   VARCHAR(32) NOT NULL,
  account_name  VARCHAR(100) NOT NULL,
  created_at    TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  INDEX idx_broker_accounts_user (user_id)
) ENGINE=InnoDB;

CREATE TABLE IF NOT EXISTS watchlist_items (
  id            BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
  user_id       BIGINT UNSIGNED NOT NULL,
  symbol        VARCHAR(32) NOT NULL,
  created_at    TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  UNIQUE KEY uk_watchlist_user_symbol (user_id, symbol)
) ENGINE=InnoDB;
