USE quantport;

-- 메모장 v2 — 작성 시점 가격 박제 + 태그
--
-- 메모는 예측이 아니라 기록이므로 스탬프처럼 적중/빗나감을 매기지 않는다. 다만
-- 「그때 얼마였나」가 없으면 두 달 뒤에 다시 읽을 때 내가 옳았는지 알 방법이 없어서,
-- 저장하는 순간의 종목 가격을 같이 박제한다. 나중에 계산할 수 없는 값이다 --
-- 과거 시점 시세를 사후에 맞춰 넣으면 그건 박제가 아니라 재구성이다.
--
-- price_date 는 그 가격이 속한 거래일이다. 장 마감 후 21시에 쓴 메모의 가격은
-- 그날 종가지 다음 날 시세가 아니므로 날짜를 따로 들고 있어야 한다.
--
-- tags_json 은 스탬프/리포트와 같은 저장 형태를 쓴다. 종목 미지정 메모(환율/시장
-- 관찰)는 지금 분류할 축이 아예 없어서 시간순으로만 쌓인다.

ALTER TABLE journal_notes
  ADD COLUMN price_at_write DECIMAL(18, 4) NULL AFTER symbol_name,
  ADD COLUMN price_currency VARCHAR(3)     NULL AFTER price_at_write,
  ADD COLUMN price_date     DATE           NULL AFTER price_currency,
  ADD COLUMN tags_json      JSON           NULL AFTER price_date;
