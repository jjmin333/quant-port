-- User-defined watchlist display order (lower = higher in list).

ALTER TABLE watchlist_items
  ADD COLUMN sort_order INT NOT NULL DEFAULT 0 AFTER symbol;

UPDATE watchlist_items w
INNER JOIN (
  SELECT id, ROW_NUMBER() OVER (PARTITION BY user_id ORDER BY created_at DESC) - 1 AS rn
  FROM watchlist_items
) ranked ON w.id = ranked.id
SET w.sort_order = ranked.rn;

CREATE INDEX idx_watchlist_user_sort ON watchlist_items (user_id, sort_order);
