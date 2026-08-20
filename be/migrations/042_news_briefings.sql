-- AI 뉴스 브리핑 (국내주식·해외주식 뉴스 화면 상단 카드).
--
-- 한 행이 한 번의 생성 결과다. 최신 한 건만 화면에 쓰지만, 다음 생성의 프롬프트가
-- 직전 10건을 되읽으므로 이력을 남긴다.
CREATE TABLE IF NOT EXISTS news_briefings (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  market VARCHAR(16) NOT NULL,
  generated_at TIMESTAMP NOT NULL,
  source VARCHAR(16) NOT NULL DEFAULT 'rules',
  mood VARCHAR(16) NOT NULL DEFAULT '',
  summary MEDIUMTEXT NOT NULL,
  -- payload는 mood_reason·highlights·watch_points·facts를 한 덩어리로 담는다.
  -- 화면이 통째로 읽고 통째로 쓰기 때문에 컬럼으로 쪼개서 얻을 게 없다.
  payload JSON NOT NULL,
  article_count INT NOT NULL DEFAULT 0,
  latest_published_at TIMESTAMP NULL DEFAULT NULL,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  KEY idx_news_briefings_market_time (market, generated_at DESC)
);
