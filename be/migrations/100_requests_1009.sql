-- 2026-10-09 사용자 요청 (docs/mockups/requests-1009-v1.html)
--
-- ③ 오늘의 변화 — 시가총액 순위
--   stock_change_events.market_cap: 감지한 날 점수 행의 inputs.marketCap(종가 x 주식 수, 원/달러).
--   목록이 이 값의 내림차순이고, 화면의 순위 번호는 그 순번이다.
--   지난 행은 같은 날 점수 행에서 채운다(10-09 실측: 11일치 6,190행 전부 짝이 있다).
--
-- ④ 뉴스 카테고리
--   news_articles.topic: stock(기존 전부) · real_estate · rates · deposit · loan.
--   news_articles.asset_tags_json: 기사 제목에서 수집 때 한 번 뽑은 자산 칩 [{kind,key,name}] —
--   조회 때 이름 색인을 훑지 않는다(PRINCIPLES 응답 속도 3).
--
-- 모든 ALTER는 중복 컬럼·키 오류를 무시하는 경로(applyMigrationFile)로 돈다. UPDATE는 NULL만 채워 몇 번 돌아도 같다.

USE quantport;

ALTER TABLE stock_change_events ADD COLUMN market_cap DOUBLE NULL AFTER importance;

UPDATE stock_change_events e
  JOIN stock_scores s ON s.symbol = e.symbol AND s.trade_date = e.detected_for
   SET e.market_cap = CAST(s.inputs->>'$.marketCap' AS DOUBLE)
 WHERE e.market_cap IS NULL;

ALTER TABLE news_articles
  ADD COLUMN topic VARCHAR(12) NOT NULL DEFAULT 'stock',
  ADD COLUMN asset_tags_json JSON NULL;

ALTER TABLE news_articles ADD KEY idx_news_market_topic_published (market, topic, published_at DESC);

-- ① 실적 AI 요약 — 시장별 최신 1행(이력 없음). 배치(07:10 · 국내 18:50 · 해외 21:30)가 쓰고 화면은 이 행만 읽는다.
--   input_hash가 같고 이미 AI 글이면 다시 부르지 않는다.
CREATE TABLE IF NOT EXISTS earnings_digests (
  market        VARCHAR(10)   NOT NULL PRIMARY KEY,
  body          JSON          NOT NULL,
  facts         JSON          NOT NULL,
  source        VARCHAR(8)    NOT NULL DEFAULT 'rule',
  model         VARCHAR(64)   NOT NULL DEFAULT '',
  input_hash    CHAR(64)      NOT NULL DEFAULT '',
  tokens        INT           NOT NULL DEFAULT 0,
  generated_at  TIMESTAMP(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
