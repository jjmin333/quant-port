-- 뉴스 주제 차선 검색어를 관리자 키워드로 (2026-10-09 사용자 요청)
--
-- news_poll_keywords.topic: '' = 지금까지의 관리자 키워드(종목 뉴스 큐에 섞여 돈다) · real_estate · rates · deposit · loan =
-- 주제 차선(news_topics.go)이 도는 검색어. 국내만. 씨앗 18개는 그동안 코드에 박혀 있던 목록 그대로다.

USE quantport;

ALTER TABLE news_poll_keywords ADD COLUMN topic VARCHAR(16) NOT NULL DEFAULT '' AFTER keyword;

INSERT IGNORE INTO news_poll_keywords (market, keyword, topic, enabled, sort_order) VALUES
  ('domestic', '아파트 매매', 'real_estate', 1, 100), ('domestic', '아파트 전세', 'real_estate', 1, 101),
  ('domestic', '부동산 시장', 'real_estate', 1, 102), ('domestic', '청약', 'real_estate', 1, 103),
  ('domestic', '재건축', 'real_estate', 1, 104), ('domestic', '주택 공급', 'real_estate', 1, 105),
  ('domestic', '기준금리', 'rates', 1, 110), ('domestic', '국고채 금리', 'rates', 1, 111),
  ('domestic', '한국은행 금리', 'rates', 1, 112), ('domestic', '미국 금리', 'rates', 1, 113),
  ('domestic', '예금 금리', 'deposit', 1, 120), ('domestic', '적금 특판', 'deposit', 1, 121),
  ('domestic', '파킹통장', 'deposit', 1, 122), ('domestic', '정기예금', 'deposit', 1, 123),
  ('domestic', '주담대 금리', 'loan', 1, 130), ('domestic', '전세대출', 'loan', 1, 131),
  ('domestic', '신용대출 금리', 'loan', 1, 132), ('domestic', 'DSR', 'loan', 1, 133);

-- 「기준금리」는 이미 일반 관리자 키워드로 있었다(INSERT IGNORE가 건너뜀) — 금리 주제로 옮긴다.
UPDATE news_poll_keywords SET topic = 'rates' WHERE market = 'domestic' AND keyword = '기준금리' AND topic = '';

-- ── 아파트 단지 찾기 (2026-10-09 사용자 제보: 「대구 수성구 동대구로 345 범어역우방유쉘」이 안 찾아진다) ──
--
-- 원인: ① 단지 검색이 실거래 표를 complex_name LIKE로만 훑어 띄어쓰기(「범어역 우방유쉘」)가 다르면 0건,
-- ② 도로명 주소를 저장하지 않았다, ③ 시군구를 먼저 골라야만 검색됐다, ④ 글 태그·미리보기 색인은 점수가 매겨진
-- 단지(1년 5건 이상 · 최근 3개월 2건 이상, 전체의 약 3분의 1)만 안다.
-- 그래서 도로명을 받고(re_trades.road — 아파트 매매 API만 준다), 단지 한 줄씩의 목록 re_complexes를 둔다:
-- 이름에서 띄어쓰기를 뺀 name_norm, 도로명, 법정동+지번. 4만여 행이라 전국 LIKE 검색이 가볍다.
-- 자산 점수 잡(worker-asset)이 매일 다시 채운다. 여기서는 첫 채우기만.

ALTER TABLE re_trades ADD COLUMN road VARCHAR(80) NOT NULL DEFAULT '';

CREATE TABLE IF NOT EXISTS re_complexes (
  complex_key  VARCHAR(120) NOT NULL PRIMARY KEY,
  lawd_cd      CHAR(5)      NOT NULL,
  name         VARCHAR(100) NOT NULL,
  name_norm    VARCHAR(100) NOT NULL,
  umd_nm       VARCHAR(40)  NOT NULL DEFAULT '',
  jibun        VARCHAR(40)  NOT NULL DEFAULT '',
  road         VARCHAR(80)  NOT NULL DEFAULT '',
  build_year   SMALLINT     NULL,
  trades       INT          NOT NULL DEFAULT 0,
  last_deal    DATE         NULL,
  KEY idx_re_complexes_lawd (lawd_cd, trades)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

INSERT INTO re_complexes (complex_key, lawd_cd, name, name_norm, umd_nm, jibun, road, build_year, trades, last_deal)
SELECT complex_key, MAX(lawd_cd), MAX(complex_name), REPLACE(MAX(complex_name), ' ', ''), MAX(umd_nm), MAX(jibun), MAX(road),
       MAX(build_year), COUNT(*), MAX(deal_date)
  FROM re_trades WHERE kind = 'apt' AND complex_key <> '' AND complex_name <> ''
 GROUP BY complex_key
ON DUPLICATE KEY UPDATE trades = VALUES(trades), last_deal = VALUES(last_deal);
