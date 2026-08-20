USE quantport;

-- 리포트 썸네일
--
-- 목록이 가로형 카드로 바뀌면서 카드마다 이미지 자리가 생긴다. 작성자가 올리지
-- 않으면 화면이 종목 로고로 대체하므로 이 값은 NULL 이 정상이고, 빈 문자열과
-- 구분할 필요가 없어 NOT NULL DEFAULT '' 로 둔다.
--
-- 저장하는 건 URL 뿐이다. 바이트는 이미 있는 media.Store(/api/v1/media/images)에
-- 들어가고 키가 내용 해시라 같은 이미지를 두 번 올려도 한 번만 쌓인다.

ALTER TABLE research_calls
  ADD COLUMN thumbnail_url VARCHAR(512) NOT NULL DEFAULT '' AFTER body_rev;
