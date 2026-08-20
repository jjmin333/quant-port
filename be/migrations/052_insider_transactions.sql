-- 내부자 거래 — DART 임원ㆍ주요주주 소유보고(국내) · SEC Form 4(미국).
--
-- 이 표가 생기기 전에는 종목 탭을 누를 때마다 EDGAR/DART를 직접 불렀다. EDGAR는
-- 종목 하나에 최소 4호출(submissions 1 + Form4 XML 3)이 들었고, 그 전부가
-- 8/min짜리 공유 버킷을 통과해야 했다 — 동시 접속 두 명이면 두 번째 사용자는
-- 아무 데이터도 못 받는다. 게다가 최근 3건만 읽었기 때문에, 그 3건이 전부
-- 파생거래(옵션·RSU)인 종목은 **정상 응답으로 빈 화면**을 받았다.
--
-- 적재는 워커가, 조회는 이 표만. 사용자 수가 늘어도 외부 호출은 늘지 않는다.
CREATE TABLE IF NOT EXISTS insider_transactions (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  symbol VARCHAR(20) NOT NULL,
  source VARCHAR(8) NOT NULL,                      -- 'dart' | 'edgar'

  -- 한 건의 원 공시를 가리키는 키. EDGAR는 accession number를 그대로 쓰고,
  -- DART elestock은 접수번호를 주지 않으므로 행 내용을 해시해 만든다
  -- (같은 보고가 다시 와도 같은 키가 나와야 재실행이 행을 복제하지 않는다).
  filing_key VARCHAR(64) NOT NULL,
  -- 한 공시 안의 몇 번째 거래인가. Form 4 하나는 비파생 거래를 여러 줄 담는다.
  line_no SMALLINT UNSIGNED NOT NULL DEFAULT 0,

  -- 거래일(Form 4) 또는 보고 접수일(DART). 두 소스가 말하는 날짜의 의미가
  -- 다르지만 화면은 "언제"만 쓰므로 한 컬럼으로 모은다.
  txn_date DATE NOT NULL,
  reporter_name VARCHAR(255) NOT NULL DEFAULT '',
  relation VARCHAR(255) NOT NULL DEFAULT '',
  transaction_type VARCHAR(64) NOT NULL DEFAULT '',

  -- DART는 시점 스냅샷(보유수량 + 직전 대비 증감), Form 4는 개별 거래
  -- (수량·단가·거래후 보유수량)를 준다. 서로 채우는 칸이 달라 전부 NULL 허용.
  shares_change BIGINT NULL DEFAULT NULL,
  change_rate DECIMAL(14,4) NULL DEFAULT NULL,
  shares_owned_after BIGINT NULL DEFAULT NULL,
  ownership_rate DECIMAL(14,4) NULL DEFAULT NULL,
  price_per_share DECIMAL(20,6) NULL DEFAULT NULL,

  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,

  PRIMARY KEY (id),
  UNIQUE KEY uk_insider_row (symbol, source, filing_key, line_no),
  -- 화면의 유일한 질의: 종목 하나의 최근 N건.
  KEY idx_insider_symbol_date (symbol, txn_date)
) ENGINE=InnoDB;

-- 종목별 적재 상태 — **모든 종목 단위 배치가 공유한다**(내부자·재무이력·경고).
--
-- 한 표에 kind로 모은 이유: 배치마다 상태 표를 따로 두면 "다음에 누구를 받을지"
-- 고르는 SQL이 배치 수만큼 복제된다. 그 SQL이 등급제의 전부라서, 복제되면
-- 우선순위 규칙이 잡마다 조용히 갈라진다.
--
-- 이 표가 필요한 이유 자체는 따로 있다: 데이터 표만 봐서는 "아직 안 받은 종목"과
-- "받았는데 내역이 없는 종목"을 구분할 수 없다. 구분하지 못하면 배치가 매번 같은
-- 빈 종목들을 다시 조회하며 한도를 태우고, 화면은 수집 전과 내역 없음을 똑같이
-- 빈 표로 보여준다.
CREATE TABLE IF NOT EXISTS symbol_sync_state (
  -- 'insider_dart' | 'insider_edgar' | 'financials_dart' | 'financials_edgar' | 'warnings'
  kind VARCHAR(24) NOT NULL,
  symbol VARCHAR(20) NOT NULL,

  -- 성패와 무관한 마지막 시도 시각. 로테이션 순서(오래된 것부터)의 기준이다.
  last_synced_at TIMESTAMP NULL DEFAULT NULL,
  last_success_at TIMESTAMP NULL DEFAULT NULL,

  -- ok        — 받았고 내용이 있다
  -- empty     — 받았는데 해당 내역이 없다 (정상)
  -- error     — 일시적 실패(429·타임아웃 등). 다음 로테이션에서 재시도.
  -- not_found — 소스에 이 종목이 없다(EDGAR CIK 없음 등). 30일간 건너뛴다.
  status VARCHAR(12) NOT NULL DEFAULT 'ok',
  message VARCHAR(255) NOT NULL DEFAULT '',
  rows_stored INT NOT NULL DEFAULT 0,
  items_seen INT NOT NULL DEFAULT 0,

  PRIMARY KEY (kind, symbol),
  KEY idx_symbol_sync_due (kind, last_synced_at)
) ENGINE=InnoDB;
