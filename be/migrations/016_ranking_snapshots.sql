-- Ranking snapshot rows for intraday rank trajectory (Market Radar Phase 2).

USE quantport;

CREATE TABLE IF NOT EXISTS ranking_snapshot_entries (
  id              BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
  market_country  VARCHAR(8)      NOT NULL,
  ranking_type    VARCHAR(64)     NOT NULL,
  duration        VARCHAR(16)     NOT NULL,
  ranked_at       TIMESTAMP(3)    NOT NULL,
  symbol          VARCHAR(32)     NOT NULL,
  rank_pos        INT             NOT NULL,
  change_rate     DECIMAL(12, 6)  NULL,
  created_at      TIMESTAMP(3)    NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  INDEX idx_ranking_lookup (market_country, ranking_type, duration, ranked_at),
  INDEX idx_symbol_history (symbol, market_country, ranking_type, duration, ranked_at)
) ENGINE=InnoDB;
