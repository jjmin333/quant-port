USE quantport;

-- 인기순 뉴스 인덱스 + 점수 테이블의 중복 인덱스 제거.
--
-- 둘을 한 파일에 둔 이유는 성격이 같아서다 — **읽기 계획을 실측으로 확인하고
-- 넣거나 뺀 인덱스**다. 각각의 근거는 docs/PERFORMANCE_AUDIT.md P1-6·P2-11.
--
-- ============================================================
-- 1) news_articles (market, view_count DESC, published_at DESC)
-- ============================================================
--
-- 뉴스 목록은 `sort=popular`를 지원하고, 그 쿼리는 이렇게 나간다.
--
--   SELECT ... FROM news_articles
--    WHERE market = ?
--    ORDER BY view_count DESC, published_at DESC
--    LIMIT ?
--
-- 기존 인덱스에는 `view_count`가 없다. 그래서 실행계획이
-- `idx_news_market_published`로 **그 시장의 행을 전부 읽어 filesort**했다.
--
--   EXPLAIN ANALYZE (2026-08-24, domestic 67,957행)
--     -> Sort: view_count DESC, published_at DESC  (actual time=238..238)
--       -> Index lookup using idx_news_market_published  (rows=67957)
--
-- 이 인덱스를 넣은 뒤 같은 쿼리(실제 22컬럼 형태):
--
--     -> Index lookup using idx_news_market_popular  (actual time=0.0173..0.289 rows=100)
--
-- **238ms → 0.289ms, 읽는 행 67,957 → 100.** us 시장도 같은 인덱스를 탄다.
--
-- 이 쿼리는 사용자가 「인기」 칩을 눌러야만 도는 것이 아니다 —
-- `news-side-rail.tsx`가 뉴스 페이지를 열 때마다 `sort=popular`를 부른다
-- (브라우저 네트워크 로그로 확인. API 액세스 로그에는 안 보이는데,
--  로거가 `r.URL.Path`만 찍고 쿼리스트링을 버리기 때문이다).
--
-- 쓰기 대가: `view_count`를 올리는 경로는 `IncrementViewCount` 하나뿐이고
-- 호출처도 하나다(news_store_backfill.go). 관측 구간 전체에서 호출 0건이라
-- 인덱스 유지 비용은 사실상 없다.
--
-- DESC를 명시한 것은 MySQL 8의 내림차순 인덱스를 그대로 쓰기 위해서다.
-- 8.0 미만이라면 이 구문이 무시되어 filesort가 남는다.

ALTER TABLE news_articles
  ADD INDEX idx_news_market_popular (market, view_count DESC, published_at DESC),
  ALGORITHM=INPLACE, LOCK=NONE;

-- ============================================================
-- 2) stock_scores 중복 인덱스 제거
-- ============================================================
--
-- 세 인덱스가 겹쳐 있었다.
--
--   idx_stock_scores_date   (trade_date)
--   idx_stock_scores_score  (trade_date, score)
--   idx_stock_scores_rank   (trade_date, score, peer_pct)
--
-- 앞의 둘은 셋째의 **완전한 접두사**라 읽기에 아무것도 더해주지 않는다. 반면
-- 점수 배치는 500행씩 다중행 upsert를 돌리며 세 인덱스를 모두 갱신했다.
--
-- 제거 전후로 이 표를 읽는 쿼리 네 종류의 실행계획을 전부 확인했다
-- (2026-08-24, trade_date=2026-08-21):
--
--   랭킹 상위 (ORDER BY score DESC, peer_pct DESC)
--     before 0.070ms  idx_stock_scores_rank covering
--     after  0.072ms  idx_stock_scores_rank covering      ← 동일
--   커버리지 COUNT (WHERE trade_date = ?)
--     before 3.78ms   idx_stock_scores_score covering
--     after  3.4~4.3ms idx_stock_scores_rank covering     ← 노이즈 범위
--   보관기간 컷오프 (DISTINCT trade_date ORDER BY DESC)
--     before/after 모두 materialize + sort, 0.1~0.5ms     ← 동일
--   심볼 히스토리 (WHERE symbol = ?)
--     before/after 모두 PRIMARY (reverse)                 ← 동일
--
-- 인덱스 크기는 **10.5MB → 4.5MB (57% 감소)**.
--
-- 커버리지 COUNT만 약간 넓은 인덱스를 타게 되는데(1.1ms 안팎), 이 쿼리는
-- 배치·관리자 커버리지 조회 경로(`StockScoreRepo.Stats`)라 요청 경로가 아니다.
-- 쓰기 쪽에서 인덱스 두 개를 덜 만지는 이득이 더 크다고 판단했다.

-- **두 DROP을 한 문장에 묶지 않는다.**
--
-- 묶으면 둘 중 하나가 이미 없을 때 문장 전체가 1091(can't DROP)로 실패하고,
-- 마이그레이터가 그 오류를 재실행 안전성 때문에 넘기므로 **남은 하나도 영영
-- 지워지지 않는다.** 실제로 그렇게 만들었다가 검증에서 잡았다 — 인덱스를
-- 되돌리고 재기동했더니 `idx_news_market_popular`는 생겼는데
-- `idx_stock_scores_date`는 그대로 남았다.
--
-- 문장을 갈라 두면 각각이 독립적으로 멱등이라, 어느 쪽이 이미 적용된 상태에서
-- 시작해도 나머지가 반드시 수렴한다.

ALTER TABLE stock_scores
  DROP INDEX idx_stock_scores_date,
  ALGORITHM=INPLACE, LOCK=NONE;

ALTER TABLE stock_scores
  DROP INDEX idx_stock_scores_score,
  ALGORITHM=INPLACE, LOCK=NONE;
