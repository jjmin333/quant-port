-- 변화 이벤트 magnitude 폭 (2026-10-20)
-- 내부자 이벤트의 magnitude는 순매매 주식 수다. 10% 주주의 대량 취득·처분은 1억 주를 넘고
-- (SAFX 1.19억 주, 10-16), DECIMAL(12,4) 상한(약 1억)에 걸려 그날 change_detect가 통째로
-- 실패했다(10-18·10-19 Error 1264). 정수부 16자리로 넓힌다 — 행이 적어(하루 수백) 즉시 끝난다.

USE quantport;

ALTER TABLE stock_change_events MODIFY magnitude DECIMAL(20,4) NULL;
