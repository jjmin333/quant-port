CREATE TABLE market_data_settings (
  id TINYINT UNSIGNED NOT NULL PRIMARY KEY DEFAULT 1,
  rankings_poll_realtime_minutes INT NOT NULL DEFAULT 1,
  rankings_poll_short_minutes INT NOT NULL DEFAULT 15,
  rankings_poll_long_minutes INT NOT NULL DEFAULT 60,
  quote_overlay_poll_seconds INT NOT NULL DEFAULT 2,
  rankings_client_poll_seconds INT NOT NULL DEFAULT 60,
  shares_poll_hours INT NOT NULL DEFAULT 24,
  fundamentals_poll_hours INT NOT NULL DEFAULT 24,
  updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
);

INSERT INTO market_data_settings (id) VALUES (1);
