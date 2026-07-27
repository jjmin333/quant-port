-- Durable last-good quote terminal data (detail, orderbook, trades, list prices).
-- Redis = hot cache. MySQL = source of truth after successful Toss fetch.
USE quantport;

CREATE TABLE IF NOT EXISTS quote_snapshots (
  symbol      VARCHAR(32)  NOT NULL PRIMARY KEY,
  detail_json JSON         NOT NULL,
  source      VARCHAR(16)  NOT NULL DEFAULT 'toss',
  updated_at  TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS orderbook_snapshots (
  symbol      VARCHAR(32) NOT NULL PRIMARY KEY,
  book_json   JSON        NOT NULL,
  updated_at  TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS trade_ticks (
  symbol     VARCHAR(32)  NOT NULL,
  trade_id   VARCHAR(128) NOT NULL,
  trade_time VARCHAR(64)  NOT NULL,
  price      DOUBLE       NOT NULL,
  quantity   BIGINT       NOT NULL,
  side       VARCHAR(16)  NOT NULL DEFAULT 'unknown',
  PRIMARY KEY (symbol, trade_id),
  KEY idx_trade_ticks_symbol_time (symbol, trade_time DESC)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS symbol_last_prices (
  symbol     VARCHAR(32) NOT NULL PRIMARY KEY,
  last_price DOUBLE      NOT NULL,
  currency   CHAR(3)     NOT NULL DEFAULT 'KRW',
  updated_at TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
