-- 국내 ETF 점검 — 일별 지표와 그 점수 (2026-09-24 사용자 결정: 국내 ETF, 종목 화면 + 랭킹 ETF 탭)
--
-- 출처는 KRX Open API `etf_bydd_trd`(하루 1호출로 전 ETF). **다음 영업일에 공개된다** —
-- 실측 09-24 03시에 09-23은 빈 배열이었다. 그래서 적재는 오전에 「어제까지」를 받는다.
--
-- 점수는 주식 점수와 **다른 잣대**다. ETF는 회사가 아니라 상품이라 수익성·성장은 뜻이 없고,
-- 같은 지수를 따르는 상품끼리 갈리는 것은 비용·괴리·추적·유동성이다. 총보수는 소스가
-- 없어 빠졌다(화면이 그렇게 적는다).

USE quantport;

CREATE TABLE IF NOT EXISTS etf_daily_metrics (
  symbol             VARCHAR(32)    NOT NULL,
  trade_date         DATE           NOT NULL,
  close_price        DOUBLE         NOT NULL,
  nav                DOUBLE         NULL,
  net_assets         BIGINT         NULL,
  trade_value        BIGINT         NULL,
  index_name         VARCHAR(120)   NOT NULL DEFAULT '',
  index_value        DOUBLE         NULL,
  -- 기초지수 전일 대비 등락률(%). KRX가 준 값 그대로.
  index_change_rate  DOUBLE         NULL,
  PRIMARY KEY (symbol, trade_date),
  KEY idx_etf_metrics_date (trade_date)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- 한 날짜의 ETF 점검 결과. 비교 집단(peer_group)은 일반 / 채권·금리(lowvol) / 레버리지·인버스.
CREATE TABLE IF NOT EXISTS etf_scores (
  symbol              VARCHAR(32)    NOT NULL,
  trade_date          DATE           NOT NULL,
  score               DECIMAL(5,2)   NOT NULL,
  peer_group          VARCHAR(16)    NOT NULL,
  peer_count          INT            NOT NULL,
  premium_pct         DOUBLE         NULL,
  tracking_error_pct  DOUBLE         NULL,
  beta                DOUBLE         NULL,
  trade_value_median  BIGINT         NULL,
  net_assets          BIGINT         NULL,
  parts               JSON           NOT NULL,
  PRIMARY KEY (symbol, trade_date),
  KEY idx_etf_scores_date_score (trade_date, score)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
