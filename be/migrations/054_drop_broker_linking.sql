-- Remove Toss *account linking* tables. The global Toss Open API client for
-- quotes/candles/rankings/warnings is unchanged.
-- user_holdings / holding_trades / user_cash_balances stay (manual/OCR).

DROP TABLE IF EXISTS user_portfolio_cache;
DROP TABLE IF EXISTS user_broker_credentials;
DROP TABLE IF EXISTS broker_accounts;
DROP TABLE IF EXISTS portfolio_snapshots;
