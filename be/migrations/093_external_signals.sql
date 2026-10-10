-- 외부 신호 세 가지 (2026-09-28 사용자 결정 — 요청 6·7)
--
--   stock_ext_metrics        해외 지표 보조 — Finnhub /stock/metric. **EDGAR 값이 없을 때만** 쓴다
--                            (TSM처럼 20-F가 현지 통화라 비워 둔 종목 등). 비율은 본국 상장 기준이라
--                            ADR에도 그대로 맞다(통화가 약분된다).
--   stock_analyst_consensus  애널리스트 — 해외: Finnhub 추천 추이(매수·보유·매도 명수, 무료),
--                            국내: 네이버 증권 평균 목표가·투자의견(1~5).
--   social_mentions          Reddit 종목 언급 — ApeWisdom(24시간 언급·전일·추천 수·순위). 하루 한 줄.
--
-- 전부 배치가 채우고 조회는 표만 읽는다(외부 호출은 worker-filings에서만).

USE quantport;

CREATE TABLE IF NOT EXISTS stock_ext_metrics (
  symbol           VARCHAR(32)  NOT NULL PRIMARY KEY,
  source           VARCHAR(16)  NOT NULL DEFAULT 'finnhub',
  per              DOUBLE       NULL,
  pbr              DOUBLE       NULL,
  psr              DOUBLE       NULL,
  roe              DOUBLE       NULL,
  operating_margin DOUBLE       NULL,
  net_margin       DOUBLE       NULL,
  revenue_growth   DOUBLE       NULL,
  eps_growth       DOUBLE       NULL,
  current_ratio    DOUBLE       NULL,
  dividend_yield   DOUBLE       NULL,
  beta             DOUBLE       NULL,
  fetched_at       TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS stock_analyst_consensus (
  symbol          VARCHAR(32)  NOT NULL PRIMARY KEY,
  source          VARCHAR(16)  NOT NULL,
  -- 그 컨센서스의 기준일(Finnhub period · 네이버 createDate).
  as_of           DATE         NULL,
  strong_buy      INT          NULL,
  buy             INT          NULL,
  hold            INT          NULL,
  sell            INT          NULL,
  strong_sell     INT          NULL,
  -- 의견 평균 1(강력매도)~5(강력매수). 해외는 명수로 계산, 국내는 네이버 값.
  rating          DOUBLE       NULL,
  target_mean     DOUBLE       NULL,
  target_currency CHAR(3)      NOT NULL DEFAULT '',
  fetched_at      TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS social_mentions (
  symbol        VARCHAR(32)  NOT NULL,
  snap_date     DATE         NOT NULL,
  source        VARCHAR(16)  NOT NULL DEFAULT 'apewisdom',
  mentions      INT          NOT NULL,
  mentions_prev INT          NOT NULL,
  upvotes       INT          NOT NULL DEFAULT 0,
  rank_now      INT          NOT NULL,
  rank_prev     INT          NULL,
  fetched_at    TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (symbol, snap_date, source),
  KEY idx_sm_date (snap_date)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
