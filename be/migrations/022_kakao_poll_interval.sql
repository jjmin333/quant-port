ALTER TABLE news_poll_settings
  ADD COLUMN domestic_kakao_poll_minutes INT NOT NULL DEFAULT 6 AFTER domestic_kakao_keywords_per_tick;

UPDATE news_poll_settings
SET domestic_kakao_keywords_per_tick = 10,
    domestic_kakao_poll_minutes = 6
WHERE id = 1;
