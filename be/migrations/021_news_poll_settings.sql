CREATE TABLE news_poll_settings (
  id TINYINT UNSIGNED NOT NULL PRIMARY KEY DEFAULT 1,
  domestic_naver_keywords_per_tick INT NOT NULL DEFAULT 18,
  domestic_kakao_keywords_per_tick INT NOT NULL DEFAULT 4,
  us_google_keywords_per_tick INT NOT NULL DEFAULT 8,
  updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
);

INSERT INTO news_poll_settings (id) VALUES (1);
