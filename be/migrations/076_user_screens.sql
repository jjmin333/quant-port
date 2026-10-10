-- 사용자가 저장한 스크리너 조건 (V2 요구 4 「저장 가능」)
--
-- # 왜 JSON 한 칸인가
--
-- 조건은 「필드 · 최소 · 최대」 세 값의 목록이고, 개수는 사용자가 정한다. 이것을
-- 정규화하면 `user_screen_rules` 표가 하나 더 생기고 조회마다 조인이 붙는데, 얻는
-- 것이 없다 — **조건으로 검색할 일이 없기 때문이다.** 「ROE 조건을 쓴 사용자를
-- 찾아라」는 질문은 이 제품에 없다. 목록은 언제나 「내 조건 전부」이고 그때 JSON은
-- 통째로 화면에 간다.
--
-- 유효성은 DB가 아니라 `model.LookupScreenField`가 지킨다. 저장할 때 화이트리스트를
-- 통과한 조건만 들어오고, 읽을 때도 다시 통과시킨다 — 필드가 나중에 없어져도
-- (지표가 사라지는 일은 실제로 있다) 저장된 조건이 화면을 깨뜨리지 않는다.
--
-- # 왜 상한이 있나
--
-- 사용자당 20개. 조건 저장은 무료로 무한히 쓸 수 있는 쓰기 경로라서, 상한이 없으면
-- 한 계정이 표를 채울 수 있다. 20개는 실제로 관리 가능한 개수의 상한이기도 하다 —
-- 그보다 많으면 목록에서 못 찾는다.
--
-- # 크기
--
-- 행당 약 300B(조건 12개 기준 JSON 250B + 이름). 사용자 1만 명이 평균 3개를 저장해도
-- 9MB다. 정리 배치를 두지 않는다 — 사용자가 지우기 전에는 남는다.

USE quantport;

CREATE TABLE IF NOT EXISTS user_screens (
  id         BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  user_id    BIGINT UNSIGNED NOT NULL,
  name       VARCHAR(60)     NOT NULL,
  market     VARCHAR(16)     NOT NULL,  -- domestic | overseas
  -- [{"field":"roe","min":15},{"field":"debtRatio","max":100}]
  rules_json JSON            NOT NULL,
  sort_by    VARCHAR(32)     NOT NULL DEFAULT '',
  sort_asc   TINYINT(1)      NOT NULL DEFAULT 0,
  created_at TIMESTAMP       NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP       NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  -- 목록은 언제나 「내 것을 최근 수정 순으로」다.
  KEY idx_user_screens_user (user_id, updated_at DESC),
  -- 같은 이름을 두 번 저장하면 목록에서 구분할 수 없다.
  UNIQUE KEY uq_user_screens_name (user_id, name),
  CONSTRAINT fk_user_screens_user FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
