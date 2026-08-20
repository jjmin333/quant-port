-- 지수 점수 · AI 요약 (docs/STOCK_SCORE_DESIGN.md 의 지수 확장)
--
-- Mirrors the columns stock_daily_closes already carries, so the index list and
-- the stock list render from the same shape. Indices have no financial
-- statements, so the score is technical-only by construction.
--
-- ai_summary carries an explicit DEFAULT ('') because the close upsert does not
-- know about this column: without a default, every index close write would fail
-- with "Field 'ai_summary' doesn't have a default value".
--
-- source widens to 32 because a row can now cite where its history came from
-- ("stock_daily_closes" is longer than the original 16).

ALTER TABLE index_daily_closes
  ADD COLUMN score      TINYINT NULL AFTER source,
  ADD COLUMN grade      VARCHAR(2) NULL AFTER score,
  ADD COLUMN ai_summary TEXT NOT NULL DEFAULT ('') AFTER grade,
  MODIFY COLUMN source  VARCHAR(32) NOT NULL DEFAULT '';
