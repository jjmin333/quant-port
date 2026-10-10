-- DB 정리 (2026-10-10 사용자 승인)
--
-- ① 접두사 중복 인덱스 넷 — 같은 표의 UNIQUE 키가 같은 열로 시작해 그것만으로 덮인다(쓰기마다 인덱스 하나씩 덜 갱신).
-- ② 문장 검색 표 둘 — 기능을 2026-10-09에 내렸다(규칙 파서라 품질이 안 나온다). 행은 0 · 3개였다.
-- 다시 돌아도 같은 결과다: 없는 인덱스(1091)는 마이그레이터가 건너뛰고, 표는 IF EXISTS.

USE quantport;

ALTER TABLE research_comment_likes DROP INDEX idx_rcl_comment;            -- uniq_rcl_comment_user(comment_id, user_id)
ALTER TABLE research_ratings DROP INDEX idx_rrt_call;                     -- uniq_rrt_call_user(call_id, user_id)
ALTER TABLE stock_warnings DROP INDEX idx_stock_warning_symbol;           -- uk_stock_warning(symbol, warning_type)
ALTER TABLE user_holdings DROP INDEX idx_user_holdings_user;              -- uq_user_holdings_user_symbol(user_id, symbol)

DROP TABLE IF EXISTS screener_phrase_log;
DROP TABLE IF EXISTS screener_phrase_dict;
