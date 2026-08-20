-- 종목 종합 점수 (docs/STOCK_SCORE_DESIGN.md)
--
-- 3개 테이블:
--   score_settings        전역 기본 가중치 (단일 행, market_data_settings/llm_settings 와 동일 패턴)
--   score_symbol_weights  종목별 가중치 오버라이드 (있으면 전역 대신 사용)
--   stock_scores          배치 산출물 (symbol, trade_date) 멱등 UPSERT

CREATE TABLE IF NOT EXISTS score_settings (
  id                TINYINT       NOT NULL PRIMARY KEY DEFAULT 1,
  w_technical       SMALLINT      NOT NULL DEFAULT 30,
  w_quality         SMALLINT      NOT NULL DEFAULT 25,
  w_growth          SMALLINT      NOT NULL DEFAULT 20,
  w_valuation       SMALLINT      NOT NULL DEFAULT 15,
  w_sentiment       SMALLINT      NOT NULL DEFAULT 10,
  -- coverage < floor 이면 점수를 내지 않는다 (NULL). shrink_full 이상이면 수축 없음.
  coverage_floor    DECIMAL(4,3)  NOT NULL DEFAULT 0.200,
  shrink_full       DECIMAL(4,3)  NOT NULL DEFAULT 0.600,
  updated_at        TIMESTAMP(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  CONSTRAINT chk_score_settings_single_row CHECK (id = 1)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

INSERT INTO score_settings (id) VALUES (1)
  ON DUPLICATE KEY UPDATE id = id;

CREATE TABLE IF NOT EXISTS score_symbol_weights (
  symbol       VARCHAR(32)  NOT NULL PRIMARY KEY,
  w_technical  SMALLINT     NOT NULL,
  w_quality    SMALLINT     NOT NULL,
  w_growth     SMALLINT     NOT NULL,
  w_valuation  SMALLINT     NOT NULL,
  w_sentiment  SMALLINT     NOT NULL,
  note         VARCHAR(255) NOT NULL DEFAULT '',
  updated_at   TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS stock_scores (
  symbol       VARCHAR(32)  NOT NULL,
  trade_date   DATE         NOT NULL,
  -- 최종 0~100 (커버리지 수축 적용 후). 계산 불가 시 NULL → FE 는 '-'
  score        TINYINT      NULL,
  score_kind   VARCHAR(16)  NOT NULL DEFAULT 'composite',  -- composite | technical_only
  grade        VARCHAR(2)   NULL,
  coverage     DECIMAL(4,3) NOT NULL DEFAULT 0,
  peer_group   VARCHAR(64)  NOT NULL DEFAULT '',
  peer_pct     TINYINT      NULL,
  -- 축별 원본 점수. 가중치를 바꿔도 과거를 재계산할 수 있도록 그대로 보관
  breakdown    JSON         NOT NULL,
  -- 사용된 원지표 실수치 (사후 감사용 — 서버 계산이므로 완전 재현 가능)
  inputs       JSON         NOT NULL,
  computed_at  TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (symbol, trade_date),
  KEY idx_stock_scores_date (trade_date),
  KEY idx_stock_scores_score (trade_date, score)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
