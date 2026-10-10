-- 투자 리포트 「AI 검토 노트」 (2026-09-27 사용자 결정 C ④)
--
-- 공개 리포트마다 **주장과 우리 데이터가 맞는가**를 대조한 노트 하나. 게시·수정 뒤 다음 배치
-- 회차(30분)에 만든다. 작성자 줄 세우기와 무관하다 — 각 리포트 옆에만 붙는다.
-- body_rev는 검토한 본문 판이다 — 작성자가 고치면 다시 검토한다.

USE quantport;

CREATE TABLE IF NOT EXISTS research_reviews (
  call_id      BIGINT UNSIGNED NOT NULL PRIMARY KEY,
  body_rev     INT             NOT NULL DEFAULT 1,
  body         JSON            NOT NULL,
  source       VARCHAR(8)      NOT NULL DEFAULT 'rule',
  model        VARCHAR(64)     NOT NULL DEFAULT '',
  tokens       INT             NOT NULL DEFAULT 0,
  generated_at TIMESTAMP(3)    NOT NULL DEFAULT CURRENT_TIMESTAMP(3)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
