USE quantport;

-- 리서치 콜 (docs/RESEARCH_CALL_DESIGN.md)
--
-- 공개 종목 논증 + 검증 가능한 예측.
-- 발행 시점의 콜 값(방향/기준가/목표가/손절가/기간/확신도)과 서버가 수집한 근거
-- 스냅샷은 발행 이후 절대 UPDATE 하지 않는다. 수정 가능한 것은 본문/태그/공개범위뿐이고
-- 본문 수정은 research_revisions 에 이력이 남는다.
-- 철회는 삭제가 아니라 status='retracted' 이며 통계에서 빠지지 않는다 -- 지울 수 있으면
-- 리더보드가 성적표가 아니라 광고판이 되기 때문이다.
-- (마이그레이션 분리기가 세미콜론으로 자르므로 주석에 세미콜론을 쓰지 않는다)

CREATE TABLE IF NOT EXISTS research_calls (
  id             BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
  user_id        BIGINT UNSIGNED NOT NULL,
  symbol         VARCHAR(32)   NOT NULL,
  symbol_name    VARCHAR(100)  NOT NULL DEFAULT '',
  asset_type     VARCHAR(16)   NOT NULL DEFAULT '',
  currency       CHAR(3)       NOT NULL DEFAULT 'KRW',
  title          VARCHAR(200)  NOT NULL DEFAULT '',
  -- 저장은 UTC, 표시는 항상 Asia/Seoul
  published_at   TIMESTAMP(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  -- 수익률 기준 거래일 (시장 로컬 기준)
  trade_date     DATE          NOT NULL,
  -- 알파 계산에 쓸 벤치마크 지수. 발행 시 고정
  benchmark_id   VARCHAR(32)   NOT NULL DEFAULT '',
  -- 불변 콜 값
  direction      VARCHAR(8)    NOT NULL DEFAULT 'buy',
  base_price     DOUBLE        NOT NULL DEFAULT 0,
  target_price   DOUBLE        NULL,
  stop_price     DOUBLE        NULL,
  horizon_days   SMALLINT      NOT NULL DEFAULT 90,
  conviction     TINYINT UNSIGNED NOT NULL DEFAULT 50,
  score_at_pub   TINYINT       NULL,
  grade_at_pub   VARCHAR(2)    NULL,
  -- 발행 시 우리 점수와 방향이 어긋났는가
  contrarian     TINYINT(1)    NOT NULL DEFAULT 0,
  -- 서버가 수집한 근거 (시세/밸류에이션/점수 breakdown/룰기반 브리핑)
  evidence       JSON          NOT NULL,
  -- 당시 뉴스/공시/지수/AI 요약/dataGaps
  context        JSON          NOT NULL,
  -- { interval, bars: [...] } 최대 200봉
  chart          JSON          NOT NULL,
  -- 가변
  body_md        MEDIUMTEXT    NOT NULL,
  body_rev       INT           NOT NULL DEFAULT 1,
  tags_json      JSON          NULL,
  visibility     VARCHAR(12)   NOT NULL DEFAULT 'public',
  status         VARCHAR(20)   NOT NULL DEFAULT 'open',
  verdict_at     TIMESTAMP(3)  NULL,
  verdict_date   DATE          NULL,
  verdict_return DOUBLE        NULL,
  verdict_alpha  DOUBLE        NULL,
  view_count     INT UNSIGNED  NOT NULL DEFAULT 0,
  retracted_at   TIMESTAMP(3)  NULL,
  -- 관리자 숨김 (신고 처리). 작성자 통계에는 남는다
  hidden_at      TIMESTAMP(3)  NULL,
  created_at     TIMESTAMP(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at     TIMESTAMP(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  KEY idx_rc_feed (visibility, hidden_at, published_at DESC),
  KEY idx_rc_user (user_id, published_at DESC),
  KEY idx_rc_symbol (symbol, published_at DESC),
  KEY idx_rc_pending (status, published_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- 본문 수정 이력. append-only 이며 관리 콘솔에서도 read-only 다
-- (수정 이력을 고칠 수 있으면 "수정 이력 공개"가 아무 의미가 없다)
CREATE TABLE IF NOT EXISTS research_revisions (
  id         BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
  call_id    BIGINT UNSIGNED NOT NULL,
  rev        INT           NOT NULL,
  title      VARCHAR(200)  NOT NULL DEFAULT '',
  body_md    MEDIUMTEXT    NOT NULL,
  created_at TIMESTAMP(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  UNIQUE KEY uniq_rr_call_rev (call_id, rev),
  KEY idx_rr_call (call_id, rev DESC)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- 신고. 소셜 기능보다 먼저 들어간다 (소형주 펌핑 유인이 구조적으로 생기므로)
CREATE TABLE IF NOT EXISTS research_reports (
  id          BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
  call_id     BIGINT UNSIGNED NOT NULL,
  user_id     BIGINT UNSIGNED NOT NULL,
  reason      VARCHAR(32)   NOT NULL DEFAULT 'other',
  detail      VARCHAR(500)  NOT NULL DEFAULT '',
  created_at  TIMESTAMP(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  UNIQUE KEY uniq_rrp_call_user (call_id, user_id),
  KEY idx_rrp_call (call_id, created_at DESC)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
