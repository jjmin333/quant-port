USE quantport;

-- 댓글 추천 (2026-09-14 사용자 요청 3)
--
-- # 왜 필요한가
--
-- 리포트에 달리는 답은 「좋아요」가 아니라 반론이다 그래서 한 글 아래 스무 개가
-- 쌓이면 **읽을 만한 반론이 어느 것인지**를 읽는 사람이 알 수 없다 글 자체에는
-- 추천이 있는데(research_reactions) 댓글에는 없어서, 좋은 반박은 스무 번째 줄에
-- 그대로 묻혔다
--
-- # 왜 research_reactions 를 재사용하지 않나
--
-- 그 표의 고유 키는 (call_id, user_id, kind) 다 댓글에 붙이려면 call_id 자리에
-- comment_id 를 넣어야 하는데, 그러면 **같은 컬럼이 두 가지를 가리키게** 된다
-- 집계 질의마다 「이 call_id 는 진짜 콜인가」를 kind 로 갈라야 하고, 외래 관계를
-- 읽는 사람은 그것을 알 길이 없다 표를 나누는 값이 컬럼 두 개보다 싸다
--
-- # 카운터를 비정규화하지 않는다
--
-- research_comments 에 like_count 를 두지 않는다 038 이 같은 판단을 한 이유가
-- 그대로 적용된다 — 추천은 취소가 잦아 카운터가 어긋나기 쉽고, 한 글의 댓글은
-- 많아야 수백 줄이라 집계가 싸다
--
-- # 인덱스
--
-- 읽기는 언제나 「이 댓글들의 추천 수」와 「내가 누른 것」 둘이다 앞엣것은
-- comment_id 로 묶고 뒤엣것은 (comment_id, user_id) 정확 일치이므로, 고유 키
-- 하나가 둘을 다 덮는다 user_id 단독 인덱스는 두지 않는다 — 「내가 추천한 댓글
-- 목록」을 보여 주는 화면이 없다

CREATE TABLE IF NOT EXISTS research_comment_likes (
  id         BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
  comment_id BIGINT UNSIGNED NOT NULL,
  user_id    BIGINT UNSIGNED NOT NULL,
  created_at TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  UNIQUE KEY uniq_rcl_comment_user (comment_id, user_id),
  KEY idx_rcl_comment (comment_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
