-- Daily closing prices for stocks and market indices, plus the per-date LLM
-- analysis shown alongside them (see docs/DAILY_CLOSE_DESIGN.md).
--
-- These are read-optimised projections, not a second source of truth: closes
-- are derived one-way from candle_bars / the index poller by an idempotent
-- upsert keyed on (symbol|index_id, trade_date), so a re-run can only restate
-- the same numbers.

USE quantport;

CREATE TABLE IF NOT EXISTS stock_daily_closes (
  symbol           VARCHAR(32)  NOT NULL,
  trade_date       DATE         NOT NULL,
  close_price      DOUBLE       NOT NULL,
  prev_close       DOUBLE       NULL,
  change_amount    DOUBLE       NULL,
  change_rate      DOUBLE       NULL,
  volume           BIGINT       NOT NULL DEFAULT 0,
  currency         CHAR(3)      NOT NULL DEFAULT 'KRW',
  -- NULL when no analysis ran for this symbol/date. The UI renders "-".
  -- (Keep semicolons out of comments: the migration splitter cuts on them.)
  score            TINYINT      NULL,
  -- '' when no analysis ran, never NULL, so readers can print it directly.
  ai_summary       TEXT         NOT NULL,
  analysis_source  VARCHAR(8)   NOT NULL DEFAULT '',
  analyzed_at      TIMESTAMP(3) NULL,
  source           VARCHAR(16)  NOT NULL DEFAULT 'candle_bars',
  created_at       TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at       TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (symbol, trade_date),
  KEY idx_stock_daily_closes_date (trade_date)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS index_daily_closes (
  index_id      VARCHAR(32)    NOT NULL,
  trade_date    DATE           NOT NULL,
  close_value   DECIMAL(20, 4) NOT NULL,
  prev_close    DECIMAL(20, 4) NULL,
  change_amount DECIMAL(20, 4) NULL,
  change_rate   DOUBLE         NULL,
  currency      CHAR(3)        NOT NULL DEFAULT 'KRW',
  source        VARCHAR(16)    NOT NULL DEFAULT '',
  created_at    TIMESTAMP(3)   NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at    TIMESTAMP(3)   NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (index_id, trade_date),
  KEY idx_index_daily_closes_date (trade_date)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
