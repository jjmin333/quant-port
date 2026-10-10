-- 실적 「관전 포인트」 (2026-09-27 사용자 결정 C ② — 실적 메뉴에서 LLM 활용)
--
-- 향후 7일 안에 실적을 발표하는 종목마다 「이번 발표에서 무엇을 볼 것인가」를 매일 07:10에 만들어
-- 저장한다. 화면은 이 표만 읽는다(조회 경로 LLM 0 — llm_spend_rule).
--
-- body는 {headline, points[], risk} JSON. source가 'rule'이면 LLM을 못 불러(한도·503·꺼짐)
-- 규칙 문장으로 채운 것이다 — 화면이 「AI 생성」과 구분해 표시한다.
-- input_hash는 모델에 넘긴 사실표의 해시다. 같은 사실로 두 번 굽지 않는다.

USE quantport;

CREATE TABLE IF NOT EXISTS earnings_previews (
  symbol        VARCHAR(20)   NOT NULL,
  event_date    DATE          NOT NULL,
  market        VARCHAR(10)   NOT NULL,
  fiscal_label  VARCHAR(32)   NOT NULL DEFAULT '',
  body          JSON          NOT NULL,
  facts         JSON          NOT NULL,
  source        VARCHAR(8)    NOT NULL DEFAULT 'rule',
  model         VARCHAR(64)   NOT NULL DEFAULT '',
  input_hash    CHAR(64)      NOT NULL DEFAULT '',
  tokens        INT           NOT NULL DEFAULT 0,
  generated_at  TIMESTAMP(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (symbol, event_date),
  KEY idx_ep_market_date (market, event_date)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
