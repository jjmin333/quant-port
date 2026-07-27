-- Cached market overview indices (refreshed by backend poller within API quotas).

USE quantport;

CREATE TABLE IF NOT EXISTS market_indices (
  index_id       VARCHAR(32)  NOT NULL PRIMARY KEY,
  name           VARCHAR(64)  NOT NULL,
  currency       CHAR(3)      NOT NULL,
  value_amount   DECIMAL(20, 4) NOT NULL,
  change_amount  DECIMAL(20, 4) NOT NULL DEFAULT 0,
  change_rate    DECIMAL(12, 6) NOT NULL DEFAULT 0,
  badge          VARCHAR(16)  NULL,
  sparkline      JSON         NULL,
  source         VARCHAR(16)  NOT NULL,
  updated_at     TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3)
) ENGINE=InnoDB;
