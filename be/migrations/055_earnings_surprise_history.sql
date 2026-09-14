-- 과거 컨센서스 적중 이력 — Finnhub `/stock/earnings` (Earnings Surprises).
--
-- # 왜 earnings_events와 따로 두는가
--
-- 이 소스는 **발표일을 주지 않는다.** 주는 날짜는 `period`, 즉 회계기간 종료일이다
-- (AAPL 3Q FY2026 = 2026-06-30, 실제 발표는 2026-07-30). earnings_events에 그대로
-- 넣으면 두 가지가 동시에 깨진다:
--
--   1. 그 표의 upsert는 `event_date = LEAST(event_date, VALUES(event_date))`라,
--      이미 맞게 들어 있는 발표일이 **한 달 이른 기간종료일로 덮어써진다**.
--   2. 캘린더·히트맵이 분기말(3/31·6/30·9/30·12/31)에 수백 건씩 뭉친다 —
--      아무도 그날 발표하지 않았는데.
--
-- 051의 헤더가 「추정 날짜는 저장하지 않고 읽을 때 계산한다」고 못박은 것과 같은
-- 이유다. 발표일을 모르는 데이터는 발표 캘린더에 들어가면 안 된다.
--
-- # 무엇에 쓰는가
--
-- 「꾸준히 기대를 넘는 기업」 카드 하나다. 그 카드는 (종목·분기·기대·실제)만 있으면
-- 되고 발표일은 필요 없다 — 필요한 것은 **순서**뿐이라 period_end로 충분하다.
--
-- # 왜 이 표가 필요했나 (2026-08-21)
--
-- Finnhub 무료 티어의 `/calendar/earnings`는 **과거 30일까지만** 준다(실측:
-- 2026-07-22 이전은 HTTP 200에 빈 배열). 그래서 해외 실적은 일일 배치가 도는
-- 동안 쌓인 한 시즌치(3,043종목 × 1분기)밖에 없었고, 적중 이력 카드는 최소
-- 표본 4분기를 채우는 종목이 **0개**라 늘 비어 있었다.
--
-- `/stock/earnings`는 종목당 **정확히 4분기**를 준다(limit 파라미터는 무시된다).
-- 8분기에는 못 미치지만, 지금 받지 않으면 그 4분기는 캘린더 API로 영원히 되찾을
-- 수 없다 — 일회성 백필(cmd/backfill-earnings-surprises)로 한 번 확보해 둔다.
--
-- EPS만 온다. 매출 컨센서스(`/stock/revenue-estimate`)는 무료 티어에서 403이다.
CREATE TABLE IF NOT EXISTS earnings_surprise_history (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  symbol VARCHAR(20) NOT NULL,
  market VARCHAR(10) NOT NULL,                     -- 'us' (국내는 FnGuide가 2년치를 준다)

  fiscal_year SMALLINT NOT NULL,
  -- '1Q'..'4Q' | 'FY'. earnings_events.fiscal_period와 **같은 어휘**를 쓴다 —
  -- 두 표를 (symbol, fiscal_year, fiscal_period)로 합치므로 한쪽이 'Q3',
  -- 다른 쪽이 '3Q'면 같은 분기가 두 번 세어진다.
  fiscal_period VARCHAR(8) NOT NULL,
  -- 회계기간 종료일. **발표일이 아니다.** 최근 N분기를 고르는 정렬 축으로만 쓴다.
  period_end DATE NOT NULL,

  eps_est DECIMAL(18,4) NULL DEFAULT NULL,
  eps_act DECIMAL(18,4) NULL DEFAULT NULL,

  currency VARCHAR(3) NOT NULL DEFAULT 'USD',
  source VARCHAR(16) NOT NULL,                     -- 'finnhub'
  fetched_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,

  PRIMARY KEY (id),
  -- basis가 키에 없는 이유: 미국 소스에는 연결/개별 구분이 없다(earnings_events는
  -- 국내와 표를 공유하느라 빈 문자열을 넣는다).
  UNIQUE KEY uk_surprise_history_period (symbol, fiscal_year, fiscal_period),
  -- 카드의 주 질의: 시장 + 최근 N분기.
  KEY idx_surprise_history_market_period (market, period_end)
);
