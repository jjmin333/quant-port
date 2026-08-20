INSERT INTO news_poll_keywords (market, keyword, enabled, sort_order) VALUES
('domestic', '비트코인', 1, 210),
('domestic', '가상자산', 1, 220),
('domestic', 'VIX', 1, 230),
('domestic', '금값', 1, 240),
('domestic', '나스닥', 1, 250),
('domestic', 'S&P 500', 1, 260),
('domestic', '다우', 1, 270),
('domestic', '반도체 지수', 1, 280),
('us', 'VIX', 1, 210),
('us', 'SOX', 1, 220),
('us', 'Bitcoin', 1, 230),
('us', 'gold price', 1, 240),
('us', 'Nasdaq 100', 1, 250),
('us', 'Treasury yield', 1, 260),
('us', 'semiconductor index', 1, 270)
ON DUPLICATE KEY UPDATE enabled = VALUES(enabled);
