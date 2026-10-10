-- 부동산·금융상품 점수와 AI 해설 (2026-10-01 사용자 요청 — 주식처럼 점수·LLM으로 질 좋은 데이터)
--
--   asset_scores     규칙 점수(서버 계산, LLM 아님). kind별 키 하나에 최신 한 줄.
--                    re_complex(아파트 단지 aptSeq) · re_region(시군구) · fin_product(금융상품 id) · onbid(물건|조건)
--                    parts = 축별 0~100, facts = 화면이 근거로 보이는 원값.
--   asset_ai_notes   AI 해설(GEMINI_SUB_API_KEY 전용 버킷 llm_sub). 입력 해시가 같으면 다시 부르지 않는다.
--                    fin_conds(우대조건 구조화) · re_region · re_complex · rates(금리 한 줄)
--   fin_rate_history 금융상품 공시 월별 금리 — 「지난달보다 인상/인하」 표시용.
-- (마이그레이션 분리기가 세미콜론으로 자르므로 주석에 세미콜론을 쓰지 않는다)

USE quantport;

CREATE TABLE IF NOT EXISTS asset_scores (
  kind        VARCHAR(16)   NOT NULL,
  skey        VARCHAR(120)  NOT NULL,
  name        VARCHAR(120)  NOT NULL DEFAULT '',
  -- 묶음(시군구 lawd_cd · 시도 · 상품 종류) — 목록 필터·순위용.
  grp         VARCHAR(20)   NOT NULL DEFAULT '',
  as_of       DATE          NOT NULL,
  score       TINYINT UNSIGNED NOT NULL,
  grade       CHAR(1)       NOT NULL,
  parts       JSON          NOT NULL,
  facts       JSON          NOT NULL,
  updated_at  TIMESTAMP(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (kind, skey),
  KEY idx_asset_scores_rank (kind, grp, score)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS asset_ai_notes (
  kind        VARCHAR(16)   NOT NULL,
  skey        VARCHAR(120)  NOT NULL,
  input_hash  CHAR(64)      NOT NULL,
  body        JSON          NOT NULL,
  -- llm · rule
  source      VARCHAR(8)    NOT NULL,
  model       VARCHAR(60)   NOT NULL DEFAULT '',
  tokens      INT           NOT NULL DEFAULT 0,
  updated_at  TIMESTAMP(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (kind, skey)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS fin_rate_history (
  product_id  BIGINT        NOT NULL,
  dcls_month  CHAR(6)       NOT NULL,
  -- 예·적금은 12개월(없으면 가장 가까운) 기본·최고금리, 대출은 평균·최저·최고.
  base_rate   DOUBLE        NULL,
  max_rate    DOUBLE        NULL,
  PRIMARY KEY (product_id, dcls_month)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
