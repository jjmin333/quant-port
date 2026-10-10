-- 커뮤니티 (2026-09-27 사용자 요청 4 · 결정: docs/mockups/community-ai-v1.html)
--
-- 한 피드 + 종목 태그 필터. 글에 **종목·기사를 붙이면** 우리 데이터(점수·AI 요약·뉴스·일정)가
-- 함께 보인다. 붙일 때의 값(가격·점수·등락)은 **스냅샷**으로 얼려 두고, 읽을 때의 값과 나란히
-- 「그때 → 지금」으로 보여 준다.
--
-- 카운터(좋아요·댓글·조회·신고)는 글 행에 **비정규화**해 둔다. 목록 한 장에 20건인데 건마다
-- COUNT를 돌리면 DB(1 ECPU)가 먼저 버틴다. 원장(reactions·views·reports)은 중복 방지용이다.
--
-- 리포트(research_*)와 표를 나누는 이유: 리포트는 판정·성적이 붙는 공개 기록이라 지울 수 없고
-- 규칙이 다르다. 커뮤니티 글은 지울 수 있는 대화다.

USE quantport;

CREATE TABLE IF NOT EXISTS community_posts (
  id            BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
  user_id       BIGINT UNSIGNED NOT NULL,
  title         VARCHAR(100)    NOT NULL,
  body          TEXT            NOT NULL,
  -- visible | hidden(신고 누적·관리자) | deleted(작성자)
  status        VARCHAR(16)     NOT NULL DEFAULT 'visible',
  hidden_reason VARCHAR(32)     NOT NULL DEFAULT '',
  like_count    INT UNSIGNED    NOT NULL DEFAULT 0,
  comment_count INT UNSIGNED    NOT NULL DEFAULT 0,
  view_count    INT UNSIGNED    NOT NULL DEFAULT 0,
  report_count  INT UNSIGNED    NOT NULL DEFAULT 0,
  created_at    TIMESTAMP(3)    NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at    TIMESTAMP(3)    NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  edited_at     TIMESTAMP(3)    NULL,
  KEY idx_cp_status_created (status, created_at, id),
  KEY idx_cp_user_created (user_id, created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- 붙인 종목 — 붙일 때의 값을 얼려 둔다.
CREATE TABLE IF NOT EXISTS community_post_symbols (
  post_id          BIGINT UNSIGNED NOT NULL,
  symbol           VARCHAR(32)     NOT NULL,
  position         TINYINT UNSIGNED NOT NULL DEFAULT 0,
  snap_price       DOUBLE          NULL,
  snap_currency    CHAR(3)         NOT NULL DEFAULT '',
  snap_change_rate DOUBLE          NULL,
  snap_score       SMALLINT        NULL,
  snap_at          TIMESTAMP(3)    NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (post_id, symbol),
  KEY idx_cps_symbol (symbol, post_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- 붙인 기사 — 기사 id로 news_articles와 조인하므로 그 표와 같은 collation(unicode_ci).
-- 제목·출처는 복사해 둔다: 기사 정리 배치가 원문 행을 지워도 글은 온전해야 한다.
CREATE TABLE IF NOT EXISTS community_post_articles (
  post_id      BIGINT UNSIGNED NOT NULL,
  article_id   VARCHAR(64)     NOT NULL,
  position     TINYINT UNSIGNED NOT NULL DEFAULT 0,
  title        VARCHAR(300)    NOT NULL DEFAULT '',
  source       VARCHAR(100)    NOT NULL DEFAULT '',
  url          TEXT            NULL,
  published_at TIMESTAMP(3)    NULL,
  PRIMARY KEY (post_id, article_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS community_post_images (
  post_id  BIGINT UNSIGNED  NOT NULL,
  position TINYINT UNSIGNED NOT NULL,
  url      VARCHAR(512)     NOT NULL,
  width    INT              NOT NULL DEFAULT 0,
  height   INT              NOT NULL DEFAULT 0,
  PRIMARY KEY (post_id, position)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS community_comments (
  id           BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
  post_id      BIGINT UNSIGNED NOT NULL,
  parent_id    BIGINT UNSIGNED NULL,
  user_id      BIGINT UNSIGNED NOT NULL,
  body         VARCHAR(1000)   NOT NULL,
  status       VARCHAR(16)     NOT NULL DEFAULT 'visible',
  like_count   INT UNSIGNED    NOT NULL DEFAULT 0,
  report_count INT UNSIGNED    NOT NULL DEFAULT 0,
  created_at   TIMESTAMP(3)    NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at   TIMESTAMP(3)    NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  KEY idx_cc_post (post_id, created_at),
  KEY idx_cc_user (user_id, created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- 좋아요 원장 — target_type: post | comment.
CREATE TABLE IF NOT EXISTS community_reactions (
  target_type VARCHAR(8)      NOT NULL,
  target_id   BIGINT UNSIGNED NOT NULL,
  user_id     BIGINT UNSIGNED NOT NULL,
  created_at  TIMESTAMP(3)    NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (target_type, target_id, user_id),
  KEY idx_cr_user (user_id, created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- 조회 원장 — 글마다 보는 사람 하나에 한 번. user_id는 로그인한 사람만(「눈팅 장인」 뱃지 재료).
CREATE TABLE IF NOT EXISTS community_views (
  post_id    BIGINT UNSIGNED NOT NULL,
  viewer_key CHAR(64)        NOT NULL,
  user_id    BIGINT UNSIGNED NULL,
  viewed_at  TIMESTAMP(3)    NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (post_id, viewer_key),
  KEY idx_cv_user (user_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- 신고 — 한 사람이 같은 대상을 두 번 신고할 수 없다. 서로 다른 3명이면 자동 숨김.
CREATE TABLE IF NOT EXISTS community_reports (
  id          BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
  target_type VARCHAR(8)      NOT NULL,
  target_id   BIGINT UNSIGNED NOT NULL,
  reporter_id BIGINT UNSIGNED NOT NULL,
  reason      VARCHAR(200)    NOT NULL DEFAULT '',
  created_at  TIMESTAMP(3)    NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  UNIQUE KEY uniq_crp_target_reporter (target_type, target_id, reporter_id),
  KEY idx_crp_created (created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
