# 출시 전 리팩토링 · 최적화 점검서

> 작성: 2026-08-19 · 대상: `master` (v0.1 baseline snapshot 이후 작업본)
> 목적: 클라우드 배포 전에 **무엇을 고쳐야 하는지**를 우선순위와 트레이드오프로 정리한다.

---

## 0. 이 문서의 근거 (측정 조건)

숫자는 추정이 아니라 **실측**이다. 재현 방법은 [부록 A](#부록-a-측정-재현-방법)에 있다.

| 항목 | 값 |
|---|---|
| 측정 환경 | 로컬 Docker (Windows 11, 15.5GiB RAM), MySQL은 호스트(`host.docker.internal:3306`) |
| API 컨테이너 | `quant-port-api` uptime 11h, healthy |
| 동시 사용자 | 0 (측정자 본인 요청만) |
| 액세스 로그 표본 | 2,222 요청 (컨테이너 로그 전량에서 추출) |
| 코드 규모 | BE Go 약 91,000 LOC / FE TS·TSX 약 63,000 LOC |
| 엔드포인트 수 | 약 150개 (`internal/server/router.go`) |
| 테스트 | `*_test.go` 151개, **CI 워크플로 없음** |

> ⚠️ 한 가지 주의: 액세스 로그 표본에는 2026-08-18 11:13~11:20 사이 **호스트 네트워크 단절 구간**이 섞여 있다. 이 구간에는 MySQL upsert와 외부 HTTP가 동시에 `context deadline exceeded`를 냈다. 이때의 30초 응답은 코드 결함이 아니라 환경 장애다 — 다만 **"의존성이 죽으면 요청이 30초 동안 살아 있다"**는 사실 자체는 코드 문제이며, 아래 P0-2로 다룬다.

---

## 1. 시스템 구성 현황

### 1.1 프로세스 · 컨테이너

`docker-compose.yml` 기준 **9개 컨테이너**. 배치 잡은 이미 독립 워커로 분리되어 있다(잘 되어 있는 부분).

| 컨테이너 | 역할 | 트리거 | idle 메모리 | idle CPU |
|---|---|---|---|---|
| `redis` | hot cache | 상시 | 423 MiB | 0.5~1% |
| `api` | HTTP API + 폴러 + LLM 배치 큐 | 상시 | 193 MiB | **21~26%** |
| `worker-close-domestic` | KRX 종가 적재 | KRX 마감 +20분 | 8 MiB | 0% |
| `worker-close-overseas` | US 종가 적재 | US 마감 +20분 | 7 MiB | 0% |
| `worker-scores` | 종목·지수 종합 점수 | 매일 00:25 KST | 6 MiB | 0% |
| `worker-research` | 리서치 콜 판정 | 매일 07:00 KST | 7 MiB | 0% |
| `worker-earnings` | 실적 발표 적재 | 국내 시즌 10–20/2h · 비시즌 18:30 / 해외 06:00 KST | 9 MiB | 0% |
| `worker-index` | 지수 종가 백필 | 수동 | 7 MiB | 0% |
| `worker-fetch` | 종가 결손 채우기 | 수동 | 8 MiB | 0% |

**구성상 빠져 있는 것**
- **MySQL이 compose에 없다.** `host.docker.internal:3306`으로 붙는다 → 클라우드에서는 관리형 DB(RDS/Cloud SQL) 또는 별도 컨테이너로 대체하는 작업이 통째로 남아 있다.
- **FE(Next.js)가 compose에 없고 Dockerfile도 없다.** `npm run dev`로만 돈다. `next.config.ts`에 `output: "standalone"`도 없다.
- 어떤 서비스에도 **리소스 리밋(`deploy.resources`)이 없다.**

### 1.2 데이터 흐름 (읽기 경로)

```
브라우저 ──▶ Next.js rewrite (/api/* → BE) ──▶ chi Router
                                                 │
                        ┌────────────────────────┼────────────────────────┐
                        ▼                        ▼                        ▼
                  Redis (hot)             MySQL (SoT)            외부 API (Toss/DART/…)
                   1~5s TTL              write-through            apibudget 게이트
```

정책은 `docs/AGENT_STATE.md`대로 **MySQL = source of truth, Redis = hot cache**이며, 조회 경로에서 외부 HTTP를 부르지 않는 것이 원칙이다. 다만 이 원칙이 **완전히 지켜지지는 않는다** — P1-1 참조.

### 1.3 상시 폴러 (API 프로세스 안)

`api` 컨테이너가 무부하에서 CPU 22~26%를 쓰는 이유가 여기 있다.

| 폴러 | 주기 | 위치 |
|---|---|---|
| 시세 fast tick | **1초** | `market_poller.go:15` |
| 시세 slow tick | 30초 | `market_poller.go:16` |
| 환율 | 15초 | `exchange_rate.go:19` |
| 지수 | 1분 | `market_index_store.go:18` |
| 랭킹 | 1분 | `market_rankings_poller.go:15` |
| 뉴스 (5개 소스 stagger) | 2분 | `news_store.go:27` |
| 뉴스 US 기업별 | 8분 | `news_store.go:28` |
| 뉴스 브리핑 스케줄 체크 | 1분 | `news_briefing_schedule.go:28` |
| 포트폴리오 동기화 | 60초 | `portfolio_sync.go:12` |
| 배치 큐 폴 (4종) | 4초 | `batch_worker.go:20` |
| 인사이트 / 임포트 정리 | 6시간 | `insights.go:267`, `import_job.go:665` |

fast tick은 활성 심볼 40개 상한(`marketFastSymbolCap`)·동시 4개(`marketRefreshConcurrency`)로 이미 잘 조여져 있다. 그럼에도 1초 주기 자체가 상시 비용이다.

### 1.4 외부 API 실사용량 (`/health/api-quotas`, 이번 달 누적)

| Provider | 월 호출 | 한도 | 비고 |
|---|---:|---|---|
| naver | **90,999** | 24,000/일 | 뉴스 폴러. 사용자 수와 무관한 고정 비용 |
| kakao | 22,279 | 3,000/일 | 뉴스 폴러 |
| papago_text | 17,885 | 6,000/일 | 해외 뉴스 번역 |
| finnhub_news | 4,852 | 50/분 | |
| upbit | 4,827 | 10/분 | BTC 지수 |
| openai | **2,193** | 500/일 · 100만 토큰/일 | 배치·관리자 수동만 |
| edgar | 1,476 | 8/분 | |
| fred | 446 | 5,000/일 | |
| goldprice | 128 | 900/월 | 여유 적음 |

**좋은 소식**: 이 비용은 전부 배치·폴러가 만들며 **방문자 수에 비례하지 않는다.** LLM 지출 규칙(`AGENT_STATE.md`)이 실제로 지켜지고 있다. 출시 후 트래픽이 늘어도 외부 API 청구서는 그대로다.

**나쁜 소식**: 반대로 사용자가 0명이어도 이 비용이 그대로 나간다. 월 9만 건의 naver 호출은 출시 전에 한 번 재검토할 가치가 있다.

---

## 2. 실측 응답 시간

### 2.1 워밍 상태 (캐시 적중, 정상 상태)

| 엔드포인트 | 응답 | gzip | 원본 | 판정 |
|---|---:|---:|---:|---|
| `/quotes/indices` | 2.5 ms | 1.5 KB | 4.1 KB | ✅ |
| `/quotes?filter=all&limit=100` | 3 ms | 3.8 KB | 30 KB | ✅ (캐시 적중 시) |
| `/quotes/{sym}` | 23 ms | 0.4 KB | 0.6 KB | ✅ |
| `/quotes/{sym}/terminal-snapshot` | 16 ms | 1.0 KB | 9.4 KB | ✅ |
| `/quotes/{sym}/candles?interval=1d&limit=250` | 5 ms | 4.0 KB | 19 KB | ✅ |
| `/quotes/batch?symbols=×6` | 88~361 ms | 0.6 KB | 1.9 KB | ⚠️ 편차 큼 |
| `/stocks/{sym}/summary` | 2.5 ms | 0.9 KB | 1.5 KB | ✅ |
| `/stocks/{sym}/earnings` | 8 ms | 1.4 KB | 5.1 KB | ✅ |
| `/stocks/{sym}/daily-closes?limit=250` | 5 ms | 4.3 KB | 22 KB | ✅ |
| `/stocks/scores/top` | 5 ms | 1.0 KB | 4.1 KB | ✅ (`050_perf_indexes` 효과) |
| `/stocks/search?market=domestic&limit=50` | **210 ms** | 2.9 KB | 19 KB | ⚠️ |
| `/rankings?type=…&marketCountry=KR` | **150 ms** | 3.5 KB | 19 KB | ⚠️ |
| `/rankings?type=…&marketCountry=US` | **900~1,110 ms** | 4.1 KB | 19 KB | 🔴 |
| `/news/domestic` | 13~37 ms | 29 KB | **98 KB** | ✅ |
| `/news/us` | **660~2,270 ms** | 27 KB | **94 KB** | 🔴 |
| `/news/{market}/briefing` | 4 ms | 1.8 KB | 3.5 KB | ✅ |
| `/news/{market}/dashboard` | 2.6 ms | 11 KB | 36 KB | ✅ |
| `/earnings/calendar` | 79~101 ms | 28 KB | **179 KB** | ⚠️ |
| `/research/calls` | 30~53 ms | 6.6 KB | 22 KB | ✅ |
| `/market/data-settings` | 2.6 ms | 0.2 KB | 0.2 KB | ✅ |

**gzip은 잘 켜 두었다** (`router.go`의 `chimw.Compress(5)`). 뉴스 목록 98 KB → 29 KB, 실적 캘린더 179 KB → 28 KB. 모바일 회선에서 체감 차이의 대부분이 여기서 온다.

### 2.2 콜드 상태 (캐시 미스)

| 엔드포인트 | 콜드 | 워밍 | 배수 |
|---|---:|---:|---:|
| `/quotes` (기본 파라미터) | **4.2~5.0 s**, 관측 최대 20 s | 2.5 ms | 약 2,000× |
| `/rankings` (US) | 1.1 s | — | — |
| `/news/us` | 2.3 s | 0.8 s | — |

`/quotes`는 **인증이 필요 없는 공개 엔드포인트**인데 캐시 TTL이 3초(`MarketDataListCacheSec`)다. 즉 3초마다 한 번은 누군가 4~5초를 기다린다.

### 2.3 액세스 로그 집계 (2,222 요청)

| 엔드포인트 | N | 평균 | 최대 |
|---|---:|---:|---:|
| `/quotes/{sym}` | 141 | 2,457 ms | 39,480 ms |
| `/stocks/{sym}/earnings` | 46 | 7,848 ms | 30,016 ms |
| `/news/domestic` | 32 | 7,888 ms | 30,271 ms |
| `/rankings` | 32 | 6,838 ms | 30,031 ms |
| `/market/data-settings` | 62 | 5,049 ms | 30,126 ms |
| `/auth/me` | 61 | 986 ms | 30,004 ms |
| `/quotes/batch` | — | — | 39,480 ms |

`/market/data-settings`와 `/auth/me`처럼 **DB 한 줄 읽는 요청까지 30초**를 찍었다는 것이 핵심이다. 특정 엔드포인트가 느린 게 아니라 **프로세스 전체가 멎었다**. 원인은 위에서 밝힌 호스트 네트워크 단절이지만, 30초는 `http.Server{WriteTimeout: 30s}`가 잘라 준 값일 뿐 — 코드 스스로 포기한 지점이 아니다.

---

## 3. 페이지별 API 맵

> 아래는 **import 그래프 정적 추적** 결과다(각 `page.tsx`에서 도달 가능한 `/api/v1/*` 문자열). 실제 런타임 호출의 상위 집합 — 한 페이지가 공유 컴포넌트(`stock-workspace`, `research`)를 끌어오면 그 API도 함께 잡힌다. **정확한 런타임 호출 수는 브라우저 실측이 필요**하며, 브라우저 패널이 표시되지 않아 이번에는 못 했다(부록 A-4).

### 3.1 공유 셸 — 모든 페이지가 지불

`app/layout.tsx` + `(main)/layout.tsx` → 38개 파일에서 도달:

```
/auth/me            /watchlist           /watchlist/order
/quotes             /quotes/batch         /quotes/{sym}        /quotes/{sym}/orderbook
/quotes/{sym}/trades     /quotes/{sym}/terminal-snapshot       /rankings
/stocks/{sym}
```

(2026-08-20: `/portfolio/*`·`/insights/dashboard`·`/accounts`·`/orders`·`/broker/toss/*` 는 계좌 연동 제거로 사라졌다.)

여기에 **셸이 `/quotes`와 `/rankings`를 포함**한다는 점이 중요하다 — 2.2에서 본 콜드 4~5초짜리가 모든 페이지 진입 비용에 들어간다.

### 3.2 페이지별 고유 API

| 페이지 | 고유 API 수 | 주요 엔드포인트 |
|---|---:|---|
| `/` (홈), `/quotes` | 셸만 | 셸 API + 심볼 워크스페이스 |
| `/market` | 29 | `stocks/scores/top`, `stocks/{sym}/{fundamentals,insider,financials/history,summary,score}`, `stamps/*`, `earnings/*` |
| `/analysis` | 46 | `research/*` 전체, `stamps/*`, `stocks/{sym}/*`, `earnings/*`, `import/holdings/*` |
| `/holdings` | 33 | `user/holdings/*`, `user/holdings/trades*`, `user/cash-balances`, `user/price-targets`, `sample/*` |
| `/news/domestic`, `/news/us` | 36 | `news/{market}/dashboard`, `research/*`, `indices/scores`, `stocks/scores/batch` |
| `/earnings/domestic`, `/earnings/us` | 12 | `earnings/{calendar,summary,highlights,stats,surprises}`, `stocks/earnings/batch` |
| `/journal` | 10 | `journal/notes*`, `user/price-targets*`, `sample/*` |
| `/reports`, `/reports/[id]`, `/reports/new`, `/reports/authors/[userId]` | — | `research/*` |
| `/stamps`, `/targets`, `/watchlist`, `/transactions`, `/settings`, `/portfolio`, `/risk`, `/backtest` | 소수 | 각 기능 전용 |
| `/admin/*` (7개) | 25 (공유 허브) | `admin/*` 전체 — `lib/admin.ts` 한 파일을 공유해 페이지 구분이 안 됨 |
| `/dashboard`, `/calendar`, `/financials`, `/earnings`, `/news`, `/disclosures`, `/ai-analysis` | 0 | **redirect 전용 스텁** (정상) |

### 3.3 FE 폴링 주기 — 사용자당 요청량

이게 서버 부하를 결정한다. 사용자 1명이 해당 화면을 열어 두면:

| 훅 / 컴포넌트 | 주기 | 대상 |
|---|---|---|
| `use-quote-detail` | **2초** | `/quotes/{sym}` |
| `use-quote-candles` (forming bar) | **2초** | 캔들 tail |
| `market-radar-config` quote overlay | **2초** | `/quotes/batch` |
| `use-quotes-list` | 5초 | `/quotes` |
| `quotes-symbol-list` (pin) | 5초 | `/quotes/batch` |
| `watchlist-client` | 5초 | `/watchlist` |
| `holdings-page-client` | 5초 | `/quotes/batch` |
| `admin-batch-panel` | 4초 | `/admin/batch/jobs` |
| `use-market-indices` | 30초 | `/quotes/indices` |
| `market-radar-config` rankings | 60초 | `/rankings` |

**동시 접속 100명이 `/market`을 열어 두면** 2초 폴링 3종 + 5초 폴링 → 대략 **초당 200~300 요청**. 여기에 콜드 미스가 섞이면 4~5초짜리 `/quotes`가 물린다.

`isPageVisible()` 가드가 걸려 있어 백그라운드 탭은 폴링하지 않는다 — 이건 잘 되어 있다.

---

## 4. 우선순위별 조치 항목

각 항목: **현상 → 근거 → 조치 → 트레이드오프**.

### P0 — 출시 전 필수 (없으면 장애 또는 사고)

#### P0-1. 레이트 리밋이 전혀 없다

- **현상**: `internal/middleware/`에 rate limit·throttle 미들웨어가 없다. `router.go`의 미들웨어 체인은 `RequestID → RealIP → Recoverer → Logger → CORS → Compress`뿐이다.
- **~~악화 요인~~ 정정 (08-19)**: `/quotes/batch`가 "개수 제한이 없다"고 쓴 것은 **틀렸다.** `service/quotes_batch.go:11`에 `batchQuoteSummariesMax = 100`이 있고 동시성도 8로 묶여 있다(`batchQuoteConcurrency`). 팬아웃은 이미 100건으로 유계다.
  실제로 남아 있던 문제는 두 가지였고 **둘 다 처리했다**:
  1. 핸들러가 쿼리 문자열을 `strings.Split`으로 통째로 쪼갠 뒤 서비스가 잘랐다. URL은 `MaxHeaderBytes`(1MB)까지 오므로 콤마 14만 개면 요청마다 14만 원소 슬라이스가 잡혔다 → 파싱 상한 `4 × 100`으로 제한. **완료**
  2. 100건 초과분을 **조용히 버렸다**. 보유·메모·목표가처럼 사용자 데이터에서 심볼을 만드는 화면은 이 수를 넘길 수 있는데 응답 어디에도 그 사실이 없었다 → 응답에 `limit` 필드 추가. **완료**
- **`limit` 상한 — 정정 (08-19)**: "핸들러에서 강제해야 한다"고 썼지만 **이미 전부 클램프되어 있었다.** `/quotes` 200(`quotes_stocks.go:51`), `/stocks/search` 200(`normalizeSearchLimit`), `/news` 200·해외 300(`news_repo.go:375`). 할 일이 없다.

- **심볼 목록 엔드포인트 전수 점검 — 완료 (08-19)**: `?symbols=`를 받는 공개 핸들러 여섯 개를 모두 확인했다.

  | 엔드포인트 | 점검 전 | 조치 |
  |---|---|---|
  | `/quotes/batch` | 서비스 100 | 파싱 상한 + `limit` 노출 |
  | `/stocks/scores/batch` | 핸들러 200 | 그대로 |
  | `/stocks/earnings/batch` | 핸들러 상한 有 | 그대로 |
  | `/earnings/calendar` | `EarningsSymbolsMax` | 그대로 |
  | **`/stocks`** | **상한 없음** | 100 + `limit` 노출 |
  | **`/market-indicators/prices`** | **상한 없음** | 100 + `limit` 노출 |

  파싱은 `handler/symbols.go`의 `parseSymbolList`로 한 군데에 모았다. 뒤의 두 개는 **FE에 호출부가 없어**(`fetchStocksBatchClient`는 export만 되고 호출 0, `/market-indicators/prices`는 FE 참조 0) 상한 추가에 사용자 영향이 없다.

  **상한을 넣고도 남는 것**: 500개를 보내 확인하니 `/stocks`가 **429**를 돌려줬다 — 카탈로그에 없는 심볼은 Toss 레퍼런스를 부르기 때문이다. 상한으로 5배 줄었지만, 공개 경로에서 한 요청이 업스트림 100건을 부르고 **실시간 시세 폴러가 쓰는 Toss 한도를 잠식**하는 벡터는 남아 있다. 레이트 리밋(아래)이 들어가야 실질적으로 닫힌다.

- **레이트 리밋 완료 — 08-19**: `middleware/ratelimit.go`. **로그인 사용자는 `user_id`, 비로그인은 IP**로 센다(기본 10 rps/burst 20, 30 rps/burst 60). 임계값은 FE 폴링 실측(`/quotes` 화면 약 1.4 req/s, 탭 3개 약 4 req/s)의 2.5배로 잡았다. 5개 라우트 그룹 각각에 **인증 미들웨어 뒤**로 걸었다 — 앞에 두면 컨텍스트에 사용자가 없어 로그인 사용자까지 공유 IP 버킷에 떨어진다.

  | 검증 | 결과 |
  |---|---|
  | 정상 폴링 4 req/s × 10초 | 200 × 40, **429 없음** |
  | 남용 200 동시 | 159 통과 / 41 차단 |
  | 429 응답 | `Retry-After: 1` 확인 |
  | SSE | 정상 스트리밍 유지 |
  | 액세스 로그 | 429가 상태 코드로 기록됨 |

  버킷은 idle 10분이면 회수한다(없으면 IP마다 영구히 쌓이는 느린 누수다). `RATE_LIMIT_ENABLED=false`로 **재배포 없이 끌 수 있다** — 임계값을 잘못 잡았을 때의 탈출구다.

- **부수 발견 및 수정: `X-Forwarded-For` 위조로 한도 우회**. chi의 `middleware.RealIP`는 XFF를 조건 없이 믿고 `RemoteAddr`를 덮어쓴다. 로그용으로는 괜찮지만 레이트 리밋 키로 쓰면 헤더만 바꿔 매번 새 버킷을 받는다. `middleware/realip.go`로 교체했다 — (1) **loopback·사설 대역 peer에서 온 헤더만** 신뢰하고, (2) nginx·ALB가 기존 헤더에 **덧붙인다**는 점을 이용해 **오른쪽 hop부터** 읽는다(맨 왼쪽은 공격자가 심을 수 있는 값이다).

  200 동시 요청으로 검증:

  | XFF 패턴 | 결과 |
  |---|---|
  | 고정 | 145 통과 / **55 차단** ✅ |
  | `spoofed, 진짜` (프록시 덧붙임 형태) | 155 통과 / **45 차단** ✅ 오른쪽 hop 집계 동작 |
  | 단일값 회전 | 200 통과 / 0 차단 ⚠️ |

  **배포 요구사항 — 실측으로 확인됨 (08-19)**: 엣지가 XFF를 **덧붙이거나 덮어써야** 한다. 그리고 **지금의 Next rewrite는 그렇게 하지 않는다.**

  개발 서버(`localhost:3000`)를 통해 XFF를 회전시키며 150 동시 요청을 보내니 **150건 전부 통과**했다. 즉 Next의 `rewrites()`는 클라이언트가 보낸 `X-Forwarded-For`를 **그대로 BE에 전달**한다. 현재 구조를 그대로 배포하면 비로그인 IP 한도는 헤더 위조로 우회된다.

  | 대응 | 효과 | 비용 |
  |---|---|---|
  | **리버스 프록시(nginx/ALB)를 앞에 둔다** | XFF를 엣지가 덮어써 우회가 닫힌다. API 트래픽이 Node 홉을 안 거쳐 P0-6의 성능 항목도 같이 해결된다 | 인프라 설정 1개 추가, 쿠키 도메인 정리 |
  | `TRUST_PROXY_HEADERS=false` | 헤더를 무시한다 | **쓰면 안 된다** — Next 경유 트래픽이 전부 Node 서버 IP 하나로 보여 모든 비로그인 사용자가 버킷 하나를 공유한다 |
  | 그대로 둔다 | 로그인 사용자 한도(`user_id`)는 정상 동작 | 비로그인 IP 한도는 사실상 없는 것 |

  **로그인 사용자 기준 한도는 이 문제와 무관하게 정상**이다(키가 헤더가 아니라 토큰에서 온다). 뚫리는 것은 비로그인 읽기 경로뿐이다.

  *(측정 주의: 처음에 "XFF 회전 300발이 전부 통과 = 우회 확정"이라고 봤는데, 그건 **순차 curl이라 프로세스 생성 오버헤드 때문에 애초에 한도 미만**이었다. 동시 요청으로 다시 재고서야 (b)만 실제 우회임이 드러났다.)*

- **한계 (의도적)**: 카운터가 프로세스 메모리에 있어 API를 2대로 늘리면 실효 한도가 2배가 된다. Redis로 옮기면 정확하지만 요청마다 왕복이 붙는다 — 단일 인스턴스인 지금은 그 값을 치르지 않는다. P1-4(다중 인스턴스)와 함께 다시 볼 것.

- **남은 조치**: **FE 청킹** — 보유 100종목 초과 사용자는 지금도 일부 종목의 시세가 비어 보인다. `limit` 필드가 생겼으니 FE가 100개씩 나눠 부르면 해결된다(별도 작업).
- **트레이드오프**: FE가 2초·5초 폴링을 여러 개 돌리므로 리밋이 너무 촘촘하면 정상 사용자가 429를 맞는다. 한 화면이 초당 2~3 요청을 내는 것을 감안해 IP당 20 req/s 이상에서 시작하고, 로그로 실측 후 조인다. 사무실·학교처럼 NAT 뒤 다수 사용자는 IP 기준이 불리하므로, 로그인 사용자는 user_id 기준으로 분리하는 편이 낫다.

#### P0-2. 의존성이 멎으면 요청이 30초 동안 살아 있다

- **현상**: 액세스 로그에서 `/auth/me`, `/market/data-settings`까지 30,004~30,126 ms를 기록했다. 30초는 `server.go`의 `WriteTimeout: 30 * time.Second`가 잘라 준 값이다.
- **근거**: 같은 시각 `market index store: upsert usd-krw: context deadline exceeded`가 반복 — MySQL과 외부 HTTP가 동시에 죽은 구간. 핸들러들은 `r.Context()`를 그대로 넘기며 **자체 타임아웃이 없다**.
- **완료 — 08-19. 그리고 원인은 문서에 적었던 것보다 나빴다.**

  Redis를 정지시켜 실측하니 `WriteTimeout` 30초가 **아무 역할도 하지 않았다**. 그 값은 응답을 **쓰기 시작한 뒤**부터 재기 때문에, 아무것도 쓰지 않고 멈춘 핸들러는 영원히 붙잡힌다.

  | Redis down | 이전 | 이후 |
  |---|---|---|
  | `/quotes/{symbol}` | **120초+ 무응답** (curl `--max-time`이 끊음) | **12.1초에 응답** |
  | `/quotes/{symbol}/terminal-snapshot` | **120초+ 무응답** | **12.1초에 응답** |
  | `/news/domestic`, `/scores/top`, `/earnings` | 200 | 200 (변화 없음) |
  | `/ready` | `degraded` + 200 | 동일 (설계대로) |

  FE가 2초마다 폴링하는 경로다 — Redis가 잠깐 흔들리면 연결과 goroutine이 그대로 쌓인다.

  1. `middleware.RequestTimeout` 신설, 기본 **10초**(`REQUEST_TIMEOUT_SECONDS`, 0이면 끔). `chimw.Timeout`으로 컨텍스트에 마감을 걸면 go-redis와 database/sql이 이를 존중해 멈춤이 에러로 바뀐다.
  2. **제외 경로** — 무조건 걸면 정상 기능이 깨진다: SSE(`/stream`, `/quote-stream`), 업로드(`/import/*`, `/media/images`), 그리고 **`/admin/*`**. 마지막은 `RecomputeScores`가 전 종목 점수를 **동기로** 재계산하기 때문이다(배치 트리거는 큐에 넣고 202로 곧장 반환하지만 이쪽은 아니다).
  3. MySQL DSN에 `timeout=5s&readTimeout=30s&writeTimeout=30s`를 **없을 때만** 채운다. go-sql-driver 기본값이 셋 다 무제한이라, 컨텍스트 마감이 없는 경로(폴러·배치 워커)는 여기서만 보호된다. 운영자가 DSN에 적어 둔 값은 덮어쓰지 않는다.

  단위 테스트 10개(제외 경로 목록, 마감 전달, SSE 비적용, 멈춘 핸들러 해제, DSN 병합·미덮어쓰기 등).
- **남은 사소한 것**: Redis 장애 시 응답이 `404`다(핸들러의 기존 `if !ok` 분기). 의미상 `503`이 맞지만 FE가 404를 다르게 다룰 수 있어 건드리지 않았다.

#### P0-3. Redis가 `maxmemory 0` + `noeviction`

- **현상**: 현재 사용량 **427 MB**, 키 23,719개(만료 설정 21,515개 → **약 2,200개는 TTL 없음**). `maxmemory_policy: noeviction`.
- **왜 위험한가**: 클라우드 인스턴스는 로컬의 15.5 GiB가 아니다. 상한이 없으면 Redis가 계속 자라 호스트를 OOM으로 몰거나, 메모리가 차는 순간 **쓰기가 전부 에러**가 된다. `AGENT_STATE.md`는 "기동 시 Redis 필수"라고 못박고 있어 파급이 크다.
- **완료 — 08-19**: compose의 redis에 `--maxmemory ${REDIS_MAXMEMORY:-1gb} --maxmemory-policy volatile-lru`. 적용 확인(`CONFIG GET` → `1073741824` / `volatile-lru`), 재기동 후 키 22,384개 보존 확인.

  **TTL 없는 키의 정체를 먼저 확인했고, 그 결과가 정책을 결정했다.** 전수 조사하니 `md:rev:<symbol>` **2,204개뿐**이었다(각 56 B, 합계 약 123 KB). 이건 `quotes_cache.go:69`의 `bumpRev`가 `INCR`로 올리는 **캐시 무효화 리비전 카운터**다 — 사라지면 값이 1부터 다시 세어져 낡은 캐시가 새 것으로 보인다. `allkeys-lru`는 이걸 버릴 수 있고 `volatile-lru`는 정확히 이것만 보호하므로, 후자가 이 코드베이스에 맞는 선택이다. 나머지 키는 전부 TTL이 있어 축출 대상으로 충분하다.

  기본값 1 gb는 현재 사용량 338 MB·피크 482 MB의 2배 이상이라 **지금은 아무것도 축출되지 않는다** — 상한을 두는 것이 목적이지 캐시를 줄이는 게 아니다. 클라우드에서는 `REDIS_MAXMEMORY`를 인스턴스 RAM의 60~70%로 넘긴다.
- **남은 조치**: compose에 `deploy.resources.limits.memory` 추가 (P2와 함께).

#### P0-4. 8개 프로세스가 각각 MySQL 커넥션 50개를 연다

- **현상**: `repository/mysql.go`의 기본값 `defaultMaxOpenConns = 50`. `api`와 워커 7개가 모두 `repository.NewMySQL`을 쓴다(`worker_bootstrap.go:45`). 최대 **8 × 50 = 400 커넥션**.
- **왜 위험한가**: MySQL 기본 `max_connections`는 151이다. 관리형 DB의 소형 인스턴스도 대개 150~300이다. 워커가 동시에 기동하는 재배포 순간 커넥션 고갈 → 전 프로세스 기동 실패.
- **조치 (1) 완료 — 08-19**: compose의 워커 7개에 `MYSQL_MAX_OPEN_CONNS=8` / `MYSQL_MAX_IDLE_CONNS=2` 지정. 배치 코드에 동시성 풀이 없고(순차 실행) 트랜잭션이 커넥션 1개를 점유하므로 8은 여유 있는 값이다. **최대 350 → 56**, API 50을 더해도 106으로 기본 한도 151 안에 들어온다. 7개 워커 전부 정상 기동 확인.
- **남은 조치**: `api`의 50은 **부하 테스트 전까지 건드리지 않는다.** `mysql.go` 주석대로 API는 폴러 십수 개 + SSE + 사용자 요청을 동시에 처리하므로, 근거 없이 낮추면 커넥션 대기로 직렬화돼 P0-2가 재발한다. DB 인스턴스의 `max_connections`를 실제 합계보다 여유 있게 잡는 쪽이 먼저다.

#### P0-5. 마이그레이션이 8개 프로세스에서 동시 실행된다

- **현상**: `api`와 워커 7개가 모두 기동 시 `repository.EnsureSchema`를 호출한다(`server.go:88`, `worker_bootstrap.go:49`).
- **구조**: 버전 테이블 없이 `MigrateXIfNeeded` 함수 **약 30개**가 테이블/컬럼 존재 여부를 마커로 삼아 각자 판단한다. `AGENT_STATE.md`도 `isSchemaRaceError` 무시를 명시하고 있다 — 레이스를 인정하고 삼키는 구조다.
- **왜 위험한가**: 순서 보장 없음, 롤백 없음, 락 없음. 배포마다 8개 프로세스가 같은 DDL을 경쟁 실행한다. 로컬에서 무사한 이유는 매번 스키마가 이미 최신이기 때문이며, **처음 마이그레이션이 실제로 도는 프로덕션 첫 배포가 가장 위험하다.**
- **단기 조치 완료 — 08-19**: 마이그레이터를 `api` 하나로 줄였다.
  1. `repository.VerifySchema` 신설 — **DDL을 실행하지 않고** 필요한 테이블이 보일 때까지 기다린다(3분, 3초 간격). 워커의 `BootstrapWorker`가 `EnsureSchema` 대신 이걸 부른다.
  2. compose의 워커 7개에 `depends_on: api: service_healthy` 추가. `/health` 응답은 `server.New()`가 `EnsureSchema`를 끝냈다는 뜻이므로 순서가 보장된다. 코드 쪽 대기와 이중으로 걸어 기동 순서를 보장하기 어려운 환경에서도 버틴다.
  3. `BootstrapWorker(ctx, migrationsDir)` → `BootstrapWorker(ctx)`. 안 쓰는 인자를 남기면 "워커가 마이그레이션한다"는 오해가 그대로 남는다 (호출부 7곳 + `path/filepath` import 정리).
  4. 로그 문구를 갈랐다 — 마이그레이터는 `mysql: schema ok (all required tables present)`, 워커는 `mysql: schema verified (read-only; api is the migrator)`. 처음엔 둘이 같은 문구를 찍어 로그만으로 구분이 안 됐다.

  **검증**: `docker compose down` 후 전체 기동 → `redis Healthy → api Healthy → 워커 7개 Started` 순서 확인, 워커 로그에 마이그레이션 문구 없음 확인.
- **남은 조치**: `AGENT_STATE.md` TODO의 **goose 도입**. 배포 파이프라인에서 마이그레이션을 별도 단계(init container / job)로 분리.
- **트레이드오프**: 워커 기동이 API에 종속돼 순서가 직렬화된다(healthcheck `start_period` 20초 + interval 30초). 배치 워커라 문제되지 않는다. goose 전환은 기존 30개 함수를 버전 파일로 옮기는 작업량이 있으나, `migrations/001~051`이 이미 번호로 정렬돼 있어 매핑은 기계적이다.

#### P0-6. FE가 배포 가능한 형태가 아니다

- **현상**: `fe/`에 Dockerfile 없음, compose에 서비스 없음, `next.config.ts`에 `output: "standalone"` 없음, 실행 스크립트는 `next dev -H 0.0.0.0`.
- **추가**: `next.config.ts`의 rewrite가 **모든 `/api/*`를 Node 프로세스를 통해 BE로 프록시**한다. 브라우저 폴링(2초 3종)과 SSE가 전부 이 홉을 거친다.
- **완료 — 08-19. 리버스 프록시(nginx) 채택.** 성능이 아니라 **보안**이 결정 근거였다 — Next rewrite가 클라이언트 XFF를 그대로 전달해 IP 레이트 리밋이 우회됐다(P0-1 참조).

  | 산출물 | 내용 |
  |---|---|
  | `fe/Dockerfile` | multi-stage + `output: "standalone"`. `.next/static`·`public`은 standalone에 포함되지 않아 따로 복사한다(빠뜨리면 CSS·JS가 404) |
  | `fe/next.config.ts` | `output: "standalone"` 추가. rewrite는 **개발용으로 남긴다** — 프로덕션에서는 nginx가 `/api/`를 가로채 타지 않는다 |
  | `deploy/nginx/nginx.conf` | 엣지. `/api/`·`/media/`·`/health`·`/ready` → api, 그 외 → fe |
  | `deploy/nginx/proxy_headers.conf` | 공통 프록시 헤더 |
  | `docker-compose.yml` | `fe`·`nginx` 서비스, **`profiles: ["prod"]`** |

  **기존 개발 흐름은 그대로다.** 프로파일로 분리해 `docker compose up -d`와 `be/scripts/restart.ps1`은 이 둘을 띄우지 않는다. 배포 토폴로지는 `docker compose --profile prod up -d --build`.

  **작업 중 만든 버그 하나와 그걸 잡은 방법.** 처음엔 공통 헤더를 `server` 블록에 한 번만 적었다. nginx의 `proxy_set_header`는 상속이 **merge가 아니라 replace**라, 각 `location`이 갖고 있던 `proxy_set_header Connection '';` 한 줄 때문에 `X-Forwarded-For`가 통째로 사라졌고 우회가 그대로 열려 있었다. `proxy_headers.conf`로 빼서 location마다 `include` 하도록 고쳤다.

  이걸 잡은 것은 **측정 방법을 고친 덕분**이다. 처음 200 동시 요청 테스트는 세 조건이 모두 "전부 통과"로 나와 아무것도 구분하지 못했는데, 경과 시간을 같이 재보니 **200건이 ~5초에 걸쳐 나가 허용치(60 burst + 30/s × 5s ≈ 210) 아래**였다. 한도를 임시로 1 rps/burst 5로 낮추자 신호가 분명해졌다.

  | 한도 1rps/burst5, 40 동시 | 결과 |
  |---|---|
  | 직접 `:8080` + XFF 회전 | 40 통과 / 0 차단 (우회 재현) |
  | **nginx `:8088` + XFF 회전** | **5 통과 / 35 차단** ✅ |
  | nginx `:8088` + XFF 고정 | 5 통과 / 35 차단 (동일 — 헤더를 무시한다는 뜻) |

  검증: FE 렌더 200(73 ms), API 3종 200, **SSE 즉시 스트리밍 + 70초 유지**(nginx 기본 60초 read timeout을 3600초로 올림, `proxy_buffering off`).

- **⚠️ 배포 시 필수**: 위 표의 첫 줄이 말해 주듯 **`api`의 8080 포트를 인터넷에 노출하면 안 된다.** 지금 compose는 로컬 테스트 편의로 `8080:8080`을 열어 두는데, 우회는 그 포트로 직접 붙을 때만 남는다. 클라우드에서는 그 포트 매핑을 지우고 nginx(80/443)만 노출한다.
- **트레이드오프**: 인프라 구성 요소가 하나 늘었고 쿠키 도메인을 nginx 기준으로 맞춰야 한다. 대신 API 트래픽이 Node 싱글 프로세스를 거치지 않게 됐다 — 2초 폴링 3종과 SSE가 전부 그 홉을 지나고 있었다.

---

### P1 — 출시 직후 (사용자가 체감하는 느림)

#### P1-1. `/news/us`가 매 요청 풀스캔한다 — 660~2,270 ms

- **근거**: `repository/news_repo.go:443`
  ```go
  func (r *NewsRepo) Count(ctx, market) (int, error) {
      query := `SELECT COUNT(1) FROM news_articles WHERE market = ?`
      if market == "us" {
          query += usTranslationReadySQL()   // " AND COALESCE(headline_ko, '') != ''"
      }
  ```
  `COALESCE(headline_ko,'') != ''`는 인덱스를 못 탄다. `market='us'` 18,726행을 전부 읽고 TEXT 컬럼에 함수를 적용한다. 같은 조건이 `List`(`:401`)에도 붙는다.
- **대조군**: `/news/domestic`은 행이 더 많은데도(59,238행) 13 ms — 이 술어가 없어서다.
- **캐시가 못 막는 이유**: `NewsStore`의 15초 리스트 캐시(`news_store.go:256`)는 **목록만** 덮는다. `meta.totalCount`를 채우는 `Count`는 핸들러(`handler/news.go:172`)에서 캐시 밖으로 직접 호출된다.
- **조치 (1) 완료 — 08-19**: `NewsStore.Count`에 목록과 같은 TTL·같은 무효화 지점을 쓰는 캐시를 넣었다. 여기에 더해 **만료 시 낡은 값을 그대로 주고 갱신은 배경으로** 돌린다(stale-while-revalidate). 그냥 캐시만 했으면 15초마다 한 요청이 1.6초를 뒤집어썼다.

  | 상태 | 이전 | 이후 |
  |---|---:|---:|
  | `/news/us` 워밍 | 660~2,270 ms | **2.5~7 ms** |
  | `/news/us` TTL 만료 직후 단일 요청 | ~1.6 s (카운트) | **60 ms** |
  | `/news/domestic` 워밍 (회귀 확인) | 13 ms | 2.4 ms |
  | `totalCount` 정확성 | — | 일치 확인 (us 18,782 / domestic 59,283) |

  **측정 중 진단이 한 번 바뀌었다.** 콜드 2.5초를 목록 재구성 탓으로 봤는데, `limit`을 바꿔 캐시 키를 강제로 비껴가며 재보니 `limit=25/50/100/200`이 13~272 ms로 깔끔히 비례했다 — 느린 건 목록이 아니라 카운트 쿼리 하나였다. 처음 잰 6.9초는 재기동 직후 백필과 겹친 값이었다.

- **남은 조치**:
  2. `headline_ko`에 대한 술어를 `translation_ready TINYINT` 생성 컬럼으로 바꾸고 `(market, translation_ready)` 인덱스 추가. **이것이 근본 해법** — 지금은 15초에 한 번 배경에서 1.6초를 여전히 쓴다.
  3. 목록 검색(`q`)의 `LIKE '%…%'` 4컬럼과 `JSON_CONTAINS(symbols_json, …)`도 같은 이유로 풀스캔이다(`news_repo.go:410`, `:415`) → FULLTEXT 인덱스 또는 별도 태그 테이블.
  4. ~~목록 캐시에 stampede 보호가 없다~~ → **완료 (08-19)**. 시세 쪽 `quoteRefreshCoordinator.doSync`를 그대로 재사용해 같은 목록 키의 동시 재구성을 하나로 묶었다(새 추상화를 만들지 않았다). 대기는 `quoteRefreshWait` 5초에서 끊겨 재구성이 길어져도 요청이 잡혀 있지 않고 스냅샷 폴백으로 내려간다.

     | 40 동시 요청 (TTL 만료 직후) | 이전 | 이후 |
     |---|---:|---:|
     | p50 | 2.004 s | **1.214 s** |
     | p95 | — | 1.361 s |
     | max | 6.602 s | **1.405 s** |
     | avg | 2.988 s | 1.195 s |

     분포가 0.88~1.41초로 좁아진 것이 "한 번만 만들고 나머지는 대기"의 신호다. 동시 응답 20개가 모두 같은 `totalCount`(18,846)를 돌려주는 것으로 정합성도 확인했다.
  5. **남은 개선 (미착수)**: 목록에도 카운트와 같은 stale-while-revalidate를 적용하면 대기 1.2초가 사라진다. 폴러가 ingest 시 캐시를 nil로 만드는 동작은 그대로 두면 되므로(그때는 낡은 값이 없어 동기 재구성) "새 기사가 바로 뜬다"는 보장은 유지된다. TTL 만료로 인한 대다수 재구성만 배경으로 밀린다.
- **트레이드오프**: (2)는 마이그레이션 + 기존 행 백필이 필요하고 MySQL 5.7에서는 생성 컬럼 인덱스 동작이 다르다. (3)은 스키마 변경 규모가 커서 출시 후로 미뤄도 된다. (4)는 hot path에 손을 대므로 신중해야 한다.

#### P1-2. 목록 엔드포인트의 심볼별 N+1

- **근거**: `service/quote_live_price.go:186` `EnrichRankingPrices`가 항목마다 `ResolveLiveQuotePrice`를 **직렬로** 호출한다. 그 안의 `resolveBestLivePrice`(`:242`)는 심볼 하나당 최대 7단계 폴백을 탄다:
  ```
  cachedTrades → cachedOrderBook → 1m forming bar → ranking last price
  → marketStore.EnsurePrice → directTossPrice(외부 HTTP!) → 1d forming bar
  ```
  `/quotes`의 `applyLivePrices`(`quotes_stocks.go:145`)도 같은 구조다.
- **관측 결과**: `/rankings` US 900~1,110 ms vs KR 150 ms. `/quotes` 콜드 4~5초(최대 20초).
- **특히 문제**: `directTossPrice`는 **사용자 요청 경로에서 외부 HTTP를 부른다.** `AGENT_STATE.md`의 "조회 경로에서 외부 HTTP 금지" 원칙과 충돌하며, P0-2의 30초 스톨과 직결된다.
- **`/rankings` 병렬화 완료 — 08-19. 우선순위를 재조정했다.**

  처음엔 `/quotes` 목록(콜드 29~46초)이 최악이라 봤지만, **`/quotes`는 사이드바 네비게이션에 없는 고아 라우트**였다. 실제로 쓰이는 것은 `/rankings`다 — 「실시간 차트」(`/market`)가 열자마자 `count=100`으로 부른다.

  건수에 정확히 비례하는 것이 직렬 처리의 지문이었다:

  | count | KR 이전 → 이후 | US 이전 → 이후 |
  |---|---|---|
  | 8 | 0.15s | 0.22s |
  | 25 | 0.29s | 0.49s |
  | 50 | 0.61s | 1.24s |
  | **100** (실제 화면) | **1.44s → 0.158s** | **2.36s → 0.168s** |

  9~14배 빨라졌고 건수 비례가 사라졌다. 동시도 8(`enrichWatchlistQuotes`와 같은 패턴), 가격 해석 로직은 **손대지 않아 표시 값이 동일**하다 — 랭킹의 `lastPrice`가 `/quotes/{symbol}`의 `currentPrice`와 일치하는 것을 확인했고(000660 1514000, 005930 249000, 009150 1376000), 반복 요청 결과도 결정적이다.

  **"병렬화하면 Toss 호출량이 늘지 않나"** — 늘지 않는다. 이 경로의 비용은 Toss가 아니라 **Redis 왕복**이었다: `count=100` 요청 하나가 Redis 명령 약 700회를 냈고(항목당 7회), 왕복 2 ms를 곱하면 1.4초로 실측치와 정확히 맞는다. 랭킹 상위 종목은 폴러가 데워 두므로 `EnsurePrice`가 대부분 캐시에 적중한다. 격리 측정으로 확인: **랭킹 요청 1회당 Toss `rate-limit exceeded` 증가 0건.**

- **남은 조치**: Redis 왕복 자체를 파이프라인/MGET으로 묶으면 항목당 7회를 몇 회로 줄일 수 있다(병렬화로 체감은 이미 해결).
- **`/quotes` 목록(29~46초)**: 고아 라우트라 우선순위가 낮다. 요청 타임아웃(10초)이 들어가 최소한 무한 대기는 없다. 라우트 정리는 별도 항목.

#### P1-3. `/quotes` 캐시 TTL 3초 vs 콜드 4~5초

- **현상**: `model.MarketDataListCacheSec = 3`. 콜드 미스가 4~5초 걸리므로 **캐시가 만료되는 순간마다 요청이 밀린다.** FE는 이 엔드포인트를 5초마다 폴링한다.
- **조치**: P1-2로 콜드 비용을 낮추는 것이 근본 해법. 병행 조치로 **singleflight**를 적용해 동시 미스가 업스트림을 한 번만 때리게 한다(`quoteRefresh.doSync`가 이미 유사 패턴을 쓰고 있어 재사용 가능).
- **트레이드오프**: TTL을 늘리면(3초 → 10초) 즉시 완화되지만 목록 가격이 낡는다. 폴링 주기가 5초인데 캐시가 10초면 절반은 같은 값을 받는다 — 시세 서비스로서 받아들일지 판단이 필요하다. singleflight는 트레이드오프 없이 이득이므로 **먼저 넣는다.**

#### P1-4. SSE가 in-process hub라 다중 인스턴스에서 깨진다

- **근거**: `service/quotes_marketdata.go:477` `s.quoteStreamHub.Subscribe(symbol)` — 프로세스 로컬 채널.
- **영향**: API를 2대 이상으로 늘리면 인스턴스 A의 폴러가 받은 틱이 인스턴스 B에 붙은 클라이언트에게 가지 않는다. **수평 확장이 불가능하다.**
- **조치**: `AGENT_STATE.md` TODO의 "Redis Pub/Sub + 다중 BE SSE fan-out". 폴러 리더 선출도 같은 TODO에 있다.
- **트레이드오프**: 출시 초기 트래픽이 단일 인스턴스로 감당된다면 **미뤄도 되는 항목**이다. 다만 "확장이 필요해진 시점"은 대개 급하게 오므로, 최소한 로드밸런서에 **sticky session**을 걸어 두면 임시로 버틸 수 있다. 이건 설정 한 줄이다.

#### P1-5. `/stocks/search` 210 ms · `/earnings/calendar` 179 KB

- `/stocks/search`: 빈 쿼리로 50건 뽑는 데 210 ms. `stocks` 테이블 17,000행 대상 정렬로 추정 — `repository/stock_search.go` 확인 후 인덱스 보강.
- `/earnings/calendar`: 응답 원본 **179 KB** (gzip 28 KB). 79~101 ms. 기간 축소 또는 페이지네이션 검토.
- **트레이드오프**: 실적 캘린더는 월 단위로 통째로 보는 화면이라 페이지네이션이 UX를 해칠 수 있다. gzip 후 28 KB면 당장 급하지 않다 — **P2로 미뤄도 되는 항목**.

---

### P2 — 여유 있을 때 (비용·운영 품질)

#### P2-0. 배경 폴러가 Toss 한도를 상시 초과하고 있다 (08-19 신규 발견)

- **현상**: **요청이 하나도 없는 상태**에서 `toss api rate-limit exceeded` 로그가 계속 쌓인다. API를 재시작해 활성 심볼 집합을 비운 직후에도 **30초간 417건**(≈14/s)이었다.
- **분포**: `/api/v1/trades` 33 · `/api/v1/orderbook` 26 · 그 외 소수 (최근 60건 기준). 즉 시세(`/prices`)가 아니라 **체결·호가**를 과도하게 부른다.
- **확인된 것**: 사용자 요청과 무관하다 — 재시작 후 무요청 30초에 417건, 랭킹 1회 호출 후 30초에는 오히려 277건(캐시가 데워져 감소). 이번에 넣은 랭킹 병렬화와도 무관하다(요청당 증가 0건).
- **왜 문제인가**:
  1. 실제 시세 갱신이 이 잡음과 한도를 다툰다.
  2. `market_poller.go` 주석은 이 압력을 이미 줄였다고 적고 있는데(“This used to be the largest source of MARKET_DATA rate-limit pressure”), 실측은 여전히 상시 초과다 — 문서와 현실이 어긋나 있다.
  3. P2-1의 idle CPU 22~26%와 뿌리가 같을 가능성이 높다.
- **완료 — 08-19. 근본 원인은 "목록을 보면 그 종목 전부가 '보는 중'이 된다"였다.**

  `poller.Touch()`는 "지금 누가 이 종목을 보고 있다"는 표시이고, 폴러는 그 표시가 붙은 종목을 1초마다 **체결·호가까지 통째로** 다시 받는다(대상 40개면 초당 80 호출, Toss 한도는 각 15/s). 그런데 `enrichSummaryLivePrice`가 **목록 항목마다** Touch를 불렀다 — 관심 종목 목록 한 번이 그 종목 전부를 활성으로 만들었고, 목록은 체결·호가를 그리지도 않는다. `market_poller.go`의 주석은 목록에 이 처리를 하지 않는다고 적고 있어 **주석과 코드가 어긋나 있었다.**

  7분 관측이 결정적이었다 — 분당 598·414·686·661·585·469·**126**. 마지막 분의 급락이 `marketActiveTTL`(5분) 만료와 정확히 일치했다. 즉 사용자가 계속 보고 있으면 영원히 포화된다.

  | 무요청 30초 | 이전 | 이후 |
  |---|---:|---:|
  | Toss `rate-limit exceeded` | 230~253 | **70** |
  | (수정 직후, 캐시 식은 상태) | — | 13 |

  조치 셋:
  1. **목록 경로에서 `Touch()` 제거** — 활성 집합이 실제로 보고 있는 종목만 남는다.
  2. **폴러 fast 틱에 세션 게이트** (`filterLiveSessionSymbols`) — 장이 닫힌 종목은 체결·호가를 갱신하지 않는다. `shouldLivePollSymbol`은 요청 경로 11곳에서 다른 의미로 쓰여 건드리지 않고, 폴러에만 걸었다. *위 수치는 양 시장이 열린 시각에 잰 것이라 이 게이트는 아직 기여하지 않았다 — 장 마감 시간대에 추가로 줄어든다.*
  3. **목록 경로의 종목별 업스트림 제거** (`withListMode`) — 1·2를 넣자 폴러가 더 이상 데워 주지 않아 목록이 매 요청 업스트림을 치렀다(**limit=40에서 58초**). 배치로 받은 가격과 캐시·DB만 쓰도록 막았다.

  | `/quotes` | 조치 전 | 1·2 직후 (회귀) | 3 이후 |
  |---|---:|---:|---:|
  | limit=20 | — | 6.4 s | **0.73 s** |
  | limit=40 | — | **58 s** | **1.65 s** |
  | limit=100 (워밍) | 29~46 s | 90 s | **0.004 s** |

  **트레이드오프**: 아무도 보고 있지 않은 종목은 목록에서 "최신 체결가"가 아니라 "배치 마지막가"로 보인다(급변동 시 한두 틱). 상세·차트·SSE는 그대로다 — 검증에서 목록 가격이 상세와 정확히 일치했고 SSE도 정상이었다.

#### P2-1. API가 무부하에서 CPU 22~26%를 쓴다

- **원인**: 1초 fast tick(`marketFastPollInterval`)을 중심으로 한 상시 폴러 십수 개.
- **조치 후보**:
  - 장 마감·주말에 fast tick을 늦춘다 (`calendar` 패키지가 이미 세션을 안다).
  - 활성 심볼이 0개일 때 fast tick을 건너뛴다 (지금도 `capActiveTargets`로 좁히지만 티커 자체는 계속 돈다).
- **트레이드오프**: 클라우드 vCPU 요금에 직결된다(상시 0.25 vCPU). 반대로 폴링을 늦추면 장 시작 직후 첫 사용자가 낡은 값을 본다. **장 시간 외에만 늦추는 것이 위험 없는 절충**이다.

#### P2-2. naver 뉴스 API 월 90,999건

- 사용자 수와 무관한 고정 비용. 일 한도 24,000 대비 여유는 있으나(일 약 3,000), 폴링 주기 2분 × 소스 5종 × 키워드 수의 곱이다.
- **조치**: 키워드별 히트율을 측정해 실제로 기사를 만들지 못하는 키워드를 정리. 관리자 화면에 `admin/news/stats`가 이미 있다.
- **트레이드오프**: 키워드를 줄이면 커버리지가 준다. 무료 한도 안이면 **비용 문제는 아니고 정리 품질 문제**다.

#### P2-3. 관측성이 없다

- 구조화 로그 없음(`log.Printf` 한 줄), 메트릭 없음, 트레이싱 없음, **상태 코드조차 로그에 없다**.
- **조치 (1) 완료 — 08-19**: 액세스 로그를 `<reqID> METHOD PATH STATUS SIZE DURATION`으로 확장.
  ```
  70169b1bc62b/2bQpuOBFiW-000015 GET /api/v1/news/us 200 93.4kB 6.885007753s
  ```
  - 상태 코드가 생겨 **에러율을 셀 수 있다** (404가 200과 구분되는 것을 실측 확인).
  - 크기는 **on-the-wire 값**이라 gzip 동작을 로그만으로 검증할 수 있다 (같은 요청 28.3kB gzip / 95.5kB 평문 확인).
  - 요청 ID는 `Recoverer`의 패닉 로그와 같은 값이라 스택 트레이스를 요청에 붙일 수 있다.
  - 쿼리 문자열은 **일부러 남기지 않는다** — `q=`가 사용자가 친 검색어라 그대로 로그에 쌓인다.

  **주의해서 처리한 부분**: ResponseWriter를 평범한 struct로 감싸면 SSE가 죽는다. 스트리밍 핸들러는 `w.(http.Flusher)`와 `http.NewResponseController(w).SetWriteDeadline`에 의존하는데, 후자가 조용히 실패하면 `WriteTimeout` 30초가 SSE 연결을 끊는다. chi의 `WrapResponseWriter`가 `Unwrap()`을 구현하는 것을 소스에서 확인하고 채택했고, **SSE가 45초까지 유지되는 것을 실측으로 검증**했다(30초에 끊기지 않음).

  덤으로 `Recoverer`가 `http.ErrAbortHandler`를 다시 패닉시키도록 했다 — net/http가 클라이언트 연결 끊김에 쓰는 값이라, 그대로 두면 SSE 사용자가 탭을 닫을 때마다 스택 트레이스가 쌓여 정상 종료가 장애처럼 보인다.
- **남은 조치**: `/metrics` Prometheus 엔드포인트.

#### P2-4. CI가 없다

- 테스트 파일 151개가 있는데 `.github/workflows`가 비어 있다. 출시 후 회귀를 잡을 자동 방어선이 없다.
- **조치**: `go test ./...` + `go vet ./...` + `next build` + `npm run lint`가 도는 워크플로 하나.
- **주의 (08-19 발견)**: **`gofmt` 게이트는 처음부터 넣으면 안 된다.** 이 코드베이스는 gofmt를 거친 적이 없다 — `internal`·`cmd` 아래 **466개 파일**이 미준수다. 원인은 줄바꿈이 아니라 정렬로, `var (...)` 블록 등에서 탭이 아니라 **공백으로 정렬**되어 있다(HEAD 커밋 상태에서도 동일하며, 작업 중인 변경분 때문이 아니다).
  선택지는 둘이다: (a) `gofmt -w ./...`를 **독립 커밋 하나**로 먼저 넣고 게이트를 켠다 — 466 파일 diff가 한 번 생기지만 이후 깨끗하다. (b) 게이트를 빼고 신규 파일만 관리한다. 작업 중인 브랜치가 크면 (a)는 충돌을 부르므로 머지 이후로 미루는 편이 낫다.

#### P2-5. 리포지토리 위생

- ~~루트에 `tmp-*.json` 11개가 커밋 트리에 남아 있다.~~ **정정**: 13개 모두 untracked였다(커밋된 적 없음). 실수로 커밋되지 않도록 `.gitignore`에 `/tmp-*.json` 추가 — **완료 (08-19)**. 파일 자체는 사용자 소유라 남겨 둔다.
- `Recoverer`가 패닉 시 전체 스택을 로그에 남긴다 — 응답으로는 안 나가므로 안전하나, 프로덕션 로그 수집기에 스택이 그대로 쌓인다.

---

## 5. 잘 되어 있는 부분 (건드리지 말 것)

점검 결과 **이미 옳게 되어 있어 리팩토링 대상이 아닌 것들**:

- **배치 워커 분리**: 잡별 독립 컨테이너, 단일 이미지 + `command` 오버라이드. idle 6~9 MB. 교과서적이다.
- **LLM 지출 통제**: 진입점이 예약 배치와 관리자 수동뿐. 조회는 저장분만 읽는다. 실측 월 2,193콜로 통제되고 있다.
- **apibudget**: 15개 provider의 일·분·토큰 한도를 Redis 공유 카운터로 강제. 다중 인스턴스에서도 정확하다.
- **gzip 압축**: `Compress(5)`가 SSE와 이미지를 올바르게 비켜간다. 뉴스 98 KB → 29 KB.
- **인덱스 설계**: `050_perf_indexes.sql`의 `(trade_date, score DESC, peer_pct DESC)`는 정렬을 인덱스 순서에 정확히 맞춰 2.10초 → 0.036초를 만들었다. 주석에 근거와 MySQL 5.7 주의사항까지 적혀 있다.
- **FE 가시성 가드**: `isPageVisible()`로 백그라운드 탭 폴링 차단.
- **쿠키 보안**: `HttpOnly` + `Secure`(env) + `SameSite=Lax`.
- **graceful shutdown**: HTTP 드레인 → 백그라운드 취소 → 임포트 잡 대기 → 풀 종료. 순서가 정확하다.

---

## 6. 배포 전 체크리스트

### 코드 (위 항목 순서대로)
- [x] **P2-3 액세스 로그에 상태 코드·크기·요청 ID** — 08-19 (SSE 45초 유지 실측 검증)
- [x] **P0-1 부분: `/quotes/batch` 파싱 상한 + `limit` 필드 노출** — 08-19
- [x] **P1-1 `/news/us` Count 캐시 + 배경 갱신** — 08-19 (워밍 660~2,270 ms → 2.5 ms)
- [x] **P0-4 워커 커넥션 풀 축소** — 08-19 (최대 350 → 56)
- [x] **P2-5 `.gitignore`에 `/tmp-*.json`** — 08-19
- [x] **P0-1 부분: 심볼 목록 엔드포인트 전수 상한** — 08-19 (`/stocks`·`/market-indicators/prices` 무제한 → 100)
- [x] **P0-3 Redis `maxmemory` + `volatile-lru`** — 08-19
- [x] **P0-5 워커에서 `EnsureSchema` 제거 + 기동 순서** — 08-19 (전체 재기동으로 순서 검증)
- [x] ~~P0-1 `limit` 상한 강제~~ — 이미 되어 있었음 (정정)
- [x] **P0-1 레이트 리밋 (user_id / IP 혼합) + XFF 신뢰 경계** — 08-19
- [x] **P1-1 뉴스 목록 캐시 stampede 보호 (singleflight)** — 08-19 (40동시 max 6.60s → 1.41s)
- [x] **P2-4 CI 워크플로** — 08-19 (go build/vet/test -race + tsc + next build; lint·gofmt는 기존 부채라 비차단)
- [x] **P0-6 FE Dockerfile + nginx 리버스 프록시** — 08-19 (XFF 우회 차단 실증, SSE 70초 유지 확인)
- [x] **P0-2 요청 타임아웃(10초) + MySQL DSN 타임아웃** — 08-19 (Redis 장애 시 120초+ → 12초)
- [x] **P1-2 `/rankings` 병렬화** — 08-19 (count=100 KR 1.44s→0.158s, US 2.36s→0.168s)
- [ ] **배포 시: `api`의 `8080:8080` 포트 매핑 제거** — nginx만 노출. 남은 우회 경로가 그 포트다
- [x] **P2-0 배경 폴러의 상시 Toss 한도 초과** — 08-19 (무요청 30초 230~253 → 70)
- [x] **`/quotes` 고아 라우트 + 미사용 코드 정리** — 08-19
- [x] **뉴스 목록 stale-while-revalidate** — 08-19 (40동시 p50 1.21s → 0.003s)
- [x] **컨테이너 리소스 리밋** — 08-19 (12개 전부, docker inspect로 적용 확인)
- [x] **P2-3 /metrics** — 08-19 (MySQL 풀 대기/goroutine/상태코드)
- [ ] FE 미사용 판정이 갈리는 BE 엔드포인트 5종 (아래 「라우트 정리」 참조)

---

## 7. 라우트·미사용 코드 정리 (08-19)

### 제거함

| 대상 | 근거 |
|---|---|
| `/quotes` 페이지 | 사이드바에 없음. `/market`이 같은 역할 |
| `QuotesSymbolList`, `useQuotesList` | `/quotes` 전용 (`/market`은 `Footer`만 씀) |
| `QuotesDetailPanel`, `MarketIndicesBar`, `MarketIndicesBarLazy` | 사용처 **0** |
| `GET /api/v1/insights/diagnostics` + 핸들러·서비스·모델 | FE 참조 0, 문서 언급 0 |

유입 링크 4곳은 `/market`으로 돌렸다(상세 화면 「돌아가기」, `loginNext` 기본값, 헤더 검색 분기).

**`GET /api/v1/quotes`는 남겼다** — 관심 종목 화면이 `filter=watchlist`로 쓴다. 고아인 것은 페이지지 엔드포인트가 아니다.

### 남긴 것 (고아가 아님)

- **`/backtest`·`/portfolio`·`/risk`·`/transactions`·`/onboarding/link-broker`** — 2026-08-20 **삭제**. 미들웨어가 `/`로 돌린다. 시세용 Toss API는 유지 (`docs/FE_ACCOUNT_FEATURES.md`).
- **리다이렉트 스텁**(`/dashboard`, `/calendar`, `/financials`, `/earnings`, `/news`, `/ai-analysis`, `/disclosures`) — 옛 북마크·외부 링크를 살리는 것이 존재 이유다. 5줄짜리라 유지 비용이 없고 지우면 그 링크가 404가 된다.
- **`/health`, `/ready`, `/media/*`** — 인프라·이미지 서빙.

### 판단이 필요한 BE 엔드포인트 5종

FE 참조는 0이지만 **문서에 기재**되어 있어 임의로 지우지 않았다:

| 엔드포인트 | 문서 맥락 |
|---|---|
| `GET /health/api-quotas` | 운영 점검용. `AGENT_STATE`가 검증 절차에서 사용 |
| `GET /market-calendar/{KR,US}` | 「장 상태 바」로 제안됐다가 **채택 안 됨**(AGENT_STATE:220) |
| `GET /market-indicators/prices` | 뉴스 위젯 목업이 "기존 API"로 참조 — **예정된 사용처** |
| `GET /rankings/history` | v0.3.3 릴리스 노트에 신설 기록 |
| `POST /watchlist/bulk` | FE 참조 0 |
*(위 다섯 줄은 완료된 항목의 오래된 사본이었다 — P0-6·P1-2·P1-3·뉴스 singleflight는 08-19에 끝났고, 남은 것은 아래 하나뿐이다.)*

- [ ] P1-1 나머지: `translation_ready` 생성 컬럼 + 인덱스 (근본 해법) — **스키마 변경이라 이번 범위 밖**

### 인프라
- [ ] MySQL을 관리형 DB로 이전 (`host.docker.internal` 제거)
- [ ] `max_connections`를 프로세스별 풀 합계 + 여유로 설정
- [x] 모든 컨테이너에 CPU·메모리 리밋 — 08-19
- [ ] `quant_port_media` 볼륨의 백업 정책 (**사용자 업로드 이미지 — 유실 시 복구 불가**)
- [ ] `AUTH_COOKIE_SECURE=true`, `CORS_ALLOWED_ORIGINS`를 실 도메인으로
- [ ] `JWT_SECRET` · `ENCRYPTION_MASTER_KEY`를 시크릿 매니저로 (43개 env 중 이 둘이 가장 민감)
- [ ] 로드밸런서 sticky session (P1-4 임시 대응)

### 검증
- [ ] 부하 테스트로 `MYSQL_MAX_OPEN_CONNS` 결정 (추측 금지)
- [ ] 첫 배포를 **빈 DB에** 리허설 — P0-5의 마이그레이션이 실제로 도는 유일한 시나리오
- [x] Redis를 죽인 상태로 `/ready`가 `degraded`(200)를 내는지 확인 — 08-19 완료

---

## 부록 A. 측정 재현 방법

**A-1. 엔드포인트 응답 시간**
```bash
curl -s -m 20 -o /dev/null -w "%{time_total}s %{size_download}\n" "http://localhost:8080/api/v1/news/us"
```

**A-2. 액세스 로그 집계** — `middleware/logging.go`가 `METHOD PATH DURATION`을 찍으므로 컨테이너 로그에서 추출해 집계한다.
```bash
docker logs quant-port-api 2>&1 | grep -E "(GET|POST) /api/v1"
```

**A-3. 인프라 상태**
```bash
docker stats --no-stream
```
```bash
docker exec quant-port-redis redis-cli INFO memory
```

**A-4. 페이지별 런타임 API (미완 — 후속 작업)**
정적 import 추적은 상위 집합만 준다. 정확한 수치는 FE 실행 후 브라우저 네트워크 패널로 페이지별 요청 수·순서·워터폴을 잡아야 한다. 이때 **브라우저 패널이 화면에 표시되어 있어야** `isPageVisible()` 가드에 막히지 않는다.

---

## 부록 B. 참고 문서

- `docs/AGENT_STATE.md` — 저장 정책, 읽기 체인, 개발 원칙, TODO
- `docs/MARKET_INDICES_BATCH.md` — 지수 배치 설계
- `be/docs/api-quotas.md` — 외부 API 한도
- `migrations/050_perf_indexes.sql` — 인덱스 튜닝 근거와 실측치가 주석에 있다. **이 문서를 새 인덱스를 넣을 때의 서식 표준으로 삼을 것.**
