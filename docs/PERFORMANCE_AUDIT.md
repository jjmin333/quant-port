# Performance · Scalability · Reliability Audit

> 2026-08-23 · 코드 변경 없음 (분석·문서화 전용)
>
> 모든 수치는 **실행 중인 스택에서 실측**했다 — MySQL `EXPLAIN ANALYZE`,
> `SHOW GLOBAL STATUS` 델타, API 액세스 로그 33,078건, Redis `INFO`/`SCAN`,
> `docker stats`. 추측만으로 세운 항목은 넣지 않았다.

## 진행 상황 (2026-08-24 갱신)

| 항목 | 상태 | 실측 결과 |
|---|---|---|
| P1-6 인기순 뉴스 filesort | **완료** | 쿼리 238ms → **0.289ms**, 읽는 행 67,957 → 100 |
| P2-11 `stock_scores` 중복 인덱스 | **완료** | 인덱스 10.5MB → **4.5MB**, 읽기 계획 4종 회귀 없음 |
| P2-10 / P0-1(1·2단계) 무변경 쓰기 | **완료** | 유휴 INSERT **분당 1,080건 → 1건**, 실제 쓰기는 유지 |
| P0-2 요약 API single-flight | **완료** | 20동시 요청의 DB 질의 **291 → 69 (−76%)** |
| P1-7 리서치 피드 N+1 | **완료** | 종가 조회 O(심볼) → **O(1) 왕복**, 응답 동일성 확인 |
| P1-8 BulkRegister 원자성 | **완료** | 부분 저장 제거, 8동시×40종목에서 **락 대기 0건** |
| P0-1(3·4단계) `tickPrice` 캡 | 미착수 | 설계 합의 필요 |
| P0-3 예열 쿼터 분리 | 미착수 | 예열 정책 결정 선행 |
| P1-4 뉴스 캐시 LRU | 미착수 | |
| P1-5 뉴스 필터 풀스캔 | 미착수 | 스키마 변경 동반 |
| P1-9 커넥션 풀 | 미착수 | 운영 값 결정 필요 |
| 나머지 P2 | 미착수 | |

수정 이력과 상세 근거는 `be/migrations/059_news_popular_index.sql` 머리말,
`market_data_store.go`의 `RefreshPrices`·`lastPriceRewriteInterval` 주석,
`stock_summary.go`의 `buildOnce` 주석, `holding_trades_repo.go`의
`BulkRegisterAll` 주석에 각각 코드 옆에 남겼다.

### 이번 작업에서 새로 드러난 것

- **`/stocks/search` 178.7ms (avg, n=6)** — 기존 상위 지연들이 사라지면서 새 1위가 됐다.
  DB 부분은 최악 14.7ms(매칭 0건일 때 `stocks` 17,912행 풀스캔)뿐이라
  **나머지 약 163ms의 출처는 아직 모른다.** 추측하지 않고 별도 프로파일링이 필요하다.
- **`BulkRegister`의 `syncHolding`이 여전히 심볼당 1 SELECT + 1 upsert**다. 이번에는
  중복 판정 조회(2N)만 배치화했다. 80종목 요청에서 항목당 약 1.65 질의가 남는다.
- **동시 일괄 등록의 중복 경합**은 그대로다(단건 `Create`에도 있는 오래된 성질).
  여기서만 행 락을 걸면 일관성이 없어 손대지 않았다.

## 측정 환경

| 항목 | 값 |
|---|---|
| 측정 시각 | 2026-08-23 01:24 ~ 02:35 KST |
| MySQL | 8.0.46 · `max_connections=151` · buffer pool 4GB · Uptime 32.8h |
| 관측 요청 | 33,078건 / 53분 (대부분 e2e 테스트 트래픽, 유기적 폴링 포함) |
| 유휴 부하 | **DB 66 q/s** (SELECT 18 · INSERT 23) · Redis 37 ops/s · API CPU 0.6% |
| 데이터 규모 | `candle_bars` 6.25M행 · `stock_daily_closes` 3.80M행 · `news_articles` **90,913행**(domestic 67,571 / us 23,342) · `stocks` 17,912 |

> `information_schema.TABLES.TABLE_ROWS`는 InnoDB 추정치라 `news_articles`를 81k로
> 보고한다. 위 값은 `COUNT(*)` 실측이고, 아래 `EXPLAIN ANALYZE`의 스캔 행수(67,558)가
> domestic 실제 행수(67,571)와 일치하는 것으로 교차 확인된다.
| 사용자 | 3,775 (대부분 QA 계정) · 실사용 관심종목 distinct 7 · 보유 distinct 16 |

**해석 시 주의**: 현재 실사용자가 사실상 0이다. 아래 P0 항목들은 "지금 느리다"가
아니라 **"현재 값이 작아서 안 보일 뿐, 사용자 수에 선형·초당으로 곱해지는 구조"**
라는 뜻이다. 그래서 각 항목에 현재값과 증가 계수를 함께 적었다.

---

# P0 — 트래픽 증가 시 장애로 직결

## P0-1. 가격 폴러가 전 사용자 심볼 합집합을 매초 갱신하고, 심볼당 개별 DB 쓰기를 한다

- **위치**: `be/internal/service/market_poller.go:167` (`tickPrice`), `:101` (`refreshPinned`),
  `be/internal/service/market_data_store.go:183` (`AllTrackedSymbols`), `:554` (per-symbol upsert)

- **현재 동작**:
  `marketPricePollInterval = 1 * time.Second`로 매초 `tickPrice`가 돈다. 대상은
  `pollSymbols()`가 돌려주는 **pinned 전체**이고, pinned는 `AllTrackedSymbols()` =
  `SELECT DISTINCT symbol FROM user_holdings` ∪ `SELECT DISTINCT symbol FROM watchlist_items`
  — **전체 사용자의 합집합**이다. 그 결과를 `RefreshPrices()`가 받아 심볼마다
  `UpsertLastPrice()`를 **한 건씩** 실행한다.

- **문제 원인**:
  1. `tickPrice`에는 **상한이 없다.** 같은 파일의 `tickFast`는 `marketFastSymbolCap = 40`,
     `tickSlow`는 `slowTargets`로 대상을 좁히는데 `tickPrice`만 무제한이다.
  2. 대상 집합이 **동시 접속자가 아니라 누적 사용자**에 비례한다. 한 번 담아 두고
     떠난 사용자의 종목도 영원히 pinned에 남는다.
  3. 쓰기가 심볼당 개별 statement다. 같은 리포지터리 계층에 배치 upsert 패턴이
     이미 있다(`daily_closes.go:57` chunk=400, `stock_scores.go:46` chunk=500).

- **근거**:
  - 20초 델타 실측: `Com_insert = 23/s`, **`Innodb_rows_inserted = 0/s`**.
    → 초당 23건의 쓰기가 **한 행도 바꾸지 않는다**(장 마감이라 가격 동일).
  - `PROCESSLIST` 30회 샘플링에서 잡힌 유일한 in-flight 쓰기:
    `INSERT INTO symbol_last_prices (symbol,last_price,currency) VALUES (?,?,?) ON DUPLICATE KEY UPDATE ...`
  - 현재 pinned ≈ 23종목 (distinct watch 7 ∪ held 16). `symbol_last_prices` 행 수는
    측정 중 3,359 → 3,730으로 증가.

- **예상되는 영향**:
  DB 쓰기 = `|pinned|` TPS, 상류 Toss 호출 = `ceil(|pinned|/200)` /초, **24시간 상시**.

  | 사용자 | 예상 pinned | DB upsert/s | Toss 배치/s |
  |---|---|---|---|
  | 현재 | 23 | 23 | 1 |
  | 1,000명 (1인 15종목, 중복 80%) | ~3,000 | **3,000** | 15 |
  | 5,000명 | ~8,000 | **8,000** | 40 |

  3,000 TPS의 단일행 upsert는 이 스키마·하드웨어에서 MySQL 포화 지점이다. 게다가
  아무도 접속하지 않은 새벽에도 같은 부하가 흐른다.

- **추천 해결 방법**:
  1. `UpsertLastPrice` 루프를 **다중행 upsert**로 교체(기존 chunk=400 패턴 재사용).
     즉시 23 → 1 statement/s.
  2. **변경 감지 추가.** `quotes_persist.go:23`에 이미 `shouldSkipLastPricePersist`가
     있는데 `market_data_store.go:554` 경로에는 빠져 있다. 값이 같으면 쓰지 않는다.
  3. `tickPrice` 대상에 **캡과 라운드로빈**을 도입한다 — `capActiveTargets`가
     이미 그 커서 구현을 갖고 있다(`market_poller.go:252`).
  4. pinned를 "누적 사용자"가 아니라 **최근 활동 사용자**로 좁힌다
     (`watchlist_items`/`user_holdings`에 최근 접속 사용자 조인).

- **개선 시 주의사항**:
  - 다중행 upsert는 한 statement가 잡는 락 범위를 넓힌다. chunk를 400 이하로 두고,
    아래 P2-16의 락 대기(현재 평균 123ms)를 개선 전후로 비교할 것.
  - 라운드로빈 캡을 넣으면 심볼별 가격 신선도가 `캡/|pinned|` 비율만큼 떨어진다.
    화면이 "몇 초 전 가격"을 감당하는지 먼저 정하고 캡을 정해야 한다.
  - 변경 감지는 **장중 동일가 체결**을 "변경 없음"으로 오인할 수 있다.
    `shouldSkipLastPricePersist`가 세션을 보는 이유가 그것이니 그 로직을 그대로 쓸 것.

---

## P0-2. 공개 AI 요약 엔드포인트에 single-flight가 없다 — stampede이자 cache penetration

- **위치**: `be/internal/service/stock_summary.go:200` (`Get`), `:209` (`go s.refreshOne`)
- **라우트**: `GET /api/v1/stocks/{symbol}/summary` — `OptionalAuth`, **비로그인 접근 가능**

- **현재 동작**:
  ```go
  if cached ... {
      if s.isFresh(&out) { return &out, nil }
      go s.refreshOne(symbol)     // ← 가드 없음
      return &out, nil
  }
  out, _, err := s.build(ctx, symbol, false)   // ← 캐시 미스: 동기 빌드, 가드 없음
  ```
  `stockSummaryStaleAfter = 6h`, `stockSummaryCacheTTL = 24h`.

- **문제 원인**:
  1. **stale 경로**: 6~24시간 구간의 항목은 읽을 때마다 `go refreshOne`을 띄운다.
     동시 독자 N명 → **N개의 동시 전체 빌드**. `grep`으로 확인한 결과
     `stock_summary*.go`에 `singleflight`/`inflight`/`doSync` 가드가 **하나도 없다**.
     (같은 코드베이스의 `NewsStore.List`는 `doSync`로 정확히 이 문제를 막아 두었다 —
     `news_store.go:337` 주석에 "40 동시 요청에서 1.7~6.6초" 실측이 남아 있다.)
  2. **미스 경로**: 캐시에 없으면 **동기** `build()`. 요청 슬롯을 점유한 채 돈다.
  3. 심볼이 **17,912개**이고 엔드포인트가 공개다. 캐시에 없는 심볼을 순회하면
     방어막이 IP 레이트리밋(30 req/s)뿐이다.

- **근거**:
  - 캐시에 없는 국내 종목 3개 콜드 측정: **406ms · 377ms · 159ms**, 웜은 **1.9~2.6ms**.
  - `build()`가 하는 일: fundamentals + financial history + earnings tab + insider tab
    + news + candles + scores (`stock_summary.go:275`).

- **예상되는 영향**:
  - **Stampede**: 인기 종목 하나가 stale이 되는 순간 동시 독자 수만큼 빌드가 뜬다.
    goroutine·DB 커넥션·Toss 쿼터를 동시에 태운다.
  - **Penetration/DoS**: 30 req/s × 0.4s = **초당 12 CPU-초**의 작업을 단일 IP가
    합법적으로 유발할 수 있다. API 컨테이너 CPU 상한은 2.0이다 → 즉시 포화.

- **추천 해결 방법**:
  1. 같은 코드베이스의 `doSync`(단일 비행) 헬퍼를 `Get`의 **두 경로 모두**에 적용.
     `NewsStore.List`가 참조 구현이다.
  2. 미스 경로를 **비동기 + 즉시 폴백**으로 바꾼다 — 빈 요약을 돌려주고 뒤에서 채운다.
     동기 빌드가 요청 슬롯을 잡는 것이 penetration의 본체다.
  3. **negative caching**: 존재하지 않거나 빌드 실패한 심볼을 짧은 TTL로 캐싱.
  4. 이 엔드포인트에 별도(더 낮은) 레이트리밋을 건다.

- **개선 시 주의사항**:
  - 비동기 폴백은 "요약이 잠깐 비어 보이는" 화면을 만든다. FE가 그 상태를
    구분해 표시하는지 먼저 확인할 것 — 빈 값을 "데이터 없음"으로 굳히면 안 된다.
  - single-flight의 대기 상한을 반드시 둘 것(`quoteRefreshWait = 5s` 선례).
    상한이 없으면 느린 빌드 하나가 모든 대기자를 잡는다.

---

## P0-3. 배경 작업이 사용자 요청과 같은 Toss 쿼터를 상시 소진한다

- **위치**: `be/internal/service/market_poller.go:368` (`tickSlow` → `refreshRawCandles`)

- **현재 동작**: 30초마다 `slowTargets`(= 최근 5분 내 조회된 심볼 ∪ `cared`)의
  종목마다 Toss 캔들을 받아 예열한다.

- **문제 원인**: Toss `/candles` 한도는 **20 req/s**인데, 예열이 그 버킷을
  사용자 요청과 공유한다. 한도를 넘기면 Toss가 429를 주고, `toss.Client.do`는
  429에 대해 재시도하지 않지만(`client.go:82`, 재시도는 401 토큰 갱신뿐) **폴러가
  1~30초마다 같은 대상을 다시 시도**하므로 폴러 층위에서 사실상 상시 재시도가 된다.

- **근거** (최근 30분, 실사용자 없음):
  ```
  375  toss api rate-limit low:      GET /api/v1/candles
  343  toss api rate-limit exceeded: GET /api/v1/candles   ← 분당 약 11건
  102  toss api rate-limit low:      GET /api/v1/stocks
   40  toss api rate-limit exceeded: GET /api/v1/stocks
  ```
  코드 주석에도 선행 실측이 남아 있다: "60초에 rate-limit 경고 451건 · remaining=0 261건"
  (`market_poller.go:44`).

- **예상되는 영향**:
  사용자가 종목 상세를 열어 캔들을 요청하는 바로 그 순간, 쿼터가 이미 배경 예열에
  소진돼 있으면 **사용자 요청이 실패하거나 폴백 경로로 떨어진다.** 즉 배경 최적화가
  전경 성능을 깎는다. 동시 사용자가 늘수록 `active` 집합이 커져 예열 대상도 함께
  늘어나므로 **자기 증폭**한다.

- **추천 해결 방법**:
  1. 예열에 **쿼터 예산을 분리 배정**한다 — `apibudget`에 이미 provider별 관리자가
     있으니, 배경 소비자에게 상한(예: 한도의 30%)을 두고 초과 시 이번 틱을 건너뛴다.
  2. 주석(`market_poller.go:378`)이 이미 결론을 적어 두었다 — 계층 저장 도입 후
     예열 이득이 "1.7초 → 4ms"에서 "11ms → 4ms"로 줄었다. **예열 자체를 끄는 것**을
     기본안으로 검토할 가치가 있다.
  3. 429를 받은 provider에 대해 폴러가 **backoff**하도록 한다(현재는 다음 틱에 그대로 재시도).

- **개선 시 주의사항**:
  예열을 끄면 "60일간 아무도 안 연 종목"의 인트라데이 차트 첫 열람이 1.7초가 된다
  (주석의 실측). 일봉 모달은 영향 없다. 그 트레이드오프를 UI 기준으로 먼저 합의할 것.

---

# P1 — 중요 병목 · 정합성

## P1-4. 뉴스 목록 캐시가 64개를 넘으면 통째로 비워진다 + 캐시 키에 사용자 입력이 들어간다

- **위치**: `be/internal/service/news_store.go:296` (`rememberList`), `:269` (`newsListCacheKey`)

- **현재 동작**:
  ```go
  cacheKey = market ⊕ sourceID ⊕ sort ⊕ Query ⊕ Symbol ⊕ limit   // Query·Symbol은 사용자 입력
  if len(st.listCache) >= 64 {
      // 만료분 제거 후에도 64 이상이면
      st.listCache = map[string]newsListCacheEntry{}   // ← 전면 flush
  }
  ```

- **문제 원인**:
  키 공간이 사용자 입력(`?q=`, `?symbol=`)이라 무한하다. 개발자도 이를 알고 상한
  64를 두었지만, 축출 정책이 **LRU가 아니라 전면 삭제**다. 서로 다른 검색어 64개가
  들어오면 **기본 목록(실사용자가 실제로 보는 것)까지 함께 날아간다.**

- **근거**:
  - 코드 주석 자체가 "필터 조합은 사용자가 만드는 값이라 키가 무한히 늘 수 있다"를 인정.
  - 캐시 미스 비용 실측(아래 P1-5): 미스 1건에 **67,558행 스캔 / 298~320ms**.
  - 액세스 로그: `/news/domestic` avg 47.8ms · **max 5.43s**, `/news/us` avg 55.8ms ·
    **max 5.87s** — 전형적인 이중 분포(히트 vs 미스).

- **예상되는 영향**:
  종목 상세 페이지가 `?symbol=`을 붙여 뉴스를 읽으므로, **사용자 64명이 서로 다른
  종목을 보기만 해도** 캐시가 비워진다. 그 직후 기본 목록 요청들이 전부 미스로
  몰린다. `doSync`가 중복 빌드는 막지만 **빌드 자체(300ms급)는 반드시 발생**하고,
  대기자는 최대 5초(`quoteRefreshWait`)까지 묶인다.

- **추천 해결 방법**:
  1. 전면 flush를 **LRU 축출**로 교체(가장 오래 안 쓰인 1건만 제거).
  2. 캐시를 **두 계층으로 분리**: 기본 목록(market×source×sort, 유한)은 별도 맵에
     상한 없이, 사용자 입력이 섞인 키만 작은 LRU에 둔다.
  3. `?q=` 검색은 애초에 캐싱 대상에서 빼는 것도 방법이다(히트율이 낮다).
  4. 이 캐시는 **프로세스 내부**다. API를 2대로 늘리면 각자 따로 빌드한다 —
     Redis로 옮기는 것을 함께 검토할 것.

- **개선 시 주의사항**:
  LRU로 바꾸면 메모리 상한이 "엔트리 수"로만 걸린다. 목록 응답이 최대 300건 ×
  기사 payload이므로 **엔트리 크기 기준 상한**도 함께 두어야 한다.

## P1-5. 뉴스 필터가 매칭 0건일 때 파티션 전체(67,558행)를 스캔한다

- **위치**: `be/internal/repository/news_repo.go:409` (`JSON_CONTAINS`), `:413` (`LIKE`)

- **현재 동작**:
  ```sql
  WHERE market = ?
    AND JSON_CONTAINS(symbols_json, JSON_QUOTE(?))          -- 종목 필터
    AND (headline LIKE ? OR summary LIKE ? OR headline_ko LIKE ? OR summary_ko LIKE ?)  -- 검색
  ORDER BY published_at DESC LIMIT ?
  ```

- **문제 원인**: 두 조건 모두 **인덱스를 탈 수 없다**(JSON 함수, 선행 와일드카드).
  MySQL은 `idx_news_market_published`를 `published_at DESC` 순으로 훑으며 필터링하고,
  `LIMIT`이 차면 멈춘다. **매칭이 없으면 멈출 수 없어 파티션을 끝까지 읽는다.**

- **근거** (`EXPLAIN ANALYZE`, 실데이터):
  | 케이스 | 실행시간 | 스캔 행 | 반환 |
  |---|---|---|---|
  | `symbol=005930` (흔함) | 2.28ms | 760 | 100 |
  | **`symbol=999999` (없음)** | **298ms** | **67,558** | 0 |
  | `q=반도체` (흔함) | 2.57ms | 532 | 100 |
  | **`q=zzqqxx` (없음)** | **320ms** | **67,558** | 0 |

- **예상되는 영향**:
  뉴스가 없는 종목의 상세 페이지를 열 때마다 298ms + 파티션 풀스캔이다. `news_articles`가
  81k → 500k로 늘면 **이 비용도 선형으로 늘어난다**(약 1.8초 추정). P1-4의 캐시 flush와
  겹치면 미스가 반복된다. 검색어는 사용자가 임의로 만들 수 있으므로
  **cache penetration 벡터**이기도 하다.

- **추천 해결 방법**:
  1. 종목 필터: `symbols_json`을 정규화한 **`news_article_symbols(symbol, article_id, published_at)`
     조인 테이블**로 빼고 인덱스를 건다. 또는 MySQL 8 **multi-valued index**
     (`CAST(symbols_json AS CHAR(16) ARRAY)`)를 쓴다.
  2. 텍스트 검색: `FULLTEXT` 인덱스 도입, 또는 검색 자체를 별도 저장소로.
  3. 즉효 완화: **매칭 0건 결과를 짧은 TTL로 negative caching**.

- **개선 시 주의사항**:
  조인 테이블을 만들면 뉴스 적재 경로에 쓰기가 하나 늘고, `symbols_json` 재태깅
  배치(`news_store_retag.go`)와 **동기화가 어긋날 여지**가 생긴다. 재태깅이
  두 저장소를 한 트랜잭션에서 갱신하도록 설계해야 한다.

## P1-6. `sort=popular`가 파티션 전체를 filesort 한다

- **위치**: `be/internal/repository/news_repo.go:386`
- **현재 동작**: `ORDER BY view_count DESC, published_at DESC` — `view_count`에 인덱스 없음.
- **근거** (`EXPLAIN ANALYZE`):
  ```
  -> Limit: 100 row(s)  (actual time=149..149 rows=100)
    -> Sort: view_count DESC, published_at DESC  (actual time=149..149)
      -> Index lookup using idx_news_market_published (market='domestic')
         (actual time=0.0435..140 rows=67558)      ← 67,558행 전부 읽어 정렬
  ```
- **예상되는 영향**: 인기순 정렬 1회당 **149ms + 67,558행 읽기**. 기사 수에 선형.
  `view_count`는 `IncrementViewCount`로 계속 바뀌므로 캐시 효율도 낮다.
- **추천 해결 방법**: `(market, view_count DESC, published_at DESC)` 복합 인덱스 추가.
  또는 인기 목록을 배치로 미리 계산해 캐싱(조회수는 실시간 정확도가 필요 없다).
- **개선 시 주의사항**: 인덱스를 추가하면 `IncrementViewCount`(조회마다 UPDATE)의
  쓰기 비용이 올라간다. 조회수 증가를 Redis에 모았다가 주기적으로 flush하는 편이
  인덱스와 궁합이 낫다.

## P1-7. 리서치 피드 `decorate()`의 N+1

- **위치**: `be/internal/service/research.go:722`, `:733`
- **현재 동작**:
  ```go
  for symbol, since := range earliestBySymbol {
      points, err := s.closes.ClosesSince(ctx, symbol, since)   // 심볼당 1쿼리
  }
  for id, since := range benchmarks {
      points, err := s.indexCloses.ClosesSince(ctx, id, since)  // 벤치마크당 1쿼리
  }
  ```
- **근거**:
  - 액세스 로그: `GET /api/v1/research/calls` **n=190, avg 503ms**.
  - 개별 쿼리 자체는 빠르다 — `EXPLAIN ANALYZE`상 PK 레인지 스캔 **0.14ms / 162행**.
    즉 비용의 대부분이 **순차 왕복 횟수**다.
  - 같은 함수의 `attachSocial`은 "Fixed number of queries regardless of page size"라고
    주석까지 달아 배치화해 두었다 — **종가 조회만 빠졌다.**
- **예상되는 영향**: 쿼리 수 = 페이지 내 distinct 심볼 수. 현재 `research_calls` 71건이라
  견디지만, 피드가 100건/페이지가 되면 100회 순차 왕복이다. 커넥션 점유 시간도 함께 늘어
  P1-9(풀 여유 부족)를 악화시킨다.
- **추천 해결 방법**: `WHERE symbol IN (...) AND trade_date >= ?`(가장 이른 날짜) 한 방으로
  받아 메모리에서 심볼별로 가른다. `attachSocial`이 이미 그 패턴이다.
- **개선 시 주의사항**: 심볼별 `since`가 다르므로 최소 날짜로 한 번에 받으면 **불필요한
  행을 더 읽는다.** 심볼 수 × 평균 234행이 메모리에 올라오므로, 페이지 크기 상한을
  함께 정해야 한다.

## P1-8. `BulkRegister`가 항목당 5쿼리를 순차 실행하고, 트랜잭션이 없다

- **위치**: `be/internal/service/holding_trades.go:111`
- **현재 동작**: 항목마다 `FindDuplicateTrade`(stocks JOIN) → `FindBulkRegisterTrade`
  → `Insert`/`UpdateTrade`, 그 뒤 영향받은 심볼마다 `syncHolding`(`ListSymbol` + `UpsertForUser`).
- **문제 원인**:
  1. **N+1**: 항목 N개 → 약 5N 쿼리, 전부 순차.
  2. **트랜잭션 없음** — `grep BeginTx|Commit|Rollback`가 이 파일에서 0건.
     루프 중간에서 에러가 나면 `return err`로 빠져나가 **앞쪽 심볼만 등록된 상태로 남는다.**
- **근거**: 코드 구조 + `holding_trades.go`에 트랜잭션 API 부재(실측 grep).
  스크린샷 임포트가 이 경로를 쓰며 한 번에 수십 종목을 넣는다.
- **예상되는 영향**: 50종목 일괄 등록 = **약 250회 순차 쿼리**를 한 요청이 점유.
  중간 실패 시 사용자는 "일부만 등록된" 포트폴리오를 보게 되고, 재시도하면
  `FindDuplicateTrade` 덕에 중복은 안 생기지만 **부분 상태가 정상인지 사용자가 알 수 없다.**
- **추천 해결 방법**:
  1. 전체를 **하나의 트랜잭션**으로 감싼다(부분 성공 제거).
  2. 중복 판정을 항목마다가 아니라 **한 번의 `IN` 조회**로 선행 수집.
  3. `syncHolding`을 심볼마다가 아니라 마지막에 **일괄 재계산**.
- **개선 시 주의사항**: 트랜잭션으로 묶으면 락 보유 시간이 길어진다. `holding_trades`는
  사용자별로 분리돼 있어 사용자 간 경합은 없지만, 같은 사용자의 동시 요청은 직렬화된다.
  또한 트랜잭션 안에서 **Redis 캐시 무효화를 하면 안 된다**(롤백 시 캐시만 날아간다) —
  커밋 후로 옮길 것.

## P1-9. 커넥션 풀 총합이 서버 상한의 76%다 — 스케일아웃 여유가 없다

- **위치**: `be/internal/repository/mysql.go:19` (`defaultMaxOpenConns = 50`), `docker-compose.yml`
- **현재 동작**: API 50 + 워커 8개 × 8 = **114** / `max_connections = 151`.
- **근거**:
  - `Max_used_connections = 34`, `Threads_connected = 19` (현재는 여유).
  - **이미 한 번 터졌다** — `mysql.go:35` 주석: "2026-08-22 실측: 「Too many connections」로
    DB 전체 정지". 그래서 CLI 기본값을 4로 낮춘 상태다.
  - `Aborted_connects = 267`.
- **예상되는 영향**: API 인스턴스를 2대로 늘리면 50+50+64 = **164 > 151** →
  기동 즉시 고갈. 워커를 하나 더 추가해도 같은 방향이다. 즉 **현재 구성은
  수평 확장이 원천 차단**돼 있다.
- **추천 해결 방법**:
  1. `max_connections`를 상향(메모리 여유 확인 후)하거나,
  2. API 풀을 실측 기반으로 낮춘다 — 현재 최대 사용 34이므로 50은 과하다.
  3. 장기적으로 **ProxySQL 같은 커넥션 멀티플렉서** 도입.
- **개선 시 주의사항**: 풀을 낮추면 SSE 스트림·배경 폴러가 커넥션을 오래 잡는 구간에서
  요청이 대기한다. `quantport_mysql_wait_total`(현재 0)을 지표로 두고 내릴 것.

---

# P2 — 낭비 · 잠재 위험

## P2-10. `symbol_last_prices` 무변경 쓰기 (P0-1의 하위 항목, 독립 개선 가능)
- **근거**: `Com_insert 23/s` vs `Innodb_rows_inserted 0/s`.
- **영향**: redo 로그·버퍼풀 더티 페이지·행 락을 **아무 데이터 변화 없이** 소비.
  P2-16의 락 대기에 직접 기여.
- **해결**: 값 동일 시 쓰기 생략(`shouldSkipLastPricePersist` 재사용).

## P2-11. `stock_scores`의 중복 인덱스 3개
- **근거**:
  ```
  idx_stock_scores_date   (trade_date)
  idx_stock_scores_score  (trade_date, score)
  idx_stock_scores_rank   (trade_date, score, peer_pct)
  ```
  앞의 둘은 셋째의 **접두사**라 완전히 중복이다. 테이블 235MB 중 인덱스 10.5MB.
- **영향**: 점수 배치가 chunk=500 다중행 upsert를 돌 때 **인덱스 3개를 모두 갱신**한다.
  쓰기 증폭 + 락 범위 확대.
- **해결**: `idx_stock_scores_date`, `idx_stock_scores_score` 제거.
- **주의**: `trade_date` 단독으로 도는 쿼리가 하나 있다 —
  `stock_scores.go:78`의 `SELECT DISTINCT trade_date FROM stock_scores ORDER BY trade_date DESC`.
  이 쿼리는 **`trade_date`로 시작하는 아무 인덱스나** 쓸 수 있으므로 남는
  `idx_stock_scores_rank(trade_date, score, peer_pct)`가 그대로 커버한다.
  그래도 제거 전후로 이 쿼리의 `EXPLAIN`을 비교해 커버링이 유지되는지 확인할 것.

## P2-12. `candle_bars_backup_20260822` — 1.27GB / 17.6M행
- **근거**: `information_schema.TABLES` — 1,273MB, 인덱스 0, `CREATE_TIME 2026-08-22 22:44`.
  코드 참조는 관리자 DB 콘솔 카탈로그 1곳뿐(`db_admin_catalog.go:290`, 읽기 전용).
- **영향**: 4GB 버퍼풀의 **32%에 해당하는 크기**. 콘솔에서 훑거나 백업/덤프가 돌 때
  실제 워킹셋을 밀어낸다. 디스크·백업 시간도 함께 먹는다.
- **해결**: 롤업 검증이 끝났으면 드롭. 유지해야 하면 **드롭 예정일을 문서에 명시**.
- **주의**: 계층 롤업은 되돌릴 수 없는 변환이다(`batch_job.go:29` — "접힌 1분봉은 삭제된다").
  롤업 결과 검증이 끝나기 전에는 지우지 말 것.

## P2-13. 뉴스 폴러가 고정 2분 주기로 외부 API를 태운다
- **근거**: `newsPollInterval = 2 * time.Minute`, 소스 5개 staggered.
  `/health/api-quotas` 실측(실사용자 0):
  | provider | 일 소비 | 일 한도 | 소진율 |
  |---|---|---|---|
  | naver | 5,538 | 24,000 | 23% |
  | **kakao** | **1,170** | **3,000** | **39%** |
  | edgar_bulk | 10,207 | (미설정) | — |
- **영향**: 사용자 수와 무관한 **고정 비용**이다. 지금도 kakao 한도의 39%를 아무도
  안 볼 때 쓴다. 폴링 주기를 조금만 당기거나 소스를 추가하면 한도에 닿는다.
- **해결**: 장 시간·요일에 따라 주기를 가변으로(마감 후·주말 완화). 소비량 대비
  **신규 기사 수집률**을 로깅해 주기 근거를 데이터로 잡을 것.
- **주의**: 뉴스 신선도는 이 서비스의 체감 품질이다. 주기를 늘리기 전에 "몇 분 늦어도
  되는가"를 화면 기준으로 정해야 한다.

## P2-14. Redis 히트율 59%
- **근거**: `keyspace_hits 2,640,085 / keyspace_misses 1,807,984` → **59.4%**.
  `expired_keys 1,277,817`, `evicted_keys 0`.
- **영향**: 미스의 상당수가 P0-2·P1-5 같은 비싼 재계산으로 이어진다. 축출은 0이므로
  **용량 부족이 아니라 TTL이 짧거나 키가 과분할된 것**이다.
- **해결**: 미스가 집중되는 키 접두사를 계측해 TTL을 조정. `used_memory_peak 592MB`가
  `maxmemory 1GB`의 59%까지 갔던 이력이 있으므로, TTL을 늘릴 때 **축출이 시작되는
  지점**을 함께 감시할 것(축출이 시작되면 `volatile-lru`가 캐시를 버려 stampede가 된다).

## P2-15. `cared` 집합이 항상 비어 있다 — 설계 의도가 실행되지 않는다
- **위치**: `market_poller.go:130` → `quotes_overlay.go:65` (`loadWatchlistSymbols`)
- **근거**:
  ```go
  func (s *QuotesService) loadWatchlistSymbols(ctx context.Context) []string {
      userID, ok := authctx.UserID(ctx)
      if !ok { return nil }        // ← 폴러 ctx에는 사용자가 없다
  ```
  폴러는 `server.go`의 `bgCtx`(= `context.Background()` 파생)로 돈다.
  따라서 `refreshPinned`의 `watch`/`held`는 **항상 nil**이고 `p.cared`도 항상 빈 슬라이스다.
- **영향**: 기능적 버그는 아니다(pinned는 `AllTrackedSymbols`로 이미 같은 집합을 얻는다).
  하지만 `tickSlow`의 예열 대상이 "관심 ∪ 보유 ∪ 조회중"이라는 **주석과 실제가 다르다** —
  실제로는 "조회중"만이다. P0-3의 완화책으로 도입된 `cared` 분리가 **아무 일도 하지 않는다.**
- **해결**: 폴러 경로에서는 `ListAllSymbols`(사용자 무관 전역 조회)를 쓰도록 바꾸거나,
  `cared` 개념을 걷어내고 주석을 실제에 맞춘다.
- **주의**: `cared`를 진짜로 채우면 P0-3의 `/candles` 소진이 **더 심해진다**.
  고치기 전에 예열 정책부터 결정할 것.

## P2-16. 행 락 대기 21,974건 · 최대 51.9초
- **근거** (Uptime 32.8h): `Innodb_row_lock_waits = 21,974` (≈670/h),
  `Innodb_row_lock_time = 2,709초` 누적, `avg 123ms`, **`max 51,895ms`**.
  현재 `Innodb_row_lock_current_waits = 0`, 데드락 카운터는 노출 안 됨.
- **영향**: 평균 123ms의 락 대기는 쓰기 경로가 이미 서로를 기다린다는 뜻이다.
  단발 51.9초는 대형 배치 트랜잭션이 존재함을 시사한다.
- **해결**: 후보는 두 곳이다 — ① `symbol_last_prices` 동일 행에 대한 폴러/요청 경로
  동시 upsert(P0-1·P2-10), ② chunk=400~500 다중행 upsert. 개선 전후로 이 카운터를
  비교하는 것이 가장 값싼 검증이다.
- **주의**: 이 카운터는 **누적값**이라 재기동 시 초기화된다. 개선 효과를 보려면
  같은 Uptime 구간으로 정규화해 비교할 것.

## P2-17. OFFSET 기반 페이지네이션
- **위치**: `research_repo.go:280`, `stamp_repo.go:209`, `news_repo.go:474`,
  `news_briefing_repo.go:148`, `stock_scores.go:80`
- **영향**: 현재 데이터가 작아 문제없다(`research_calls` 71행). 깊은 오프셋은
  `OFFSET + LIMIT` 행을 읽고 버리므로, 스탬프·리서치가 수만 건이 되면 뒤쪽 페이지가
  선형으로 느려진다.
- **해결**: 정렬 키가 이미 `(published_at DESC, id DESC)`·`(stamped_at DESC, id DESC)`로
  유니크하므로 **키셋(seek) 페이지네이션**으로 바꾸기 쉽다.
- **주의**: 키셋으로 바꾸면 "N페이지로 점프"가 불가능해진다. FE가 페이지 번호 UI를
  쓰는지 먼저 확인할 것.

## P2-18. TTL 없는 Redis 키가 심볼 수만큼 무한 증가
- **근거**: SCAN 표본 3,002키 중 TTL 없는 키 379개, **전부 `md:rev:*`**.
  전체 28,121키 중 3,675개가 TTL 없음.
- **영향**: 설계상 의도된 것이다(docker-compose 주석: 캐시 무효화 카운터라 사라지면
  낡은 캐시가 새 것으로 보인다 → `volatile-lru`로 보호). 다만 **상한이 없어**
  심볼 17,912개 전부가 조회되면 그만큼 영구 키가 쌓인다. 지금은 약 123KB로 무해.
- **해결**: 지금 조치할 필요는 없다. `maxmemory`에 근접하면 `volatile-lru`가
  TTL 있는 캐시만 버리므로, **영구 키 비중을 모니터링 지표로 둘 것**.

---

# 우선순위 요약과 작업 순서

## 심각도 정렬

| 순위 | 문제 | 현재 영향 | 증가 계수 | 수정 난이도 |
|---|---|---|---|---|
| 1 | **P0-1** 가격 폴러 무제한 × 초당 × 심볼당 개별 쓰기 | 23 무의미 write/s | **누적 사용자에 선형** | 중 |
| 2 | **P0-2** 요약 API single-flight 부재 (공개) | 콜드 400ms | **동시 요청에 선형** | 하 |
| 3 | **P0-3** 배경 예열이 Toss 쿼터 상시 초과 | 분당 11건 초과 | 동시 사용자에 선형 | 하 |
| 4 | **P1-9** 커넥션 풀 114/151 | 여유 없음 | 인스턴스 수에 선형 | 하 |
| 5 | **P1-8** BulkRegister 트랜잭션 부재 | 부분 저장 가능 | — (정합성) | 하 |
| 6 | **P1-4** 뉴스 캐시 전면 flush | 미스 시 300ms | 동시 사용자에 선형 | 중 |
| 7 | **P1-5** 뉴스 필터 파티션 풀스캔 | 298~320ms | **기사 수에 선형** | 상 |
| 8 | **P1-6** `sort=popular` filesort | 149ms | 기사 수에 선형 | 하 |
| 9 | **P1-7** 리서치 피드 N+1 | 503ms | 페이지 크기에 선형 | 하 |
| 10~18 | P2 항목 | 낭비 위주 | — | 하 |

## 권장 작업 순서

**1단계 — 값싸고 효과 큰 것부터 (1~2일, 위험 낮음)**
1. **P0-2** 요약 API에 `doSync` 적용 + 미스 경로 비동기화.
   기존 `NewsStore.List` 구현을 그대로 재사용하므로 **가장 적은 코드로 가장 큰
   장애 리스크를 제거**한다.
2. **P2-10 / P0-1(1·2단계)** `symbol_last_prices` 변경 감지 + 다중행 upsert.
   유휴 DB 쓰기 23/s → 거의 0. 측정 지표(`Com_insert` vs `Innodb_rows_inserted`)가
   이미 있어 효과 검증이 즉시 가능하다.
3. **P1-6** `(market, view_count, published_at)` 인덱스 1개 추가.
4. **P2-11** 중복 인덱스 2개 제거 — 2번의 쓰기 개선과 상승 작용.

**2단계 — 정합성·확장성 (3~5일)**
5. **P1-8** BulkRegister 트랜잭션화 + 중복 판정 배치화. *정합성 문제라 성능과 무관하게 우선.*
6. **P1-7** 리서치 피드 종가 조회 배치화.
7. **P1-9** 풀 크기를 실측(Max_used 34) 기반으로 재조정 + `max_connections` 상향.
8. **P0-3 / P2-15** 예열 정책 결정 → `cared` 실장 또는 제거, provider 예산 분리.

**3단계 — 구조 변경 (1~2주, 설계 합의 필요)**
9. **P0-1(3·4단계)** `tickPrice` 캡·라운드로빈, pinned를 활동 사용자 기준으로 축소.
   *1단계에서 쓰기 비용을 먼저 줄여 두면 이 작업의 긴급도가 낮아진다.*
10. **P1-4** 뉴스 캐시 LRU + 계층 분리 (필요 시 Redis 이전).
11. **P1-5** 종목 조인 테이블 또는 multi-valued index, 검색 FULLTEXT.

**상시**
- **P2-12** 롤업 검증 완료 후 1.27GB 백업 테이블 드롭 (예정일 명시).
- **P2-13** 뉴스 폴링 주기 가변화 — 먼저 "수집률 대비 소비량" 로깅부터.
- **P2-14/16/18** 지표로 전환: Redis 히트율, 락 대기 카운터, 영구 키 비중.

## 이번 감사에서 문제로 분류하지 **않은** 것

근거를 확인했고 이미 적절히 처리돼 있어 제외한다.

- **배치 중복 실행** — `HasPending`으로 큐 중복을 막고, `ClaimNext`가
  `SELECT ... FOR UPDATE`로 원자적 클레임을 한다. 재기동 시 stale run도 정리한다.
- **뉴스 목록 stampede** — `doSync` 단일 비행이 이미 있고, 주석에 실측 근거가 있다.
- **예시 화면 부하** — `sample_cache.go`로 오늘 해결됨. 실측 콜드 478ms → 웜 2.4ms.
- **Toss 클라이언트 재시도 폭증** — 재시도는 401 토큰 갱신 1회뿐. 429는 재시도하지 않는다.
  (다만 폴러 층위의 반복 시도는 P0-3으로 별도 분류했다.)
- **full table scan** — `Handler_read_rnd_next = 0/s`. 스캔은 전부 인덱스 경유다.
- **goroutine 무제한 팬아웃** — `quotes_batch`·`quotes_watchlist` 모두 세마포어 있음.
- **Redis 축출로 인한 캐시 붕괴** — `evicted_keys = 0`, 현재 28MB / 1GB.
