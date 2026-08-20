-- Historical: per-user Toss-linked portfolio cache.
-- Dropped by 054_drop_broker_linking.sql. Do not re-apply on existing DBs
-- (removed from extendedMigrations so a missing table is not recreated).

USE quantport;

CREATE TABLE IF NOT EXISTS user_portfolio_cache (
  user_id       BIGINT UNSIGNED NOT NULL PRIMARY KEY,
  summary_json  JSON NOT NULL,
  symbols_json  JSON NOT NULL,
  synced_at     TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  CONSTRAINT fk_portfolio_cache_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
) ENGINE=InnoDB;
