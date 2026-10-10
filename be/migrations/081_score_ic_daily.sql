-- AI 점수 **성적표** — 축별 예측력(IC)의 날짜별 기록
--
-- # 무엇을 재나
--
-- 점수일 D마다, 시장마다, 축마다(종합 + 다섯 축):
--
--   ic         그날 축 점수와 **그 뒤 h거래일 수익률**의 스피어만 순위상관(−1..1).
--              0이면 점수가 수익을 가르지 못했다는 뜻이다.
--   top_excess 그 축 상위 20종목의 평균 수익률 − 전체 평균 수익률(%p)
--   top_hit    상위 20종목 중 전체 중앙값보다 오른 비율
--   n          계산에 들어간 종목 수(랭킹 자격 — 축 3개 이상 · 지수 추종 상품 제외)
--
-- **미래를 보지 않는다**: 수익률은 D **다음** 세션부터 센다. 미국 종목은 D일 KST 배치 때
-- 아직 끝나지 않은 세션을 건너뛴다(보수적).
--
-- # 왜 표로 두나
--
-- 한 날짜를 재려면 종목 수천 개의 종가를 두 번씩 찾는다. 조회마다 30일치를 다시 재면
-- 1 ECPU DB에 부담이다 — 점수 배치 꼬리에서 새로 잴 수 있게 된 날짜만 재서 쌓는다.
-- 하루 2시장 × 6축 = 12행.

USE quantport;

CREATE TABLE IF NOT EXISTS score_ic_daily (
  trade_date  DATE          NOT NULL,
  market      VARCHAR(16)   NOT NULL,   -- domestic | us
  axis        VARCHAR(16)   NOT NULL,   -- composite | technical | quality | growth | valuation | sentiment
  horizon     SMALLINT      NOT NULL,   -- 거래일 수
  ic          DECIMAL(6,4)  NULL,
  top_excess  DECIMAL(8,4)  NULL,
  top_hit     DECIMAL(5,4)  NULL,
  n           INT           NOT NULL DEFAULT 0,
  computed_at TIMESTAMP(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (trade_date, market, axis, horizon)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
