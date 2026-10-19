-- 봇 초안 작성자 유형 (2026-10-19 사용자 요청: 특정 계정 말투가 아니라 실제 커뮤니티처럼 각양각색 — 한 줄 글부터 컬럼까지)
-- bot_drafts.archetype — 초안을 만들 때 뽑은 작성자 유형 키(service/bot_archetypes.go). 길이·말투·형식을 정한다.

USE quantport;

ALTER TABLE bot_drafts ADD COLUMN archetype VARCHAR(20) NOT NULL DEFAULT '' AFTER length;
