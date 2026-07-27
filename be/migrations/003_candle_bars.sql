CREATE TABLE IF NOT EXISTS candle_bars (
  symbol     VARCHAR(32)  NOT NULL,
  bar_interval VARCHAR(8) NOT NULL,
  bar_time   VARCHAR(32)  NOT NULL,
  open_price DOUBLE       NOT NULL,
  high_price DOUBLE       NOT NULL,
  low_price  DOUBLE       NOT NULL,
  close_price DOUBLE      NOT NULL,
  volume     BIGINT       NOT NULL,
  created_at TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (symbol, bar_interval, bar_time),
  KEY idx_candle_bars_symbol_interval_time (symbol, bar_interval, bar_time DESC)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
