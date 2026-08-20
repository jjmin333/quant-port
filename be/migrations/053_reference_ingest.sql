-- 조회 시점 외부 호출을 없애기 위한 적재 표 3종.
--
-- 셋 다 같은 문제를 풀고 있다: 화면이 열릴 때 DART/EDGAR/Toss를 부르던 데이터를
-- 배치가 미리 담아 두고, 조회는 표만 읽는다. 사용자 수가 늘어도 외부 호출은
-- 늘지 않는다.

-- ── 1. 분기 재무 ─────────────────────────────────────────────────────────
-- 재무제표 상세 모달(`/stocks/{symbol}/financials/history`)과 종목 실적 탭의
-- 「분기 실적」이 **같은 데이터를 서로 다른 경로로** 받아오고 있었다. 국내는 둘 다
-- DART인데 한쪽은 FetchQuarterlyHistory(연도×보고서코드 = 종목당 16호출),
-- 다른 쪽은 FetchQuarterlySnapshots였다. 한 표로 모아 한 번만 받는다.
--
-- 한 행 = 한 종목의 한 회계분기. 성장률(QoQ)은 저장하지 않고 읽을 때 계산한다 —
-- 직전 분기가 나중에 채워지면 저장된 값이 틀린 채로 남는다.
CREATE TABLE IF NOT EXISTS financial_periods (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  symbol VARCHAR(20) NOT NULL,
  source VARCHAR(8) NOT NULL,                      -- 'dart' | 'edgar'

  fiscal_year SMALLINT NOT NULL,
  -- 1..4. 국내 보고서(1분기·반기·3분기·사업)와 미국 Q1..Q4/FY를 같은 축으로 맞춘다.
  period_index TINYINT NOT NULL,

  -- 'quarter'    — 그 분기만의 값 (재무제표 상세 모달)
  -- 'cumulative' — 기초부터의 누적 (국내 실적 탭의 「반기」·「연간」)
  --
  -- 두 화면이 **다른 것을 보고 있다**. 국내 DART는 누적으로 보고하고 실적 탭은
  -- 그대로 보여주는데, 상세 모달은 4분기에서 3분기 누적을 빼 분기값을 만든다.
  -- 한 축으로 합치면 어느 한쪽 화면의 숫자가 조용히 바뀐다 — 그래서 유니크 키에
  -- basis를 넣어 둘 다 담는다. 미국(EDGAR)은 원래 두 화면이 같은 값을 쓰므로
  -- 'quarter' 한 벌만 저장한다.
  basis VARCHAR(12) NOT NULL DEFAULT 'quarter',
  -- 화면에 그대로 쓰는 표기: '1Q','반기','3Q','연간' (KR) / 'Q1'..'FY' (US).
  -- period_index에서 만들 수도 있지만, 국내 「반기」를 「2Q」로 바꿔 부르면
  -- 누적치를 분기치로 오해하게 된다 — 소스가 말한 이름을 그대로 보관한다.
  period_label VARCHAR(12) NOT NULL DEFAULT '',
  report_code VARCHAR(16) NOT NULL DEFAULT '',
  period_end DATE NULL DEFAULT NULL,

  revenue BIGINT NULL DEFAULT NULL,
  operating_income BIGINT NULL DEFAULT NULL,
  net_income BIGINT NULL DEFAULT NULL,
  eps_basic DECIMAL(18,4) NULL DEFAULT NULL,

  total_assets BIGINT NULL DEFAULT NULL,
  total_liabilities BIGINT NULL DEFAULT NULL,
  total_equity BIGINT NULL DEFAULT NULL,
  current_assets BIGINT NULL DEFAULT NULL,
  current_liabilities BIGINT NULL DEFAULT NULL,

  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,

  PRIMARY KEY (id),
  UNIQUE KEY uk_financial_period (symbol, basis, fiscal_year, period_index),
  KEY idx_financial_symbol_period (symbol, basis, fiscal_year, period_index)
) ENGINE=InnoDB;

-- ── 2. 공시 ──────────────────────────────────────────────────────────────
-- `/disclosures/today`는 **보유 종목 수만큼** DART를 불렀다. 사용자 수 × 보유
-- 종목 수로 호출이 늘어나는 유일한 경로였다.
--
-- 적재는 종목별이 아니라 **시장 전체 일별 스윕**이다. DART list.json은 corp_code
-- 없이 날짜만으로 그날 접수분 전체를 주고(2026-08-18 실측 397건), page_count=100
-- 이면 하루가 4호출이다 — 종목별로 돌면 4,400호출이 드는 일을 4번으로 끝낸다.
CREATE TABLE IF NOT EXISTS stock_disclosures (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  symbol VARCHAR(20) NOT NULL,
  -- DART 접수번호. 이것이 자연키라 재실행이 행을 복제하지 않는다.
  receipt_no VARCHAR(32) NOT NULL,
  receipt_date DATE NOT NULL,
  title VARCHAR(500) NOT NULL,
  -- 제출인(공시 주체). 종목명과 다를 수 있다 — 예: '코스닥시장본부'.
  filer VARCHAR(200) NOT NULL DEFAULT '',
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,

  PRIMARY KEY (id),
  UNIQUE KEY uk_disclosure_receipt (receipt_no),
  -- 화면 질의: 종목 목록 × 날짜.
  KEY idx_disclosure_symbol_date (symbol, receipt_date),
  KEY idx_disclosure_date (receipt_date)
) ENGINE=InnoDB;

-- ── 3. 종목 경고 ─────────────────────────────────────────────────────────
-- 투자주의·투자경고·거래정지 등 지정 상태. 종목당 Toss 1호출이라 조회 경로에
-- 두면 종목 수에 비례해 호출이 늘었다.
--
-- **없음도 저장한다**(sync_state의 'empty'). 대부분의 종목은 경고가 없는데,
-- 그 사실을 기록하지 않으면 배치가 매일 같은 종목을 "아직 모른다"며 다시 묻는다.
CREATE TABLE IF NOT EXISTS stock_warnings (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  symbol VARCHAR(20) NOT NULL,
  warning_type VARCHAR(64) NOT NULL,
  -- 아래 셋은 소스 응답을 그대로 보관한다. 날짜를 DATE로 파싱하지 않는 것은
  -- 의도적이다 — 형식이 우리 통제 밖이고, 파싱에 실패하면 그 종목의 경고가
  -- 통째로 사라진다. 화면은 이 값을 표시만 한다.
  exchange VARCHAR(32) NULL DEFAULT NULL,
  start_date VARCHAR(20) NULL DEFAULT NULL,
  end_date VARCHAR(20) NULL DEFAULT NULL,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,

  PRIMARY KEY (id),
  UNIQUE KEY uk_stock_warning (symbol, warning_type),
  KEY idx_stock_warning_symbol (symbol)
) ENGINE=InnoDB;
