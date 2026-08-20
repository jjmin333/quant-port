-- Scheduled AI 요약 batch over 마켓 레이더's 1일 기준 top 100.
--
-- pilot_symbols changes meaning here. It was the only set of symbols allowed
-- to call the LLM, and becomes the always-include list layered on top of the
-- ranking-derived targets. VARCHAR(512) was sized for that old cap of 10.
-- The radar list is built at run time and never stored in this column, so
-- TEXT is only headroom for the pinned list.
--
-- NOTE: keep statement separators out of these comments. splitSQLStatements
-- cuts the file on the separator character without parsing, so one inside a
-- comment silently splits a statement in half.
ALTER TABLE llm_settings
  MODIFY COLUMN pilot_symbols TEXT NOT NULL;

ALTER TABLE llm_settings
  ADD COLUMN target_mode VARCHAR(16) NOT NULL DEFAULT 'radar' AFTER pilot_symbols,
  ADD COLUMN daily_cap INT NOT NULL DEFAULT 250 AFTER target_mode,
  ADD COLUMN quota_reserve INT NOT NULL DEFAULT 80 AFTER daily_cap,
  ADD COLUMN token_reserve INT NOT NULL DEFAULT 120000 AFTER quota_reserve;
