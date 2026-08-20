USE quantport;

-- 메모장 (구 AI 투자일지) — 사용자별 메모, 종목 연동/고정/검색 지원.

CREATE TABLE IF NOT EXISTS journal_notes (
  id          BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
  user_id     BIGINT UNSIGNED NOT NULL,
  title       VARCHAR(200) NOT NULL DEFAULT '',
  body        TEXT         NOT NULL,
  symbol      VARCHAR(20)  NULL,
  symbol_name VARCHAR(100) NULL,
  pinned      TINYINT(1)   NOT NULL DEFAULT 0,
  created_at  TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at  TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  KEY idx_journal_notes_user_list (user_id, pinned DESC, updated_at DESC),
  KEY idx_journal_notes_user_symbol (user_id, symbol)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
