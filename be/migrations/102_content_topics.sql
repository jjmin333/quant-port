-- 글의 주제 — 주식 · 부동산 · 금융상품 (2026-10-09 사용자 요청 9)
--
-- 커뮤니티 글 · 투자 리포트 · 메모장 모두 주식에만 맞춰져 있었다. 쓸 때 주제를 고르고, 읽을 때 주제로 거른다.
--   stock        주식(기존 글 전부)
--   real_estate  부동산 — 단지·지역 태그(@아파트·@지역)
--   finance      금융상품 — 예·적금·대출 상품, 금리(@상품·@금리)
--
-- 리포트에서 stock이 아닌 글은 「분석 리포트」다: 종목·목표가·판정이 없고(status = 'note') 성적 집계에 들어가지 않는다.
-- 그래서 symbol은 빈 문자열, 근거 JSON은 빈 객체로 들어간다.

USE quantport;

ALTER TABLE community_posts ADD COLUMN topic VARCHAR(16) NOT NULL DEFAULT 'stock' AFTER body;
-- 목록은 ORDER BY id DESC — 보조 키 끝에 PK(id)가 붙으므로 (topic, status)로 정렬까지 덮는다.
ALTER TABLE community_posts ADD KEY idx_cp_topic_status (topic, status);

ALTER TABLE research_calls ADD COLUMN topic VARCHAR(16) NOT NULL DEFAULT 'stock' AFTER user_id;
ALTER TABLE research_calls ADD KEY idx_rc_topic_published (topic, published_at);

ALTER TABLE journal_notes ADD COLUMN topic VARCHAR(16) NOT NULL DEFAULT 'stock' AFTER body;
