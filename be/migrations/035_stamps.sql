USE quantport;

-- 스탬프 (docs/STAMP_DESIGN.md)
--
-- 사용자가 "지금 이 종목을 이렇게 보고 있다"를 찍어두는 불변 스냅샷 + 가벼운 가설.
-- 스냅샷 컬럼/JSON 은 생성 이후 절대 UPDATE 하지 않는다 (수정 가능한 것은 메모/태그/
-- 관점/검증기간뿐). 스냅샷이 바뀌면 적중률 통계가 사후 조작 가능해지기 때문이다.
-- (마이그레이션 분리기가 세미콜론으로 자르므로 주석에 세미콜론을 쓰지 않는다)

CREATE TABLE IF NOT EXISTS stamps (
  id              BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
  user_id         BIGINT UNSIGNED NOT NULL,
  symbol          VARCHAR(32)   NOT NULL,
  symbol_name     VARCHAR(100)  NOT NULL DEFAULT '',
  asset_type      VARCHAR(16)   NOT NULL DEFAULT '',
  currency        CHAR(3)       NOT NULL DEFAULT 'KRW',
  -- 저장은 UTC, 표시는 항상 Asia/Seoul
  stamped_at      TIMESTAMP(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  -- 수익률 기준 거래일 (시장 로컬 기준)
  trade_date      DATE          NOT NULL,
  market_session  VARCHAR(16)   NOT NULL DEFAULT '',
  -- 알파 계산에 쓸 벤치마크 지수 (kospi | kosdaq | sp500 ...). 생성 시 고정
  benchmark_id    VARCHAR(32)   NOT NULL DEFAULT '',
  price           DOUBLE        NOT NULL DEFAULT 0,
  prev_close      DOUBLE        NULL,
  change_rate     DOUBLE        NULL,
  volume          BIGINT        NOT NULL DEFAULT 0,
  score           TINYINT       NULL,
  grade           VARCHAR(2)    NULL,
  -- 사용자의 가설
  stance          VARCHAR(8)    NOT NULL DEFAULT 'neutral',
  conviction      TINYINT       NOT NULL DEFAULT 2,
  horizon_days    SMALLINT      NOT NULL DEFAULT 30,
  note            VARCHAR(1000) NOT NULL DEFAULT '',
  tags_json       JSON          NULL,
  -- 당시 시세/밸류에이션/점수 breakdown/보유·관심 상태
  snapshot        JSON          NOT NULL,
  -- 당시 뉴스/공시/지수/AI 요약/dataGaps
  context         JSON          NOT NULL,
  -- { interval, bars: [...] } 최대 200봉
  chart           JSON          NOT NULL,
  -- 개봉(만기 후 사용자가 확인) 시각
  opened_at       TIMESTAMP(3)  NULL,
  created_at      TIMESTAMP(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at      TIMESTAMP(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  KEY idx_stamps_user_time (user_id, stamped_at DESC),
  KEY idx_stamps_user_symbol (user_id, symbol, stamped_at DESC),
  KEY idx_stamps_user_open (user_id, opened_at, stamped_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
