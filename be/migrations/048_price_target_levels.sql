USE quantport;

-- 목표가 v2 — 등록 시점 박제 + 손절가 + 2차 목표
--
-- v1 은 symbol/target_price/note 뿐이라 「내가 얼마에 이 목표를 세웠나」를 알 수
-- 없었다. 등록가가 없으면 진행률(등록 -> 현재 -> 목표)을 그릴 수 없고, 화면은
-- 「목표까지 +11.4%」라는 숫자 하나로 남는다 -- 그게 가까운 건지 먼 건지 모른 채로.
--
-- 실제 매매는 「여기서 접고, 여기서 절반 팔고, 여기까지 간다」로 움직인다. 목표가
-- 칸이 하나뿐이라 그 나머지가 전부 note 필드에 한국어 문장으로 들어가 있었다.
--
-- 도달 시각(reached_at)은 컬럼으로 두지 않는다. 스탬프의 조기 적중 판정과 같은
-- 방식으로 candle_bars 일봉 고/저를 base_date 이후로 훑어 읽을 때 계산한다.
-- 저장해두면 목표가를 수정한 순간 옛 목표 기준의 도달일이 그대로 남아 어긋난다.

ALTER TABLE user_price_targets
  ADD COLUMN base_price     DECIMAL(18, 4) NULL AFTER target_price,
  ADD COLUMN base_date      DATE           NULL AFTER base_price,
  ADD COLUMN target_price_2 DECIMAL(18, 4) NULL AFTER base_date,
  ADD COLUMN stop_price     DECIMAL(18, 4) NULL AFTER target_price_2;
