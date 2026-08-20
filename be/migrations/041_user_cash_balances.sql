-- Manual portfolio cash (KRW + USD), separate from Toss-linked buying power.
CREATE TABLE IF NOT EXISTS user_cash_balances (
  user_id BIGINT UNSIGNED NOT NULL,
  currency CHAR(3) NOT NULL,
  amount DECIMAL(18,4) NOT NULL DEFAULT 0,
  updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (user_id, currency),
  CONSTRAINT fk_user_cash_balances_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);
