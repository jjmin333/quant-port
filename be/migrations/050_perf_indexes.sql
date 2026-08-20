USE quantport;

-- 출시 전 응답 속도 정리 — 인덱스 두 개.
--
-- 1) stock_scores 랭킹 (`/stocks/scores/top`)
--
-- 기존 계획은 `stocks` 17,070행을 풀스캔해 구동 테이블로 삼고, 매 행마다
-- stock_scores·stock_daily_closes를 eq_ref로 찔러본 뒤 전부 모아 filesort 했다.
-- 해외 탭(us+etf = 13,506종)에서 실측 2.10초. 스무 줄을 보여주려고 17,070행을
-- 읽는다.
--
-- 이 인덱스는 (trade_date, score DESC, peer_pct DESC)라 ORDER BY가 인덱스 순서와
-- 정확히 같다. 그래서 옵티마이저가 stock_scores를 구동 테이블로 잡으면 점수 높은
-- 순으로 읽다가 LIMIT을 채우는 순간 멈춘다 — filesort도, temporary table도 없다.
-- 실측 2.10초 -> 0.036초.
--
-- DESC 인덱스는 MySQL 8.0+ 기능이다. 5.7에서는 문법은 통과하지만 무시되고
-- (오름차순으로 만들어짐) 역방향 스캔이 되어 성능 이득만 줄어든다 — 결과는 같다.
--
-- 2) news_articles 최신 갱신 시각 (`/news/{market}`)
--
-- 목록 응답의 meta.updatedAt 한 칸을 채우려고 `MAX(updated_at) WHERE market=?`을
-- 부른다. updated_at이 어느 인덱스에도 없어 해당 market의 3만여 행을 전부 읽었다 --
-- 실측 domestic 350ms, us 727ms. 정작 목록 본 쿼리는 0.65ms다.
--
-- (market, updated_at)이면 MAX는 인덱스 끝에서 한 행만 읽는다. 실측 727ms -> 0.45ms.

CREATE INDEX idx_stock_scores_rank
  ON stock_scores (trade_date, score DESC, peer_pct DESC);

CREATE INDEX idx_news_market_updated
  ON news_articles (market, updated_at);
