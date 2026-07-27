-- Cached USD/KRW exchange rate (refreshed by backend poller).

USE quantport;

CREATE TABLE IF NOT EXISTS market_fx_rates (
  base_currency  CHAR(3) NOT NULL,
  quote_currency CHAR(3) NOT NULL,
  mid_rate       DECIMAL(14, 4) NOT NULL,
  buy_rate       DECIMAL(14, 4) NOT NULL,
  updated_at     TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (base_currency, quote_currency)
) ENGINE=InnoDB;
