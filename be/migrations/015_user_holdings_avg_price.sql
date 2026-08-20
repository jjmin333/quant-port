ALTER TABLE user_holdings
  ADD COLUMN avg_price DECIMAL(18, 4) NULL AFTER quantity,
  ADD COLUMN cost_currency VARCHAR(3) NULL AFTER avg_price;
