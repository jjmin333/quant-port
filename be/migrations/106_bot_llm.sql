-- 봇 초안 LLM 다시 쓰기 · 게시 계정 말투 · 기존 뱃지 고르기 (2026-10-19 사용자 요청)
--
-- users.persona_style — 게시 계정의 말투 설명(관리자가 적는다). LLM이 그 계정 이름으로 쓸 때 따른다.
-- users.badge — 자유 문구 → 서비스에 있는 뱃지 키 목록(쉼표, 예: writer,liked). 105 때 만든 계정은 모두 빈 값이었다.
-- bot_drafts.source — template | llm. facts — 다시 쓰기 전 템플릿 본문(=LLM 입력 사실). 다시 쓰면 body만 바뀐다.
-- bot_drafts.persona_id — 초안을 만들 때 배정한 게시 계정(그 말투로 썼다). 게시할 때 바꿀 수 있다.
-- bot_drafts.note — 다시 쓰기를 버린 이유(숫자 검증·금지어 등). 관리자 화면에 보인다.

USE quantport;

ALTER TABLE users ADD COLUMN persona_style VARCHAR(500) NOT NULL DEFAULT '' AFTER badge;
ALTER TABLE users MODIFY COLUMN badge VARCHAR(120) NOT NULL DEFAULT '';

ALTER TABLE bot_drafts ADD COLUMN source VARCHAR(12) NOT NULL DEFAULT 'template' AFTER length;
ALTER TABLE bot_drafts ADD COLUMN facts MEDIUMTEXT NULL AFTER body;
ALTER TABLE bot_drafts ADD COLUMN persona_id BIGINT UNSIGNED NULL AFTER facts;
ALTER TABLE bot_drafts ADD COLUMN note VARCHAR(200) NOT NULL DEFAULT '' AFTER persona_id;
