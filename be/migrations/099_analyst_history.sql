-- 애널리스트 의견 이력 · 증권사 리포트 목록 (2026-10-07 사용자 요청 — 「저장 기간을 늘리고 추이를 보여 주는 표」)
--
--   stock_analyst_history  093의 stock_analyst_consensus는 종목당 **최신 한 줄**만 덮어썼다.
--                          해외: Finnhub /stock/recommendation은 한 번 부를 때 최근 몇 달 치를 함께 준다 — 전부 남긴다(월 1줄).
--                          국내: 네이버 컨센서스는 **의견·목표가가 바뀐 날만** 한 줄 남긴다(매일 같은 값을 쌓지 않는다).
--                          삭제 잡은 두지 않는다 — 해외 월 1줄 · 국내 변경 시 1줄이라 연 수만 행 수준.
--   stock_analyst_reports  네이버 integration 응답의 최근 증권사 리포트(증권사 · 제목 · 날짜). 본문은 싣지 않고 링크만.
-- (마이그레이션 분리기가 세미콜론으로 자르므로 주석에 세미콜론을 쓰지 않는다)

USE quantport;

CREATE TABLE IF NOT EXISTS stock_analyst_history (
  symbol          VARCHAR(32)  NOT NULL,
  source          VARCHAR(16)  NOT NULL,
  as_of           DATE         NOT NULL,
  strong_buy      INT          NULL,
  buy             INT          NULL,
  hold            INT          NULL,
  sell            INT          NULL,
  strong_sell     INT          NULL,
  rating          DOUBLE       NULL,
  target_mean     DOUBLE       NULL,
  target_currency CHAR(3)      NOT NULL DEFAULT '',
  fetched_at      TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (symbol, source, as_of)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS stock_analyst_reports (
  source      VARCHAR(16)  NOT NULL,
  report_id   BIGINT       NOT NULL,
  symbol      VARCHAR(32)  NOT NULL,
  broker      VARCHAR(64)  NOT NULL DEFAULT '',
  title       VARCHAR(300) NOT NULL DEFAULT '',
  published   DATE         NULL,
  fetched_at  TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (source, report_id),
  KEY idx_sar_symbol (symbol, published)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- 지금까지 덮어써 온 최신 한 줄을 이력의 첫 점으로 옮긴다(이력 표가 첫날부터 비지 않게).
INSERT IGNORE INTO stock_analyst_history
  (symbol, source, as_of, strong_buy, buy, hold, sell, strong_sell, rating, target_mean, target_currency, fetched_at)
SELECT symbol, source, as_of, strong_buy, buy, hold, sell, strong_sell, rating, target_mean, target_currency, fetched_at
  FROM stock_analyst_consensus WHERE as_of IS NOT NULL;
