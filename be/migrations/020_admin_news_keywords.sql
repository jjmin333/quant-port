ALTER TABLE users ADD COLUMN role VARCHAR(16) NOT NULL DEFAULT 'user' AFTER display_name;

CREATE INDEX idx_users_role ON users (role);

UPDATE users SET role = 'admin' WHERE email = 'jjmin321@naver.com';

CREATE TABLE news_poll_keywords (
  id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  market VARCHAR(16) NOT NULL,
  keyword VARCHAR(128) NOT NULL,
  enabled TINYINT(1) NOT NULL DEFAULT 1,
  sort_order INT NOT NULL DEFAULT 0,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  UNIQUE KEY uk_market_keyword (market, keyword),
  KEY idx_market_enabled (market, enabled, sort_order)
);

INSERT INTO news_poll_keywords (market, keyword, enabled, sort_order) VALUES
('domestic', '코스피', 1, 10),
('domestic', '코스닥', 1, 20),
('domestic', '증시', 1, 30),
('domestic', '반도체', 1, 40),
('domestic', '원달러', 1, 50),
('domestic', '환율', 1, 60),
('domestic', '한국은행', 1, 70),
('domestic', '기준금리', 1, 80),
('domestic', '금리', 1, 90),
('domestic', '엔비디아', 1, 100),
('domestic', '2차전지', 1, 110),
('domestic', '배당', 1, 120),
('domestic', '실적', 1, 130),
('domestic', '공매도', 1, 140),
('domestic', 'IPO', 1, 150),
('domestic', 'ETF', 1, 160),
('domestic', '채권', 1, 170),
('domestic', '유가', 1, 180),
('us', 'S&P 500', 1, 10),
('us', 'Nasdaq', 1, 20),
('us', 'Dow Jones', 1, 30),
('us', 'Wall Street', 1, 40),
('us', 'Federal Reserve', 1, 50),
('us', 'Fed interest rates', 1, 60),
('us', 'stock market', 1, 70),
('us', 'earnings', 1, 80),
('us', 'crude oil', 1, 90),
('us', 'oil price', 1, 100),
('us', 'OPEC', 1, 110),
('us', 'Iran sanctions', 1, 120),
('us', 'Middle East oil', 1, 130),
('us', 'inflation CPI', 1, 140),
('us', 'tariff trade war', 1, 150),
('us', 'semiconductor stocks', 1, 160),
('us', 'Nvidia stock', 1, 170),
('us', 'Tesla stock', 1, 180),
('us', 'IPO', 1, 190),
('us', 'SEC regulation', 1, 200);
