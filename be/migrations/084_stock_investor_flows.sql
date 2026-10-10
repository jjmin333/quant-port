-- 종목별 투자자 매매동향(국내) — 심리·수급 축의 수급 재료 (2026-09-24 사용자 결정)
--
-- 출처는 Toss `/stocks/{symbol}/investor-trading`(STOCK_TRADING_TREND 버킷, 사용자 시세가 쓰는
-- MARKET_DATA와 다른 버킷). 수량은 전부 **주식 수**다 — 거래대금 축은 소스에 없다.
--
-- **확정치만 넣는다.** 장중 잠정 기록은 개인(individual)이 비어 있고 저녁에 값이 바뀐다.
-- 잠정치를 넣으면 같은 날이 두 값으로 두 번 읽힌다 — 적재가 개인 값이 있는 기록만 받는다.
--
-- foreign_net은 등록외국인 기준(시장 지표의 외국인 합계와 기준이 다르다).
-- 크기: 국내 보통주 약 2,760 × 연 250일 ≈ 69만 행/년. 적재가 400일 넘은 행을 지운다.

USE quantport;

CREATE TABLE IF NOT EXISTS stock_investor_flows (
  symbol             VARCHAR(32)   NOT NULL,
  trade_date         DATE          NOT NULL,
  foreign_net        BIGINT        NOT NULL,
  institution_net    BIGINT        NOT NULL,
  individual_net     BIGINT        NULL,
  foreign_hold_rate  DECIMAL(7,4)  NULL,
  updated_at         TIMESTAMP(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (symbol, trade_date),
  KEY idx_investor_flows_date (trade_date)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
