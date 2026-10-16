-- 봇 초안 · 게시 계정 (2026-10-16 사용자 요청)
--
-- bot_drafts — 3시간마다(하루 8번) 저장된 브리핑·종목 요약·실적 프리뷰·점수로 만든 커뮤니티·리포트 초안.
--   LLM을 부르지 않는다. 관리자가 고쳐서 게시 계정으로 올리면 status='published'와 올라간 글 id가 남는다.
--   쓰지 않은 초안(draft·discarded)은 다음 생성 때 24시간 지난 것부터 지운다. published는 남긴다 — 어느 글이
--   봇 초안에서 나왔는지의 기록이다.
-- users.badge — 게시 계정(role='persona')의 뱃지 문구. 관리자가 정한다. 일반 사용자는 빈 값.
--   게시 계정은 users의 일반 행이라 글·댓글·좋아요·작성자 화면이 고칠 것 없이 따라온다. 이메일은
--   `persona-…@persona.invalid`(.invalid는 가입·활동 지표에서 이미 빠진다), 비밀번호는 빈 값이라 로그인할 수 없다.

USE quantport;

CREATE TABLE IF NOT EXISTS bot_drafts (
  id            BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
  kind          VARCHAR(12)     NOT NULL,                 -- community | research
  slot_at       TIMESTAMP       NOT NULL,                 -- 생성 회차(KST 00·03·…·21시)
  symbol        VARCHAR(32)     NOT NULL,
  symbol_name   VARCHAR(100)    NOT NULL DEFAULT '',
  market        VARCHAR(16)     NOT NULL DEFAULT '',      -- domestic | us
  length        VARCHAR(8)      NOT NULL DEFAULT 'short', -- short | long
  reason        VARCHAR(200)    NOT NULL DEFAULT '',      -- 고른 이유(관리자 화면용)
  title         VARCHAR(200)    NOT NULL,
  body          MEDIUMTEXT      NOT NULL,
  articles      JSON            NOT NULL,                 -- [{id,title}] 붙일 최근 기사
  status        VARCHAR(12)     NOT NULL DEFAULT 'draft', -- draft | published | discarded
  published_ref BIGINT UNSIGNED NULL,                     -- community_posts.id · research_calls.id
  published_by  BIGINT UNSIGNED NULL,                     -- 게시 계정 users.id
  created_at    TIMESTAMP(3)    NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at    TIMESTAMP(3)    NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  KEY idx_bd_kind_status (kind, status, created_at),
  KEY idx_bd_symbol (symbol, created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

ALTER TABLE users ADD COLUMN badge VARCHAR(20) NOT NULL DEFAULT '' AFTER role;
