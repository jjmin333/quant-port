ALTER TABLE llm_settings
  ADD COLUMN refresh_interval_hours INT NOT NULL DEFAULT 6 AFTER pilot_symbols;
