USE quantport;

-- 스탬프 v2 — 예상 주가(필수) + 자유 만기
--
-- horizon_days(일수)로는 「이번 장 종료」·「다음 장 종료」를 표현할 수 없다. 만기를
-- 생성 시점에 확정한 시각(due_at)으로 저장하고 종류(horizon_kind)는 라벨로만 남긴다.
-- 나중에 계산하면 휴장일 목록이 갱신될 때 과거 스탬프의 판정이 흔들린다.
--
-- 기존 행은 target_price 를 만들어 낼 방법이 없다 (당시 사용자가 예상한 가격은
-- 아무 데도 기록돼 있지 않고, 지금 값을 넣으면 사후 조작이 된다). 사용자 결정에
-- 따라 전량 삭제한다. 백업은 docs/archive/stamps-pre-046.sql 참고
-- (마이그레이션 분리기가 세미콜론으로 자르므로 주석에 세미콜론을 쓰지 않는다)

TRUNCATE TABLE stamps;

ALTER TABLE stamps
  ADD COLUMN target_price DOUBLE       NOT NULL DEFAULT 0 AFTER conviction,
  ADD COLUMN horizon_kind VARCHAR(20)  NOT NULL DEFAULT '1m' AFTER target_price,
  ADD COLUMN due_at       TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) AFTER horizon_kind,
  DROP COLUMN horizon_days;

-- 개봉 대기 목록은 due_at 순으로 읽는다
CREATE INDEX idx_stamps_user_due ON stamps (user_id, due_at, opened_at);
