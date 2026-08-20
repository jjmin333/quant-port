USE quantport;

ALTER TABLE watchlist_items
  ADD COLUMN added_price DECIMAL(20, 4) NULL AFTER symbol;
