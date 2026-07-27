CREATE TABLE IF NOT EXISTS stock_fundamentals (
  symbol                VARCHAR(32)   NOT NULL PRIMARY KEY,
  data_source           VARCHAR(16)   NOT NULL COMMENT 'dart | edgar',
  fiscal_year           INT           NULL,
  fiscal_period         VARCHAR(8)    NULL COMMENT 'FY, Q1..Q4',
  report_code           VARCHAR(8)    NULL COMMENT 'DART reprt_code when domestic',
  period_end_date       DATE          NULL,
  filed_at              DATE          NULL,

  revenue               BIGINT        NULL,
  operating_income      BIGINT        NULL,
  net_income            BIGINT        NULL,
  total_assets          BIGINT        NULL,
  total_equity          BIGINT        NULL,
  total_liabilities     BIGINT        NULL,
  current_assets        BIGINT        NULL,
  current_liabilities   BIGINT        NULL,
  eps_basic             DECIMAL(20,6) NULL,
  eps_diluted           DECIMAL(20,6) NULL,
  shares_outstanding    BIGINT        NULL,

  roe                   DECIMAL(16,4) NULL COMMENT 'percent',
  operating_margin      DECIMAL(16,4) NULL,
  revenue_growth_yoy    DECIMAL(16,4) NULL,
  eps_growth_yoy        DECIMAL(16,4) NULL,
  fwd_per               DECIMAL(16,4) NULL COMMENT 'reserved: future analyst API',
  cy_per                DECIMAL(16,4) NULL COMMENT 'reserved: future analyst API',
  debt_ratio            DECIMAL(16,4) NULL,
  roa                   DECIMAL(16,4) NULL,
  net_profit_margin     DECIMAL(16,4) NULL,
  current_ratio         DECIMAL(16,4) NULL,

  calc_status           VARCHAR(16)   NOT NULL DEFAULT 'pending' COMMENT 'ok | partial | failed',
  calc_error            TEXT          NULL,
  fetched_at            TIMESTAMP     NULL,
  created_at            TIMESTAMP     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at            TIMESTAMP     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,

  CONSTRAINT fk_stock_fundamentals_v2_symbol
    FOREIGN KEY (symbol) REFERENCES stocks(symbol)
      ON DELETE CASCADE ON UPDATE CASCADE,
  INDEX idx_stock_fundamentals_source (data_source),
  INDEX idx_stock_fundamentals_status (calc_status),
  INDEX idx_stock_fundamentals_fetched (fetched_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
