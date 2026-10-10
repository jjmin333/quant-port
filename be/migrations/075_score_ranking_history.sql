-- AI 랭킹의 **날짜별 구성** — 백테스트가 읽는 유일한 과거 기록
--
-- # 왜 표를 하나 더 만드나
--
-- `stock_scores`는 **최신 30거래일만** 보관한다. 행당 약 1.5KB(`inputs` JSON이 평균
-- 825B)이고 하루 17,906행이라 그대로 두면 연 7.5GB다 — 정리는 선택이 아니었다
-- (`repository.ScoreRetentionDates` 머리글).
--
-- 그런데 백테스트가 묻는 것은 그 표의 대부분이 아니다. 「2026-03-04의 국내 성장
-- 상위 100은 누구였나」 한 줄이면 되고, 지표 원값도 백분위도 필요 없다.
--
-- # 왜 얇은가
--
-- 시장 2 × 축 6 × 상위 200 = **하루 2,400행**, 행당 약 40B.
--
--     하루     96KB
--     1년      876,000행 · 약 35MB (+ 인덱스)
--
-- `stock_scores`를 1년 보관하는 것(7.5GB)의 **0.5%**다. 운영 DB는 1 ECPU · 29MiB/s라
-- 이 차이가 곧 이 기능의 가능 여부다.
--
-- # 왜 상위 200인가
--
-- 백테스트가 고를 수 있는 가장 큰 바구니다. 그보다 깊은 구간은 「AI가 추천한 종목」이
-- 아니라 그냥 시장이고, 200을 넘기면 행 수가 곧바로 비례해 늘어난다.
--
-- # `rank`가 아니라 `rank_no`인 이유
--
-- `RANK`는 MySQL 8의 예약어다(윈도 함수). 백틱으로 감싸면 되지만, 그러면 이 컬럼을
-- 쓰는 모든 문장이 백틱을 기억해야 한다 — 한 번만 이름을 바꾸는 쪽이 싸다.
--
-- # 보관
--
-- 지우지 않는다. 이 표의 존재 이유가 「오래 쌓이는 것」이고, 35MB/년이면 그럴 수 있다.

USE quantport;

CREATE TABLE IF NOT EXISTS score_ranking_history (
  trade_date DATE            NOT NULL,
  market     VARCHAR(16)     NOT NULL,  -- domestic | overseas
  axis       VARCHAR(16)     NOT NULL,  -- composite | technical | quality | growth | valuation | sentiment
  rank_no    SMALLINT UNSIGNED NOT NULL, -- 1..200
  symbol     VARCHAR(20)     NOT NULL,
  -- 그 축의 점수. 종합이면 종합 점수, 축이면 그 축의 0~100 값을 반올림한 정수.
  -- 백테스트는 순위만 쓰지만, 이 값이 없으면 나중에 「그때 몇 점이었나」에 답할 수 없다.
  score      SMALLINT        NOT NULL,
  PRIMARY KEY (trade_date, market, axis, rank_no),
  -- 한 종목이 언제 랭킹에 있었는지 (종목 타임라인 · 리포트 검증이 쓴다).
  KEY idx_symbol_date (symbol, trade_date)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
