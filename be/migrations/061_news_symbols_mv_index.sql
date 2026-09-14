USE quantport;

-- 뉴스 종목 필터에 multi-valued index (2026-08-24)
--
-- # 고치려는 것
--
-- `news_repo.go:410`의 종목 필터가 인덱스를 못 탄다:
--
--   WHERE market = ? AND JSON_CONTAINS(symbols_json, JSON_QUOTE(?))
--   ORDER BY published_at DESC LIMIT ?
--
-- MySQL은 `idx_news_market_published`를 published_at 역순으로 훑으며 필터링하고
-- LIMIT이 차면 멈춘다. **매칭이 없으면 멈출 수 없어 끝까지 읽는다.**
-- 실측(EXPLAIN ANALYZE): 흔한 종목 2.28ms/760행 vs 없는 종목 **298ms/67,558행**.
--
-- 종목 코드는 사용자가 URL로 임의 지정할 수 있으므로 이건 cache penetration
-- 벡터이기도 하다. 그리고 비용이 기사 수에 선형이라 500k행이면 약 1.8초가 된다.
--
-- # 왜 조인 테이블이 아니라 multi-valued index인가
--
-- `news_article_symbols` 정규화 테이블이 성능은 조금 더 좋다(정렬까지 커버).
-- 그런데 **쓰기 경로가 하나 늘고 동기화가 어긋날 여지**가 생긴다 —
-- `symbols_json`을 갱신하는 지점이 적재(Upsert)와 재태깅 배치(`news_store_retag.go`)
-- 둘이라, 그 둘을 한 트랜잭션으로 묶지 않으면 조용히 틀린 결과가 나간다.
--
-- multi-valued index는 **같은 컬럼의 인덱스**라 그 위험이 존재하지 않는다.
-- `symbols_json`을 쓰면 인덱스가 자동으로 따라오고, 되돌리기도 DROP INDEX 하나다.
--
-- # 선행: JSON null 정규화
--
-- `json.Marshal`이 nil 슬라이스를 `[]`가 아니라 `null`로 쓰는 탓에 93,469행 중
-- **28,470행의 symbols_json이 JSON null**이었다. `CAST(... AS ARRAY)`는 배열이나
-- SQL NULL만 받고 JSON null에서는 ERROR 3907로 실패하므로 먼저 통일해야 한다.
-- 쓰기 쪽은 `repository.marshalJSONStringArray`가 같은 날 고쳤다.
--
-- 갱신은 청크로 나눈다. 한 문장으로 28,470행을 고치면 그 트랜잭션이 락을 오래
-- 쥐고(P2-16의 행 락 대기 21,974건이 이미 있다), 이 배포 대상의 스토리지
-- 처리량은 29 MiB/s다.
--
-- # 쿼리도 함께 바꿔야 한다
--
-- `JSON_CONTAINS(...)`는 이 인덱스를 타지 않는다. `? MEMBER OF (symbols_json->'$')`
-- 로 바꿔야 옵티마이저가 쓴다 — 코드 쪽에서 같이 반영했다.
--
-- # 주의
--
-- LOCK=NONE은 쓸 수 없다 — MVI가 숨은 가상 컬럼을 만들기 때문이다
-- (Error 1846: Try LOCK=SHARED). SHARED는 읽기는 두고 **쓰기만** 잠깐 막는다.
--
-- MVI는 `ORDER BY published_at DESC`를 커버하지 못해 매칭 행을 모아 filesort 한다.
-- 매칭이 수십~수백 건이라 정렬 비용은 작지만, **흔한 종목이 회귀할 수 있으므로**
-- 적용 전후로 EXPLAIN ANALYZE를 비교할 것(흔한 종목 / 없는 종목 두 케이스).

UPDATE news_articles
   SET symbols_json = JSON_ARRAY()
 WHERE symbols_json IS NOT NULL
   AND JSON_TYPE(symbols_json) = 'NULL'
 LIMIT 10000;

UPDATE news_articles
   SET symbols_json = JSON_ARRAY()
 WHERE symbols_json IS NOT NULL
   AND JSON_TYPE(symbols_json) = 'NULL'
 LIMIT 10000;

UPDATE news_articles
   SET symbols_json = JSON_ARRAY()
 WHERE symbols_json IS NOT NULL
   AND JSON_TYPE(symbols_json) = 'NULL'
 LIMIT 10000;

ALTER TABLE news_articles
  ADD INDEX idx_news_symbols_mv ((CAST(symbols_json->'$' AS CHAR(8) ARRAY))),
  ALGORITHM=INPLACE, LOCK=SHARED;
