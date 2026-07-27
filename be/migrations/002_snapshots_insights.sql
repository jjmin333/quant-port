CREATE TABLE IF NOT EXISTS portfolio_snapshots (
  id               BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
  recorded_at      TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  total_assets_krw DECIMAL(20, 2) NOT NULL,
  total_cash_krw   DECIMAL(20, 2) NOT NULL,
  total_profit_krw DECIMAL(20, 2) NOT NULL,
  exchange_rate    DECIMAL(12, 4) NOT NULL DEFAULT 0,
  INDEX idx_portfolio_snapshots_recorded (recorded_at)
) ENGINE=InnoDB;
