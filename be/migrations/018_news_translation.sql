-- Korean translations for US (and other non-Korean) news headlines/summaries.
ALTER TABLE news_articles
  ADD COLUMN headline_ko VARCHAR(512) NULL AFTER headline,
  ADD COLUMN summary_ko TEXT NULL AFTER summary;
