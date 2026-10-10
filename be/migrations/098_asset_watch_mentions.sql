-- 관심 부동산 · 글 안의 자산 태그 (2026-10-03 사용자 결정 — docs/mockups/asset-ai-tags-v1.html R3 · ③)
--
--   re_watchlist            사용자의 관심 단지·지역. kind = complex(단지 키) · region(시군구 lawd_cd).
--                           주식 관심(watchlist_items)·금융상품 관심(fin_watchlist)과 섞지 않는다.
--   content_asset_mentions  커뮤니티 글·투자 리포트 본문의 자산 태그(@아파트[..] @지역[..] @상품[..] @금리[..]).
--                           종목 태그(content_mentions)와 표를 나눠 종목 경로를 건드리지 않는다.
--                           snap = 쓴 순간 값(처음 태그한 순간, 글을 고쳐도 그대로) · names = 본문이 부른 이름들.
-- (마이그레이션 분리기가 세미콜론으로 자르므로 주석에 세미콜론을 쓰지 않는다)

USE quantport;

CREATE TABLE IF NOT EXISTS re_watchlist (
  user_id    BIGINT       NOT NULL,
  kind       VARCHAR(10)  NOT NULL,
  skey       VARCHAR(120) NOT NULL,
  lawd_cd    CHAR(5)      NOT NULL,
  name       VARCHAR(120) NOT NULL DEFAULT '',
  created_at TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (user_id, kind, skey)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS content_asset_mentions (
  target_type VARCHAR(20)  NOT NULL,
  target_id   BIGINT       NOT NULL,
  kind        VARCHAR(10)  NOT NULL,
  skey        VARCHAR(120) NOT NULL,
  name        VARCHAR(120) NOT NULL DEFAULT '',
  names       JSON         NOT NULL,
  snap        JSON         NOT NULL,
  snap_at     TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (target_type, target_id, kind, skey)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
