-- News preview: original URL, publisher, og:image, preview mode
ALTER TABLE news_articles
  ADD COLUMN original_url VARCHAR(2048) NULL AFTER url,
  ADD COLUMN portal_url VARCHAR(2048) NULL AFTER original_url,
  ADD COLUMN publisher_id VARCHAR(32) NULL AFTER source_id,
  ADD COLUMN publisher_name VARCHAR(64) NULL AFTER publisher_id,
  ADD COLUMN image_url TEXT NULL AFTER summary,
  ADD COLUMN preview_mode VARCHAR(16) NOT NULL DEFAULT 'summary' AFTER image_url;

CREATE INDEX idx_news_original_url ON news_articles (original_url(255));
