-- 종목 마스터에 「토스가 거래를 지원하는가」를 적는다
--
-- # 왜 필요했나
--
-- 우리 카탈로그(status=NORMAL)는 토스가 취급하는 집합의 **상위집합**이다 워런트·
-- 우선주·라이츠·상장폐지 직전 종목이 우리 쪽에만 남아 있다 그 차이를 예전에는
-- 종목마다 404를 한 번씩 맞아 가며 배웠다 실측으로 경고 스윕 호출의 19%(3,388건)와
-- 해외 종가 구멍 메우기의 회차당 3,250건이 그 404였다
--
-- GET /api/v1/stocks/all 이 그 집합을 마켓 단위로 한 번에 준다(하루 7호출) 받은
-- 결과를 여기 적어 두면 수집도 화면도 그 사실을 근거로 판단할 수 있다
--
-- # 왜 NULL 을 기본값으로 두는가
--
-- 세 상태를 구분해야 한다
--
--   NULL  아직 확인 안 했다 (유니버스 배치가 한 번도 안 돌았거나 새로 들어온 종목)
--   1     토스가 거래를 지원한다
--   0     지원하지 않는다
--
-- 기본값을 0으로 두면 **배치가 처음 돌기 전까지 모든 종목이 숨는다** 화면을 통째로
-- 비우는 실패이므로, 조회는 NULL 을 「숨기지 않는다」로 다룬다 모르면 보여 준다
--
-- # 인덱스
--
-- 조회가 status 와 함께 거는 조건이라 (status, toss_tradable) 복합으로 둔다
-- 단독 인덱스로는 status='NORMAL' 이 이미 걸러낸 행을 다시 훑는다

ALTER TABLE stocks
  ADD COLUMN toss_tradable   TINYINT(1) NULL DEFAULT NULL,
  ADD COLUMN toss_checked_at TIMESTAMP  NULL DEFAULT NULL;

ALTER TABLE stocks
  ADD INDEX idx_status_toss (status, toss_tradable);
