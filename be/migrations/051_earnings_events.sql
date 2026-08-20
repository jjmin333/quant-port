-- 실적 발표 이벤트 — 국내(FnGuide 잠정실적발표) · 해외(Finnhub earnings calendar).
--
-- 한 행 = 한 종목의 한 회계기간 발표. "언제 발표하며 / 시장 기대가 얼마였고 /
-- 실제가 얼마였나"를 한 행에 담아 서프라이즈를 계산할 수 있게 한다.
--
-- 이 표에는 **확정(발표됨)과 예정(소스가 준 일정)만** 들어간다. 국내 「추정」
-- 발표일(작년 동기로 역산)은 저장하지 않고 읽을 때 계산한다 — 저장하면 실제
-- 발표가 들어온 순간 추정 행을 지우러 다녀야 하고, 놓치면 같은 분기가 두 줄로
-- 뜬다(목표가 도달일을 저장하지 않는 것과 같은 이유).
CREATE TABLE IF NOT EXISTS earnings_events (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  symbol VARCHAR(20) NOT NULL,
  market VARCHAR(10) NOT NULL,                     -- 'domestic' | 'us'

  -- 원 발표일. 정정 공시가 와도 이 값은 덮지 않는다 — "삼성전자는 7/7에
  -- 발표했다"가 정정일 7/30으로 바뀌면 캘린더가 거짓말을 한다.
  event_date DATE NOT NULL,
  date_status VARCHAR(12) NOT NULL,                -- 'confirmed' | 'scheduled'
  -- 미국만: 'bmo'(장전) | 'amc'(장후) | 'dmh'(장중). 국내 소스는 시각을 안 준다.
  session VARCHAR(8) NULL DEFAULT NULL,

  fiscal_year SMALLINT NOT NULL,
  -- '1Q','2Q','3Q','4Q','FY'. FnGuide의 FREQ='D'(연간 결산)를 'FY'로 정규화한다 —
  -- 2월 발표분 480건이 'D'라서 분기 코드만 받으면 결산 시즌을 통째로 놓친다.
  fiscal_period VARCHAR(8) NOT NULL,
  period_end DATE NULL DEFAULT NULL,
  -- 국내 '연결'|'개별'. 미국은 ''. NULL로 두면 MySQL UNIQUE가 NULL 중복을
  -- 허용해 같은 미국 종목·기간이 무한히 쌓인다.
  basis VARCHAR(10) NOT NULL DEFAULT '',

  -- 금액은 **기준통화 원본 단위**(원·달러)로 저장한다. FnGuide는 억원이라
  -- 적재 시 ×1e8. 화면이 formatFinancialAmount(value, currency) 하나로 끝나야
  -- 목록·상세·홈이 같은 숫자를 말한다.
  revenue_est DECIMAL(24,2) NULL DEFAULT NULL,
  revenue_act DECIMAL(24,2) NULL DEFAULT NULL,
  op_income_est DECIMAL(24,2) NULL DEFAULT NULL,
  op_income_act DECIMAL(24,2) NULL DEFAULT NULL,
  net_income_est DECIMAL(24,2) NULL DEFAULT NULL,
  net_income_act DECIMAL(24,2) NULL DEFAULT NULL,
  eps_est DECIMAL(18,4) NULL DEFAULT NULL,
  eps_act DECIMAL(18,4) NULL DEFAULT NULL,

  -- FnGuide가 주는 전분기 대비 증감률(%). YoY가 아니라 QoQ임을 실측으로 확인했다
  -- (삼성 3Q25 매출 86.1조 · G=15.4 → 역산 Q2 74.6조 = 실제와 일치).
  revenue_qoq DECIMAL(12,2) NULL DEFAULT NULL,
  op_income_qoq DECIMAL(12,2) NULL DEFAULT NULL,
  net_income_qoq DECIMAL(12,2) NULL DEFAULT NULL,

  currency VARCHAR(3) NOT NULL,                    -- 'KRW' | 'USD'
  source VARCHAR(16) NOT NULL,                     -- 'fnguide' | 'finnhub'
  -- 이 수치를 실어 온 회차의 날짜. 정정 판정에 쓴다 — source_date가 더 큰
  -- 행만 수치를 덮어써야 오래된 회차가 최신 정정을 되돌리지 않는다.
  source_date DATE NOT NULL,
  revised_at DATE NULL DEFAULT NULL,

  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,

  PRIMARY KEY (id),
  UNIQUE KEY uk_earnings_period (symbol, fiscal_year, fiscal_period, basis),
  -- 캘린더 화면의 주 질의: 기간 + 시장.
  KEY idx_earnings_date_market (event_date, market),
  KEY idx_earnings_symbol_date (symbol, event_date)
);
