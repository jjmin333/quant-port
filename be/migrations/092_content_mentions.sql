-- 글 안의 데이터 태그 (2026-09-28 사용자 요청 2 · 결정: docs/mockups/data-tags-screener-v1.html)
--
-- 커뮤니티 글·투자 리포트 본문의 `@종목` · `@종목:지표` · `#종목`을 여기 적는다. 쓴 순간의 값
-- (스크리너 지표 26개 + 가격)을 `snap`에 얼려 두고, 읽을 때의 값과 나란히 「그때 → 지금」으로 보인다.
-- 한 표로 두 글 종류를 받는 이유: 규칙(해석·스냅샷·지금 값)이 같고, 종목 화면 「커뮤니티」 탭이
-- 두 종류를 한 목록으로 읽는다(symbol 인덱스).
--
-- target_type: community_post | research_call

USE quantport;

CREATE TABLE IF NOT EXISTS content_mentions (
  target_type VARCHAR(16)     NOT NULL,
  target_id   BIGINT UNSIGNED NOT NULL,
  symbol      VARCHAR(32)     NOT NULL,
  -- 본문에서 이 종목을 가리킨 이름들(@ 뒤의 글자) — 화면이 토큰을 다시 찾을 때 쓴다.
  names       JSON            NOT NULL,
  -- {"fields":{"score":70,...},"price":286500,"changeRate":3.2,"currency":"KRW","score":70}
  snap        JSON            NOT NULL,
  snap_at     TIMESTAMP(3)    NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (target_type, target_id, symbol),
  KEY idx_cm_symbol (symbol, target_type, target_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
