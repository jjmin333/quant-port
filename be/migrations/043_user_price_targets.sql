USE quantport;

-- 사용자 목표가 (브라우저 localStorage → 서버 DB 이전)

CREATE TABLE IF NOT EXISTS user_price_targets (
  id            BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
  user_id       BIGINT UNSIGNED NOT NULL,
  symbol        VARCHAR(20)     NOT NULL,
  target_price  DECIMAL(18, 4)  NOT NULL,
  currency      VARCHAR(3)      NOT NULL DEFAULT 'KRW',
  note          VARCHAR(500)    NOT NULL DEFAULT '',
  created_at    TIMESTAMP(3)    NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at    TIMESTAMP(3)    NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  UNIQUE KEY uq_user_price_targets_user_symbol (user_id, symbol),
  KEY idx_user_price_targets_user_updated (user_id, updated_at DESC)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
