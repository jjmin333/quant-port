-- 종목별 **축 점수 이력** — 30거래일 뒤에도 「그날 몇 점이었나」에 답하는 표
--
-- # 왜 표를 하나 더 만드나
--
-- `stock_scores`는 최신 30거래일만 남는다(행당 약 1.5KB, 대부분이 `inputs` JSON —
-- 그대로 두면 연 7.5GB). 그 결과 두 곳이 답을 잃는다:
--
--   · **보유 이유 점검** — 매수 시점 점수가 거의 언제나 없다. 대부분의 매수는 30거래일
--     보다 오래됐고, 화면은 「비교 불가」라고만 말한다.
--   · **점수 성적표(IC)** — 상위 200만 남는 `score_ranking_history`로는 「점수가 높을수록
--     더 올랐나」를 전 종목에서 잴 수 없다.
--
-- 둘 다 필요한 것은 지표 원값이 아니라 **다섯 축 값 + 종합 + 커버리지**뿐이다.
--
-- # 왜 얇은가
--
-- 실측(2026-09-23, 30거래일 백필): 537,387행 · 데이터 38.6MB + 인덱스 9.5MB ≈ 행당 90B.
-- 하루 17,906행이면 **하루 약 1.6MB · 연 약 400MB**. `stock_scores`를 1년 보관하는
-- 것(7.5GB)의 5% 수준이다. 운영 DB는 1 ECPU · 29MiB/s라
-- 이 차이가 곧 이 기능의 가능 여부다.
--
-- # 왜 DECIMAL(5,2)인가
--
-- 설명 엔진의 귀인은 저장값으로 종합 점수를 **잔차 0**으로 재현한다. 정수로 자르면
-- 축마다 ±0.5가 생겨 과거 날짜의 귀인이 맞지 않는다. 소수 둘째 자리면 오차가 가중
-- 평균 뒤 0.005 이하라 정수 점수 재현에 영향이 없다.
--
-- # 키
--
-- PK (symbol, trade_date) — 사용자 경로의 읽기(한 종목의 매수일 · 한 종목의 1년)가
-- 전부 종목 단위다. idx_date는 배치 경로(날짜별 전 종목 — IC 계산 · 채워진 날짜 확인)용.
--
-- # 보관
--
-- 지우지 않는다. 이 표의 존재 이유가 「오래 쌓이는 것」이다.

USE quantport;

CREATE TABLE IF NOT EXISTS stock_score_history (
  symbol      VARCHAR(32)   NOT NULL,
  trade_date  DATE          NOT NULL,
  -- 종합 0~100 (커버리지 수축 후, `stock_scores.score` 그대로). 보류면 NULL.
  score       TINYINT       NULL,
  -- 축 값. 없는 축은 NULL — 0이 아니다(0은 「나쁘다」로 읽힌다).
  technical   DECIMAL(5,2)  NULL,
  quality     DECIMAL(5,2)  NULL,
  growth      DECIMAL(5,2)  NULL,
  valuation   DECIMAL(5,2)  NULL,
  sentiment   DECIMAL(5,2)  NULL,
  coverage    DECIMAL(4,3)  NOT NULL DEFAULT 0,
  PRIMARY KEY (symbol, trade_date),
  KEY idx_stock_score_history_date (trade_date)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
