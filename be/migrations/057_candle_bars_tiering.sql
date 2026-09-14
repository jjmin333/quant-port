USE quantport;

-- 캔들 계층 저장 — 파티션 + src_bars
--
-- # 왜 파티션인가
--
-- `candle_bars`의 PK는 (symbol, bar_interval, bar_time)이다. 그래서
-- `WHERE bar_interval = '1d'`는 **인덱스 선두가 아니라** 아무 것도 걸러주지 못한다.
-- 1d만 쓰는 배치 두 개가 실제로 표 전체를 훑고 90%를 버리고 있었다 (실측
-- EXPLAIN: type=index, key=PRIMARY, rows=13,867,286, filtered=10.00).
--
--   internal/service/stock_score.go   `WHERE bar_interval='1d' ORDER BY symbol, bar_time`
--   internal/service/daily_close.go   같은 필터 위의 ROW_NUMBER 프로젝션
--
-- 1d는 525MB인데 스캔은 2.4GB다. 그리고 **1분봉이 쌓일수록 1분봉을 전혀 안 쓰는
-- 배치가 느려진다** — 2026-08-22의 「프로젝션 47초」가 이 구조에서 나왔다.
--
-- LIST COLUMNS 파티션이면 `bar_interval` 등치 조건에 프루닝이 걸린다. 실측으로
-- 배치 쿼리는 partitions=p_1d 하나만 읽고, 차트 쿼리는 partitions=p_1m 하나만
-- 읽으면서 range scan과 Backward index scan을 그대로 유지한다.
--
-- 정렬도 살아 있다. 파티션 안에서는 bar_interval이 상수라 PK 순서가 그대로
-- (symbol, bar_time)이 되어, 배치의 ORDER BY가 filesort를 만들지 않는다.
--
-- 파티션 컬럼은 모든 unique key에 포함돼야 한다는 MySQL 제약은 이미 충족한다 --
-- PK가 bar_interval을 갖고 있다.
--
-- # '1M'이 목록에 없는 이유
--
-- collation이 utf8mb4_0900_ai_ci라 **'1M'(월봉)과 '1m'(분봉)이 같은 값**이다.
-- 둘을 따로 나열하면 ERROR 1495 (Multiple definition of same constant)가 난다.
-- 월봉은 저장되지 않으므로(1d에서 조회 시점에 접는다) 문제가 되지 않는다.
--
-- p_etc는 방어용이다. MySQL의 LIST 파티션에는 DEFAULT가 없어서, 목록에 없는
-- 값을 INSERT하면 조용히 다른 데 들어가는 게 아니라 **에러**가 난다.
--
-- # src_bars — 이 봉이 몇 개의 1분봉에서 나왔는가
--
-- 롤업은 부분 버킷도 접는다. 완전한 버킷만 접으면, 접히지 못한 조각이 1m 계층에
-- 영구 누적되어 계층 크기가 상수가 되지 않기 때문이다.
--
-- 부분 버킷을 접는 것 자체는 새로운 부정확성이 아니다 — 조회 시점 롤업
-- (aggregateCandlesMinutes)이 지금도 똑같이 계산해서 화면에 그리고 있다. 다만
-- 「나중에 Toss 백필로 개선될 여지」가 사라지는데, src_bars가 그 여지를 남긴다.
--
-- 그리고 롤업의 안전장치로 쓰인다. Toss 백필이 이미 접힌 구간에 1분봉을
-- 되넣으면(getCandlesPageUncached는 받아온 것을 1m으로 저장한다) 다음 롤업이 그
-- 조각만으로 재집계하는데, src_bars가 줄어드는 덮어쓰기를 거부하면 더 완전한
-- 봉이 살아남는다.
--
-- 원본 1분봉과 1d는 1이다. 롤업 봉은 그 아래 1분봉의 개수 — 5m은 최대 5,
-- 60m은 최대 60이라 TINYINT UNSIGNED로 충분하다.
--
-- 맨 뒤에 붙이는 이유는 ALGORITHM=INSTANT를 쓰기 위해서다. AFTER volume으로
-- 위치를 지정하면 표를 통째로 재구축한다. 코드베이스에 candle_bars를
-- `SELECT *`로 읽는 곳이 없어 컬럼 순서는 무관하다 (전부 컬럼명을 적는다).
--
-- 실측: ADD COLUMN 0.19초 / PARTITION BY 11.9초 (2M행 프록시). 17.7M행 환산 2~3분.

ALTER TABLE candle_bars
  ADD COLUMN src_bars TINYINT UNSIGNED NOT NULL DEFAULT 1,
  ALGORITHM=INSTANT;

-- ※ 2026-08-24: 60분봉 기능 제거로 p_60m을 목록에서 뺐다 (migrations/060_drop_60m_candles.sql).
ALTER TABLE candle_bars
  PARTITION BY LIST COLUMNS(bar_interval) (
    PARTITION p_1m  VALUES IN ('1m'),
    PARTITION p_5m  VALUES IN ('5m'),
    PARTITION p_10m VALUES IN ('10m'),
    PARTITION p_30m VALUES IN ('30m'),
    PARTITION p_1d  VALUES IN ('1d'),
    PARTITION p_etc VALUES IN ('1w')
  );
