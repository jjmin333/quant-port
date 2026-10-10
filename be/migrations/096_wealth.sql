-- 순자산(Wealth) 확장 (2026-10-01 사용자 결정 — docs/ASSET_EXPANSION_RESEARCH.md §7·§9 「전체 개발 일괄」)
--
-- 사용자 자산·부채
--   user_assets       시세 기호가 없는 자산 — 부동산·예금/적금·채권·코인·금·자동차·미술품·기타.
--                     주식·ETF는 지금처럼 user_holdings에 있다(여기에 중복하지 않는다).
--                     자산마다 다른 입력(주소·면적 / 금리·만기 / 코인 마켓 …)은 detail JSON.
--   asset_valuations  평가 이력. 하루 한 줄(asset_id, as_of). kind로 「시장가 · 최근 실거래 · 추정 ·
--                     직접 입력 · 원리금 계산」을 가른다 — 화면이 숫자의 성격을 이름으로 밝힌다.
--   user_liabilities  부채(주담대 · 받은 전세보증금 · 신용 · 기타). asset_id로 담보 자산에 붙일 수 있다.
--
-- 공공데이터 적재 (worker-filings, 조회는 표만)
--   re_regions        시군구(법정동 앞 5자리). tier 'all'은 아파트 매매·전월세만, 'full'은 12종 전부.
--   re_trades         국토부 실거래가(매매·전월세 12종). (종류, 시군구, 계약월) 단위로 **통째로 갈아 끼운다** —
--                     원천에 행 id가 없고 해제·정정이 뒤늦게 반영되기 때문. 금액은 만원 단위 정수(원천 단위 그대로).
--   re_sync_state     (종류, 시군구, 계약월)의 마지막 성공 시각 — 로테이션 커서.
--   re_index          한국부동산원 R-ONE 통계(주간·월간·분기 지수 등). (표, 지역, 항목, 시점) 한 값.
--   macro_rates       한국은행 ECOS 금리(일·월).
--   onbid_items       캠코 온비드 부동산 공매 물건 + 입찰 결과.
-- (마이그레이션 분리기가 세미콜론으로 자르므로 주석에 세미콜론을 쓰지 않는다)

USE quantport;

CREATE TABLE IF NOT EXISTS user_assets (
  id               BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
  user_id          BIGINT UNSIGNED NOT NULL,
  -- real_estate · deposit · saving · bond · crypto · gold · vehicle · art · other
  asset_type       VARCHAR(16)   NOT NULL,
  name             VARCHAR(100)  NOT NULL,
  currency         CHAR(3)       NOT NULL DEFAULT 'KRW',
  -- 코인 개수 · 금 그램. 그 외는 NULL.
  quantity         DECIMAL(28, 10) NULL,
  acquired_at      DATE          NULL,
  -- 취득 총액(원). 지분율을 곱하기 전 전체 금액.
  acquired_price   DECIMAL(20, 2) NULL,
  ownership_ratio  DECIMAL(7, 6) NOT NULL DEFAULT 1,
  -- comparable · region_unit · fixed_income · bond · market_price · user_estimated
  valuation_method VARCHAR(24)   NOT NULL,
  -- 사용자가 직접 넣은 현재가(원, 전체 금액). 방법이 user_estimated면 이것이 평가액.
  user_value       DECIMAL(20, 2) NULL,
  detail           JSON          NOT NULL,
  memo             VARCHAR(300)  NOT NULL DEFAULT '',
  created_at       TIMESTAMP(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at       TIMESTAMP(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  KEY idx_user_assets_user (user_id),
  KEY idx_user_assets_type (asset_type),
  CONSTRAINT fk_user_assets_user FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS asset_valuations (
  asset_id    BIGINT UNSIGNED NOT NULL,
  as_of       DATE          NOT NULL,
  -- 전체 금액(원, 지분율 곱하기 전).
  value       DECIMAL(20, 2) NOT NULL,
  low         DECIMAL(20, 2) NULL,
  high        DECIMAL(20, 2) NULL,
  -- market · last_trade · estimate · user_input · accrual
  kind        VARCHAR(16)   NOT NULL,
  -- high · medium · low (추정만)
  confidence  VARCHAR(8)    NOT NULL DEFAULT '',
  source      VARCHAR(80)   NOT NULL DEFAULT '',
  inputs      JSON          NULL,
  created_at  TIMESTAMP(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (asset_id, as_of),
  CONSTRAINT fk_asset_valuations_asset FOREIGN KEY (asset_id) REFERENCES user_assets (id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS user_liabilities (
  id              BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
  user_id         BIGINT UNSIGNED NOT NULL,
  asset_id        BIGINT UNSIGNED NULL,
  -- mortgage · jeonse_deposit · credit · other
  kind            VARCHAR(16)   NOT NULL,
  name            VARCHAR(100)  NOT NULL,
  principal       DECIMAL(20, 2) NOT NULL DEFAULT 0,
  -- 사용자가 넣은 현재 잔액.
  balance         DECIMAL(20, 2) NOT NULL DEFAULT 0,
  -- 연 %. 전세보증금은 0.
  rate            DECIMAL(7, 4) NULL,
  -- fixed · variable · mixed
  rate_type       VARCHAR(12)   NOT NULL DEFAULT '',
  -- equal_payment(원리금균등) · equal_principal(원금균등) · bullet(만기일시)
  repayment_type  VARCHAR(16)   NOT NULL DEFAULT '',
  started_at      DATE          NULL,
  maturity_at     DATE          NULL,
  memo            VARCHAR(300)  NOT NULL DEFAULT '',
  created_at      TIMESTAMP(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at      TIMESTAMP(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  KEY idx_user_liabilities_user (user_id),
  CONSTRAINT fk_user_liabilities_user FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS re_regions (
  lawd_cd    CHAR(5)       NOT NULL PRIMARY KEY,
  sido       VARCHAR(20)   NOT NULL,
  name       VARCHAR(40)   NOT NULL,
  -- all(아파트 매매·전월세) · full(12종)
  tier       VARCHAR(8)    NOT NULL DEFAULT 'all',
  -- R-ONE 지역 분류 id (주간 지수의 CLS_ID). 못 찾으면 NULL.
  rone_cls_id INT          NULL,
  updated_at TIMESTAMP(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS re_trades (
  id            BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
  -- apt · rh(연립다세대) · sh(단독다가구) · offi · land · nrg(상업업무용) · indu(공장창고) · silv(분양권)
  kind          VARCHAR(6)    NOT NULL,
  -- sale · rent
  deal_type     VARCHAR(4)    NOT NULL,
  lawd_cd       CHAR(5)       NOT NULL,
  deal_ym       CHAR(6)       NOT NULL,
  deal_date     DATE          NOT NULL,
  umd_nm        VARCHAR(40)   NOT NULL DEFAULT '',
  jibun         VARCHAR(40)   NOT NULL DEFAULT '',
  -- 아파트는 aptSeq, 그 밖은 「동|지번|이름」 — 같은 건물을 묶는 열쇠.
  complex_key   VARCHAR(120)  NOT NULL DEFAULT '',
  complex_name  VARCHAR(100)  NOT NULL DEFAULT '',
  -- 주택유형 · 용도지역 · 건물주용도 등 종류별 분류.
  category      VARCHAR(40)   NOT NULL DEFAULT '',
  -- 전용면적(주택) · 연면적(상업·공장) · 거래면적(토지), ㎡
  area          DECIMAL(10, 2) NOT NULL DEFAULT 0,
  land_area     DECIMAL(12, 2) NULL,
  floor         SMALLINT      NULL,
  build_year    SMALLINT      NULL,
  -- 만원. 매매가 / 보증금 / 월세.
  price         INT UNSIGNED  NULL,
  deposit       INT UNSIGNED  NULL,
  monthly_rent  INT UNSIGNED  NULL,
  -- 신규 · 갱신 (전월세)
  contract_type VARCHAR(8)    NOT NULL DEFAULT '',
  direct        TINYINT(1)    NOT NULL DEFAULT 0,
  cancelled     TINYINT(1)    NOT NULL DEFAULT 0,
  KEY idx_re_trades_slice (lawd_cd, kind, deal_type, deal_ym),
  KEY idx_re_trades_complex (complex_key, deal_type, deal_date),
  KEY idx_re_trades_umd (lawd_cd, kind, umd_nm, deal_date)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS re_sync_state (
  kind       VARCHAR(6)    NOT NULL,
  deal_type  VARCHAR(4)    NOT NULL,
  lawd_cd    CHAR(5)       NOT NULL,
  deal_ym    CHAR(6)       NOT NULL,
  rows_n     INT           NOT NULL DEFAULT 0,
  synced_at  TIMESTAMP(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (kind, deal_type, lawd_cd, deal_ym)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS re_index (
  statbl_id   VARCHAR(24)   NOT NULL,
  cls_id      INT           NOT NULL,
  itm_id      INT           NOT NULL,
  -- 원천 시점 id(202639 · 202608 · 2026Q2 …)
  period      VARCHAR(10)   NOT NULL,
  period_date DATE          NOT NULL,
  cls_name    VARCHAR(40)   NOT NULL DEFAULT '',
  cls_full    VARCHAR(100)  NOT NULL DEFAULT '',
  itm_name    VARCHAR(40)   NOT NULL DEFAULT '',
  value       DOUBLE        NOT NULL,
  PRIMARY KEY (statbl_id, cls_id, itm_id, period),
  KEY idx_re_index_cls (cls_id, statbl_id, period_date),
  -- 지역 이름 → 분류 id 찾기(화면이 시군구 이름으로 지수를 찾는다).
  KEY idx_re_index_name (statbl_id, cls_name)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS macro_rates (
  -- 표/항목 (예: 817Y002/010200000)
  series_id   VARCHAR(40)   NOT NULL,
  period      VARCHAR(8)    NOT NULL,
  period_date DATE          NOT NULL,
  value       DOUBLE        NOT NULL,
  fetched_at  TIMESTAMP(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (series_id, period),
  KEY idx_macro_rates_date (series_id, period_date)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS onbid_items (
  cltr_mng_no     VARCHAR(40)   NOT NULL,
  pbct_cdtn_no    BIGINT        NOT NULL DEFAULT 0,
  pbanc_mng_no    VARCHAR(40)   NOT NULL,
  onbid_cltr_no   BIGINT        NULL,
  name            VARCHAR(300)  NOT NULL DEFAULT '',
  address         VARCHAR(300)  NOT NULL DEFAULT '',
  sido            VARCHAR(20)   NOT NULL DEFAULT '',
  usage_l         VARCHAR(40)   NOT NULL DEFAULT '',
  usage_m         VARCHAR(40)   NOT NULL DEFAULT '',
  usage_s         VARCHAR(40)   NOT NULL DEFAULT '',
  prpt_div        VARCHAR(20)   NOT NULL DEFAULT '',
  -- 매각 · 임대
  dsps_method     VARCHAR(8)    NOT NULL DEFAULT '',
  org_name        VARCHAR(100)  NOT NULL DEFAULT '',
  appraisal       BIGINT        NULL,
  min_bid         BIGINT        NULL,
  min_bid_ratio   DOUBLE        NULL,
  fail_count      INT           NOT NULL DEFAULT 0,
  bid_start       DATETIME      NULL,
  bid_end         DATETIME      NULL,
  opbd_at         DATETIME      NULL,
  -- 결과: 낙찰 · 유찰 · 취소 · ''(미정)
  result          VARCHAR(10)   NOT NULL DEFAULT '',
  winning_bid     BIGINT        NULL,
  valid_bidders   INT           NULL,
  result_checked  TINYINT(1)    NOT NULL DEFAULT 0,
  fetched_at      TIMESTAMP(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (cltr_mng_no, pbct_cdtn_no),
  KEY idx_onbid_items_open (opbd_at),
  KEY idx_onbid_items_pbanc (pbanc_mng_no)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- 공고 단위 진행 상태 — 공고상세(물건) 호출을 공고당 한 번만 하게.
CREATE TABLE IF NOT EXISTS onbid_notices (
  pbanc_mng_no  VARCHAR(40)   NOT NULL PRIMARY KEY,
  name          VARCHAR(300)  NOT NULL DEFAULT '',
  org_name      VARCHAR(100)  NOT NULL DEFAULT '',
  prpt_div      VARCHAR(20)   NOT NULL DEFAULT '',
  opbd_at       DATETIME      NULL,
  detail_done   TINYINT(1)    NOT NULL DEFAULT 0,
  fetched_at    TIMESTAMP(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  KEY idx_onbid_notices_todo (detail_done, opbd_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
