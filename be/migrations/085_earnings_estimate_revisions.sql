-- 미국 EPS 컨센서스의 **변화 이력** — 추정치 리비전 지표의 재료 (2026-09-24 사용자 결정)
--
-- earnings_events의 eps_est는 매일 06:00 Finnhub 캘린더 적재가 **덮어쓴다** — 그래서 「한 달
-- 전 추정치」가 어디에도 남지 않았다. 이 표가 예정 행의 추정치를 날짜와 함께 남긴다.
--
-- **값이 바뀐 날만** 넣는다(첫 관측 포함). 매일 전부 넣으면 예정 약 3,600행 × 365일이 되는데,
-- 추정치는 대부분의 날 그대로다. 「그날의 값」은 그날 이하의 가장 최근 행이다.
--
-- 국내는 없다 — 컨센서스 소스(FnGuide)가 라이선스 대상이라 보류했다.
-- 축 편입은 표본이 찬 뒤 따로 정한다. 지금은 적재만 한다.

USE quantport;

CREATE TABLE IF NOT EXISTS earnings_estimate_revisions (
  symbol         VARCHAR(20)    NOT NULL,
  fiscal_year    SMALLINT       NOT NULL,
  fiscal_period  VARCHAR(8)     NOT NULL,
  snap_date      DATE           NOT NULL,
  eps_est        DECIMAL(18,4)  NOT NULL,
  created_at     TIMESTAMP(3)   NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (symbol, fiscal_year, fiscal_period, snap_date)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
