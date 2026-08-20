USE quantport;

-- 리서치 콜 소셜 레이어 (docs/RESEARCH_CALL_DESIGN.md §10, Phase 2~3)
--
-- 카운터는 research_calls 에 비정규화하지 않는다. 좋아요/댓글 수는 이 테이블들에서
-- 집계하고, research_calls.view_count 만 예외로 컬럼에 둔다(조회는 쓰기가 잦고
-- 정확도가 덜 중요하다).
-- (마이그레이션 분리기가 세미콜론으로 자르므로 주석에 세미콜론을 쓰지 않는다)

-- 좋아요·북마크. kind 를 enum 으로 둬서 세 번째 반응이 생겨도 스키마가 안 바뀐다
CREATE TABLE IF NOT EXISTS research_reactions (
  id         BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
  call_id    BIGINT UNSIGNED NOT NULL,
  user_id    BIGINT UNSIGNED NOT NULL,
  kind       VARCHAR(16)  NOT NULL DEFAULT 'like',
  created_at TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  UNIQUE KEY uniq_rrx_call_user_kind (call_id, user_id, kind),
  KEY idx_rrx_call_kind (call_id, kind),
  KEY idx_rrx_user_kind (user_id, kind, created_at DESC)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- 댓글. 대댓글은 1단계까지만 (parent_id 가 가리키는 행은 parent_id IS NULL 이어야 한다).
-- 삭제는 soft delete -- 본문만 지우고 행은 남겨서 대댓글이 고아가 되지 않게 한다
CREATE TABLE IF NOT EXISTS research_comments (
  id         BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
  call_id    BIGINT UNSIGNED NOT NULL,
  parent_id  BIGINT UNSIGNED NULL,
  user_id    BIGINT UNSIGNED NOT NULL,
  body       VARCHAR(2000) NOT NULL DEFAULT '',
  deleted_at TIMESTAMP(3)  NULL,
  created_at TIMESTAMP(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at TIMESTAMP(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  KEY idx_rc_call (call_id, created_at ASC),
  KEY idx_rc_parent (parent_id, created_at ASC),
  KEY idx_rc_user (user_id, created_at DESC)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- 3축 평가. 단일 점수가 아니라 축을 나눠서 "무엇이 좋았는지"를 말하게 한다.
-- 결과 확정 전에만 남길 수 있고(서비스에서 강제), 그 뒤로는 수정도 막는다 --
-- 결과를 보고 매기면 그건 논거 평가가 아니라 성과 재측정이다
CREATE TABLE IF NOT EXISTS research_ratings (
  id         BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
  call_id    BIGINT UNSIGNED NOT NULL,
  user_id    BIGINT UNSIGNED NOT NULL,
  clarity    TINYINT UNSIGNED NOT NULL DEFAULT 3,
  evidence   TINYINT UNSIGNED NOT NULL DEFAULT 3,
  risk       TINYINT UNSIGNED NOT NULL DEFAULT 3,
  created_at TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  UNIQUE KEY uniq_rrt_call_user (call_id, user_id),
  KEY idx_rrt_call (call_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- 조회 중복 제거. 로그인 사용자는 user_id, 비로그인은 IP+UA 해시를 viewer_key 로 쓴다.
-- 같은 키가 30분 안에 다시 오면 카운트하지 않는다
CREATE TABLE IF NOT EXISTS research_views (
  id         BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
  call_id    BIGINT UNSIGNED NOT NULL,
  viewer_key CHAR(64)     NOT NULL,
  viewed_at  TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  UNIQUE KEY uniq_rv_call_viewer (call_id, viewer_key),
  KEY idx_rv_viewed (viewed_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
