-- Cached Korean reader translations for US news (Papago Website Translation).
CREATE TABLE IF NOT EXISTS news_reader_translations (
  url_hash CHAR(64) NOT NULL PRIMARY KEY,
  article_id VARCHAR(32) NULL,
  market VARCHAR(16) NOT NULL DEFAULT 'us',
  title_ko VARCHAR(512) NOT NULL DEFAULT '',
  excerpt_ko TEXT NULL,
  content_html_ko MEDIUMTEXT NULL,
  text_content_ko MEDIUMTEXT NULL,
  source_content_hash CHAR(64) NULL,
  created_at TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  KEY idx_news_reader_trans_article (article_id)
);
