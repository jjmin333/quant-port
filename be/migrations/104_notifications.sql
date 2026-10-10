-- 앱 안 알림 + 자산 관련 글 (2026-10-10 사용자 요청 4)
--
-- user_notifications — 관심 목록(종목·단지·금융상품)의 변화를 헤더 종 아이콘의 알림 칸에 쌓는다. 웹 푸시는 쓰지 않는다
--   (사용자 결정). 같은 사건이 두 번 쌓이지 않게 (user_id, dedupe_key)가 유일하다 — 생성기는 INSERT IGNORE로 몇 번
--   돌아도 같은 결과다. 90일 지난 것은 생성기가 돌 때 함께 지운다.
-- notify_cursors — 생성기가 「어디까지 봤나」. re_trades·fin_rate_history에는 적재 시각이 없어서 id·공시월로 센다.
--   처음 보는 커서는 그 자리를 적기만 하고 알리지 않는다(켜는 순간 지난 거래가 한꺼번에 쏟아지지 않게).

USE quantport;

CREATE TABLE IF NOT EXISTS user_notifications (
  id         BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
  user_id    BIGINT UNSIGNED NOT NULL,
  -- stock | re_trade | fin_rate
  kind       VARCHAR(16)  NOT NULL,
  dedupe_key VARCHAR(160) NOT NULL,
  title      VARCHAR(200) NOT NULL,
  body       VARCHAR(300) NOT NULL DEFAULT '',
  href       VARCHAR(300) NOT NULL DEFAULT '',
  created_at TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  read_at    TIMESTAMP(3) NULL,
  UNIQUE KEY uq_un_user_key (user_id, dedupe_key),
  KEY idx_un_user_id (user_id, id),
  KEY idx_un_user_unread (user_id, read_at),
  KEY idx_un_created (created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS notify_cursors (
  name       VARCHAR(80)  NOT NULL PRIMARY KEY,
  value      VARCHAR(40)  NOT NULL,
  updated_at TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- 단지·지역·상품 화면의 「이 자산 이야기」 — 자산으로 글을 찾는다(PK는 글 → 자산 방향뿐이다).
ALTER TABLE content_asset_mentions ADD KEY idx_cam_asset (kind, skey, target_type, target_id);
