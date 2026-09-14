# External API quotas

백엔드는 외부 데이터 API를 **직접 사용자 요청 경로에서 호출하지 않고**, Redis 캐시 + 백그라운드 갱신(`Ensure*`, poller)으로 제공합니다.  
추가로 `internal/marketdata/apibudget` 패키지가 Redis에 호출량을 기록하고, 한도에 가까워지면 **사전 차단**합니다.

## 모니터링

| 항목 | 값 |
|------|-----|
| HTTP | `GET /health/api-quotas` |
| Redis 키 prefix | `apibudget:v1:{provider}:` |
| 월간 카운터 | `apibudget:v1:{provider}:count:month:YYYY-MM` |
| 일간 카운터 | `apibudget:v1:{provider}:count:day:YYYY-MM-DD` |
| 분당 카운터 | `apibudget:v1:{provider}:count:min:YYYY-MM-DDTHH:MM` |
| 마지막 호출 | `apibudget:v1:{provider}:last` |
| 차단 만료 | `apibudget:v1:{provider}:blocked_until` |

카운터 타임존은 KST(UTC+9)입니다.

## 제공자별 한도 (공식 → 적용값)

공식 한도보다 **여유(headroom)** 를 두고 `DefaultLimits`에 설정합니다. 코드: `be/internal/marketdata/apibudget/providers.go`.

| Provider | 용도 | 공식 한도 (참고) | 적용 한도 | 최소 갱신 간격 |
|----------|------|------------------|-----------|----------------|
| **goldprice** | 금 현물 (`/quotes/indices` gold) | Free: **1,000/월**, 30 req/min ([goldprice.dev](https://goldprice.dev)) | 900/월, 20/min | **90분** |
| **fred** | 나스닥·S&P·다우·VIX·미 국채 10년 등 FRED 시리즈 | **120 req/min** (API key), 일일 제한은 계정별 | 5,000/일, 100/min | **3분** (batch 6 series = 1 Record) |
| **finnhub** | ETF 시세(QQQ, SOXX), 실적 캘린더 | Free: **60/min** ([finnhub.io](https://finnhub.io/docs/api/rate-limit)) | 50/min | 5분 |
| **finnhub_migrate** | 배치 ETP 카탈로그 (`/stock/symbol?exchange=US`, 1회) | 동일 API 키 | 5/min | 30초 |
| **finnhub_news** | 미국 **시장 전체** 뉴스 (`/news`) | 별도 API 키, free **60/min** | 50/min | 2분 |
| **finnhub_news_company** | 미국 **종목별** 뉴스 (`/company-news`) | 동일 API 키 | 30/min | **2초** |
| **finnhub_backfill** | 해외 과거 실적 서프라이즈 **일회성** 백필 (`/stock/earnings`, 종목당 1호출) | 동일 API 키, free **60/min** | 8000/일, 45/min | **1.5초** |

| **krx** | 국내 ETF/ETN 카탈로그 (`etf_bydd_trd`, `etn_bydd_trd`) | Open API 일일 한도 | 500/일, 10/min | **2초** |
| **upbit** | BTC/KRW, 스파크라인 | Exchange **10 req/sec**, Quotation **600/min/IP** ([Upbit](https://docs.upbit.com/docs/user-request-guide)) | 10/min | 2분 |
| **dart** | 공시·corp code | **20,000/일** ([Open DART](https://opendart.fss.or.kr/)) | 18,000/일, 60/min | 1초 |
| **azure_translate** | 해외 뉴스 제목 EN→KO | **월 200만 자** (Azure F0 무료 등급, 일일 한도 없음) | 월 2,000,000**자**, 400/min | — |
| **edgar** | SEC XBRL 재무 (조회 경로) | **10 req/sec** ([SEC fair access](https://www.sec.gov/os/webmaster-faq#developers)) | 8/min | 150ms |
| **edgar_bulk** | 내부자 거래 Form 4 적재 배치 (worker-filings) + 실적 발표일 8-K 일회성 백필 | 동일 (같은 User-Agent) | 240/min | **300ms** |
| **fnguide** | 국내 실적발표 캘린더 (잠정실적 발표일 + 컨센서스·발표치) | **공개 한도 없음** — 공개 API가 아니라 Company Guide 화면의 내부 JSON | 60/일, 2/min | **30초** |

### edgar / edgar_bulk — 같은 소스를 두 버킷으로 나눈 이유

SEC가 공표한 한도는 **10 req/sec**인데 우리 적용값은 오랫동안 8/min이었다. 그 값은
**조회 요청이 곧 EDGAR 호출이던 시절**의 방어선이다 — 내부자 거래 탭이 종목당
4호출(공시 목록 1 + Form 4 XML 3)을 요청 시점에 냈고, 그래서 동시 접속 두 명이면
두 번째 사용자가 빈 화면을 받았다. 상한을 올려 푸는 문제가 아니라, **적재를 배치로
옮겨야 하는 문제**였다.

이제 내부자 거래 적재는 `worker-filings`가 하고 조회는 `insider_transactions` 표만
읽는다. 남은 온디맨드 EDGAR 경로(이중상장 종목의 재무 스냅샷)는 `edgar` 8/min을
그대로 쓰고, 배치는 `edgar_bulk`에서 센다. 나눈 것은 한도가 아니라 **누가 언제
부르는지**다 — `finnhub` / `finnhub_earnings`가 같은 이유로 먼저 갈라졌다.

### finnhub_news_company — 같은 간격이 한쪽에는 맞고 다른 쪽에는 치명적이었다

`finnhub_news`의 2분 간격은 **시장 전체 뉴스 한 갈래**를 위한 값이다. 같은 버킷에
`/company-news`가 들어 있으면 한 회차에 종목 32개를 도는 잡은 **첫 종목 뒤 31개가
전부 그 간격에 걸린다.**

실측(2026-09-13): 한 틱마다 `finnhub_news min interval 2m0s not elapsed` 가 31줄씩
쌓였고, 종목 큐 한 바퀴가 8분이 아니라 **약 7시간**이었다. 사용자가 「해외 뉴스를
못 가져온다」고 지적한 원인의 절반이 이것이다(나머지 절반은 주말 틱 게이트 —
`weekendTickGate` 머리글).

2초 간격은 실제 상한(60/min)의 절반이고, 호출부가 그만큼 **스스로 잔다**
(`CompanyNewsPace`). 32종목 × 2초 = 64초라 8분 주기 안에 끝난다.

배치 페이스 300ms는 약 3.3 req/sec으로 공표 한도의 3분의 1이다. 상한 240/min을
그 위에 둔 것은 안전판이라서다: 두 값이 같으면 간격을 지켜도 분당 한도에 걸릴 수
있고, `Check` 실패는 그 종목을 통째로 건너뛰게 만든다.

실제 호출량은 **증분**이라 훨씬 적다. 이미 저장한 accession은 다시 내려받지 않으므로
평시 종목당 1~2호출이다(실측: 같은 8종목 재실행이 54초 → 9초, 신규 0행).

### finnhub_backfill — 무료 티어가 과거를 30일만 준다

`/calendar/earnings`는 **과거 30일까지만** 응답한다. 오류가 아니라 **HTTP 200에 빈
배열**이라 더 고약하다 — 호출하는 쪽에서는 "그날 발표가 없었다"와 구분되지 않는다.
2026-08-21 실측:

| 창 | 행수 |
|---|---|
| 2026-08-10 ~ 08-16 | 1,246 |
| 2026-07-27 ~ 08-02 | 955 |
| 2026-07-20 ~ 07-26 | 370 (07-22부터만 — **경계**) |
| 2026-07-13 ~ 07-19 | **0** |
| 2025-08-04 ~ 08-10 | **0** |
| 2024-08-05 ~ 08-11 | **0** |

그래서 해외 실적 이력은 배치가 돌기 시작한 뒤로만 쌓이고, 「꾸준히 기대를 넘는
기업」 카드는 최소 표본 4분기를 넘는 종목이 **0개**였다(국내는 FnGuide가 2년치를
줘서 403종목).

대안은 `/stock/earnings`다. 종목당 **정확히 4분기**의 EPS 컨센서스·실적을 주고
`limit` 파라미터는 무시된다(8·16을 줘도 4행). 발표일은 안 주고 회계기간 종료일만
주므로 결과는 `earnings_events`가 아니라 `earnings_surprise_history`로 간다
(migrations/055). 매출 컨센서스(`/stock/revenue-estimate`)는 무료 티어에서 **403**.

- **일회성이다.** `cmd/backfill-earnings-surprises`. 정기 잡으로 두지 않는 이유는
  새 분기를 캘린더 배치가 **발표일과 함께** 이미 넣기 때문이다 — 매일 돌리면
  3,000호출을 써서 같은 4분기를 다시 받는다.
- 버킷을 가른 이유가 `finnhub_earnings`와 **반대 방향**이다. 캘린더 배치의 20초
  간격은 하루 수십 호출에는 맞지만, 3,000종목을 도는 이 잡에는 17시간이 된다.
- 1.5초 = 분당 40회로 공표 한도 60/min의 **3분의 2**. 상한 45를 정상 속도 위에 둔
  것은 `edgar_bulk`와 같은 안전판이다.
- 중단해도 안전하다. 기본 `-resume`이 이미 이력이 있는 종목을 건너뛴다.

### edgar 8-K — 실적 발표일의 유일한 출처

Finnhub `/stock/earnings`는 수치만 주고 **발표일을 안 준다**(주는 건 달력 분기로
반올림한 기간말). 미국 상장사는 실적을 **8-K Item 2.02**로 공시하므로
`data.sec.gov/submissions/CIK{...}.json`이 그 제출일과 접수 시각을 준다 —
종목당 1호출, `edgar_bulk` 버킷으로 3,000종목 **약 15분**.

`acceptanceDateTime`은 SEC **접수** 시각이라 보도자료보다 몇 시간 늦다(실측 WMT는
장전 발표인데 10:59 ET 접수). 그래도 쓰는 이유는 반응 계산이 **장전과 장중을
똑같이 다루기** 때문이다 — 숫자가 갈리는 것은 장후뿐이고, 장후는 접수가 20시
이후로 뚜렷이 분리된다(AAPL 20:30 · NVDA 20:21 vs WMT 10:59 · CVX 10:16).

### fnguide — 한도를 우리가 정한 이유

FnGuide는 이 데이터에 공개 API를 제공하지 않는다. 우리가 읽는 것은 Company Guide
캘린더 화면(`/Calendar/CompCalendar`)이 스스로 호출하는
`GET /Calendar/getListPreliminary?dt=YYYYMM`이다. 따라서 **게시된 rate limit이 없고,
그래서 오히려 우리가 낮게 잡는다.**

- 정상 운영은 **월당 1호출**이라 종목이 늘어도 호출은 늘지 않는다 — 한 응답이 그 달
  시장 전체를 담는다(2026-08 실측 190건). 시즌(1–2·4–5·7–8·10–11월)에는 당월을
  10–20시 2시간 간격으로 다시 받고 20시만 전월을 겹쳐 **하루 7호출**, 비시즌은
  18:30에 당월+전월 **2호출**. 상한 60/일이라 시즌에도 여유가 있다.
- 적용 상한 60/일은 관리자 백필(월 단위 2년치 ≈ 24회)이 한 번에 돌 수 있는 여유다.
- **킬 스위치**: `FNGUIDE_ENABLED=false`. 약관 관계와 엔드포인트 변경 위험을 감수한
  선택이므로, 코드 배포 없이 되돌릴 길을 남겨 두었다. 끄면 국내 실적 화면은 DART
  공시 이력으로 되돌아가고 화면 자체는 계속 뜬다.
- 이력은 약 2년 롤링(202408 있음 / 202308 없음). **미래 달은 빈 배열** — 국내에는
  실적발표 예정일 소스가 없어, 예정일은 「작년 동기 → 추정」으로 만든다.

**Toss Open API**는 키당 별도 rate-limit 그룹(`MARKET_DATA` 등)과 429 백오프로 처리합니다. PRIMARY가 한 그룹에서 소진되면 그 그룹만 SECONDARY로 넘깁니다. → [`README.md`](./README.md#rate-limit-그룹-spec-tags-참고)

## Toss 호출 설계 — 「price API 위주」의 검토 (2026-09-11)

사용자 제안: **「가성비가 가장 좋은 price API 외에는 최대한 쓰지 않는 것이 한도를
막는 최적 설계」**. 실측해 보니 방향은 맞지만 **한 곳에서 전제가 어긋난다.**

### 어긋나는 전제 — 한도는 전역이 아니라 **그룹별**이다

`/prices`·`/orderbook`·`/trades`·`/price-limits`는 **`MARKET_DATA` 한 버킷을 공유**한다
(응답 헤더 실측 `limit=15`). 즉 price API를 많이 쓰는 것은 다른 API를 아끼는 일이
아니고, **같은 창 안에서 orderbook·trades와 다투는** 일이다. 2026-09-11에 목록 화면
한 번이 429를 90건 만든 사건이 정확히 그 모양이었다 — 범인은 `/prices`가 아니라 같은
버킷의 `/orderbook`·`/trades`였고, 고친 것은 「price API를 더 쓰자」가 아니라
**그 버킷에서 쓸데없는 호출을 뺀 것**이다.

그래서 목표를 다시 쓰면 이렇게 된다:

> price API만 쓰자(X) → **버킷마다 그 버킷이 꼭 해야 하는 일만 남기자**(O)

### 실측 그룹별 현황

| 그룹 | 관측 한도 | 엔드포인트 | 하루 호출 (2026-09-11 실측·추정) | 판단 |
|------|-----------|-----------|-----------------------------|------|
| `MARKET_DATA` | **15/s** | `/prices` | 장중 약 1/s (200종목/호출) | **유지** — 종목당 비용이 가장 싸다 |
| `MARKET_DATA` | (공유) | `/orderbook` · `/trades` | 웹소켓이 주 경로(400슬롯), REST는 콜드 스타트만 | **거의 이전 완료** |
| `MARKET_DATA_CHART` | 별도 | `/candles` | 약 13,000 (종가 구멍 메우기) | **대체 불가** — 아래 |
| `STOCK` | **5/s** | `/stocks/{symbol}/warnings` | 17,900 → **14,500** (404 3,388 제외) | 가장 빡빡한 버킷 |
| `STOCK` | (공유) | `/stocks` | 소량 (200종목/호출) | 유지 |
| `STOCK_ALL` | 별도 | `/stocks/all` | **0 — 안 쓰고 있다** | **도입 권장**, 아래 |
| `MARKET_INFO` | 별도 | `/exchange-rate` | 5,760 → **1,440** | 축소 완료 |
| `MARKET_INFO` | (공유) | `/market-calendar/*` | 소량 (캐시) | 유지 |
| `RANKING` | 별도 | `/rankings` | 2,304 → **약 900** (마감 시장 스킵) | 축소 완료 |
| `MARKET_INDICATOR` | 별도 | `/market-indicators/*` | 소량 | 유지 |

### `/candles`는 price API로 대체할 수 없다

`PriceResponse`의 필드는 **`symbol` · `timestamp` · `lastPrice` · `currency` 넷뿐**이다
(OpenAPI 스펙 실측). OHLC도 거래량도 전일종가도 없다. 일별 종가 표는 종가와 거래량을
저장하므로 `/candles?interval=1d`가 유일한 소스다 — 「종가를 price API로 받자」는
불가능하고, 마감 순간의 `lastPrice`를 종가로 쓰는 것은 거래량을 포기하고 마감 타이밍에
기대는 것이라 크리티컬 데이터의 소스로 삼을 수 없다.

줄일 수 있는 쪽은 **호출당 종목 수가 아니라 대상 종목 수**다. 그래서 실제로 한 일은
「Toss가 취급하지 않는 종목을 대상에서 뺀다」였다(회차당 3,250여 건 절감).

### `/stocks/all` 유니버스 — **도입했다** (2026-09-12)

`toss_universe` 잡(매일 01:00 · worker-filings)이 마켓 일곱을 돌아 「토스가 거래를
지원하는 종목」을 확정한다. **하루 7호출**이고 버킷(`STOCK_ALL`)은 우리가 아무것도
쓰지 않던 버킷이라 경고 스윕의 `STOCK`(관측 5/s)과 다투지 않는다.

**첫 실행 실측(2026-09-12 00:34, 45초):**

	유니버스 15,314종목 (마켓 7) — 거래가능 14,520 · 숨김 3,398 · 이름 4,488건 갱신

숨은 3,398은 앞서 따로 측정한 경고 스윕의 404 건수(3,388)와 **거의 같다** — 방법이
다른 두 측정이 같은 수를 가리켰다. 표본을 보면 워런트(`ILLRW`·`NTRBW`)·SPAC 유닛
(`ARTCU`)·SPAC 껍데기(`KOYN`·`HACQ`)·우선주 예탁증서(`TFC^R`·`DX^C`)·라이츠(`RFAMR`)·
OTC 해외(`FTRAF`·`IXEDF`)다. 열 수도 없는 종목이 검색 결과에 섞여 있었던 것이다.

**도입 당일 실제로 전 종목이 숨었다 (2026-09-12 01:00 — 자체 사고).**

01:00 예약 실행 도중 배포로 워커가 재시작됐고, 17,918종목이 전부 `toss_tradable=0`이
되어 **검색이 통째로 비었다**(AAPL 조회도 0건). 원인은 둘이 겹쳤다:

1. **두 문장이 트랜잭션 밖에 있었다.** 「받은 것을 1로」와 「나머지를 0으로」 사이에서
   죽으면 두 번째만 남는다.
2. **시각 비교가 정밀도에 기대고 있었다.** `toss_checked_at`은 `TIMESTAMP` 정밀도
   **0(초)**인데 Go의 `time.Now()`는 나노초를 담는다. 저장하면 잘리고, 같은 값을
   비교에 쓰면 어긋난다 — 실험으로 확인:

		SET @t = '2026-09-12 02:00:00.456';
		SELECT CAST(@t AS DATETIME(0)) <> @t;   -- 1 (참)
		SELECT CAST(@t AS DATETIME(0)) <  @t;   -- 1 (참)

   즉 방금 1로 만든 행이 「이번에 안 온 행」 조건에 그대로 걸렸다.

고친 방식은 셋이고, **세 번째가 핵심**이다:

- 두 문장을 **한 트랜잭션**에 넣었다 — 커밋 전에 죽으면 예전 상태가 남는다.
- 시각을 **초 단위로 잘라** 쓴다(`Truncate(time.Second)`) — 저장값과 비교값이 문자
  그대로 같아진다. 정밀도에 기대는 코드를 남기지 않는다.
- **커밋 직전에 결과를 센다.** 반영 후 보이는 종목이 문턱(1,000) 아래면 롤백한다.
  앞의 둘이 또 다른 이유로 어긋나도 증상은 언제나 같으므로(카탈로그가 사라진다),
  그 결과 자체를 막는 선을 마지막에 둔다. 실측으로 이 선이 작동했다 — 고치기 전
  재실행에서 「반영 후 보이는 종목이 0건뿐이다 — 롤백한다」로 거부했고 데이터는
  그대로였다.

**안전선 넷** (틀리면 증상이 「종목이 화면에서 사라진다」라 코드가 아니라 시험이 지킨다):

1. **일곱 마켓이 다 와야 반영한다.** 빠진 마켓이 있는 합집합으로 「나머지를 숨김」을
   돌리면 그 마켓이 통째로 사라진다.
2. **합집합이 1,000건 미만이면 반영하지 않는다.** 상류 장애·권한 변화로 빈 배열이
   오는 경우를 「거래 가능한 종목이 없다」로 읽지 않는다.
3. **반영 후 보이는 종목이 1,000건 미만이면 롤백한다.** 2번이 입력을 보는 선이라면
   이쪽은 **결과**를 보는 선이다 — 위 사고가 통과한 자리가 정확히 여기였다.
4. **NULL은 숨기지 않는다.** `toss_tradable IS NULL OR = 1` — 배치가 처음 돌기 전이나
   새로 들어온 종목을 숨기면 화면이 통째로 빈다.

**우리 `market` 컬럼으로 마켓 단위 반영을 하지 않는다.** 두 분류가 다르다 — 우리는
ARCX 2,655 · BATS 1,544 · OOTC 797 · KRX 2,323 · KONEX 107을 따로 쓰는데 토스는 그것을
`US_ETC`·`KR_ETC`로 답한다. 매핑해 맞출 수도 있지만 그 매핑이 틀리는 날 5,000종목이
숨으므로, 합집합만 쓰고 우리 분류는 보지 않는다.

**숨기는 것은 발견(검색)이고 가진 것은 아니다.** `/stocks/search`는 이 조건을 걸지만
`/stocks?symbols=`(보유·관심 화면이 쓰는 명시 조회)는 걸지 않는다 — 이미 담아 둔
종목이 사라지면 사용자의 기록이 사라진 것처럼 보인다.

### 한국어 종목명이 곧 뉴스 별칭이다 (2026-09-12)

같은 응답의 `name`이 한국어 종목명이고 **주식 종류까지** 들어 있다(GOOGL=알파벳 A ·
GOOG=알파벳 C · BRK.A=버크셔 해서웨이 A). 첫 실행이 **4,488건**을 갱신했다 — 카탈로그의
한국어 이름이 그만큼 낡아 있었다.

별칭을 손으로 적을 필요가 없었던 이유가 이것이다. 예전에는 `usNewsNameAliases` 13개를
손으로 유지하고 있었고 그 목록에 `구글`은 있는데 **`알파벳 A`·`알파벳 C`는 없었다.**
이제 두 자리에서 이 이름을 쓴다:

- **구글 뉴스 검색어**(`buildUSTopicQueue`) — 종목 이름으로 검색하고 결과에 그 심볼을
  **그 자리에서** 붙인다(국내 경로와 같은 구조). 본문 매칭에 의존하지 않으므로 오탐이 없다.
- **태깅 인덱스**(`addUSNameIndexKeys`) — 한국어 이름도 색인한다. 위험(더 긴 낱말의
  머리로 걸리는 것)은 국내 13,000종목이 이미 쓰는 `nameTokenMatch`의 조사 규칙이
  막는다(「애플리케이션」·「메타버스」 차단). 이름 자체가 일상어인 경우만 목록에서
  뺀다(`ambiguousUSLocalNames` — 「비자」).

실측 확인: 앱 검색에서 `알파벳` → GOOGL(알파벳 A) · GOOG(알파벳 C), `버크셔` → BRK.A/B.

### (참고) 도입 전 검토 내용

`GET /api/v1/stocks/all?market=…&status=ACTIVE`는 **「토스증권에서 거래 가능한 종목만」**
을 마켓 단위로 한 번에 준다(스펙: NASDAQ 약 2,800건 · gzip 30KB · 하루 1회 조회 권장).
마켓이 7개(KOSPI·KOSDAQ·NYSE·NASDAQ·AMEX·KR_ETC·US_ETC)이므로 **하루 7호출**이고,
그 버킷(`STOCK_ALL`)은 **우리가 아무것도 쓰지 않는** 버킷이다.

이것이 메우는 자리는 지금 **404를 한 번씩 맞아 가며 배우고 있는 사실**이다:

- `symbol_sync_state`의 warnings 3,388행이 `404 stock-not-found`였다(전체의 19%)
- 해외 종가 구멍 메우기가 회차마다 3,250여 종목을 헛돌았다

둘 다 「우리 카탈로그(NORMAL)가 Toss 취급 집합의 상위집합」이라는 하나의 사실 때문이고,
지금은 그것을 **종목마다 404 한 번을 치르고** 알아낸다(30일마다 다시). `/stocks/all`로
받으면 하루 7호출로 그 집합을 **정확히** 알 수 있고, 커버리지 분모도 추정이 아니라
사실이 된다.

**아직 구현하지 않았다** — 새 배치 잡과 저장 자리(종목 마스터에 「Toss 취급」 표시)가
필요하고, 그 설계는 사용자 결정이 하나 필요하다: 그 집합에서 빠진 종목을 **화면에서도
숨길지**(=우리 카탈로그를 Toss 기준으로 줄일지), 아니면 **수집에서만 뺄지**.

### 배경 폴러는 **세 틱 모두** 세션을 본다 (2026-09-12)

`tickFast`(호가·체결)와 `tickSlow`(캔들)는 각각 「마감 중에도 계속 받고 있었다」를
실측하고 세션 필터를 붙였는데, **`tickPrice`만 빠져 있었다.**

실측(2026-09-12 00:37~00:43, 인바운드 요청 0건):

	/prices  146호출 / 5분 = 분당 29회

스택 추적으로 전부 `marketPoller.Run → tickPrice → RefreshPrices`였다.

**그 실측은 낭비가 아니었다 — 조사자가 세션을 잘못 읽었다.** 처음에 「토요일이니 양
시장 마감」이라고 적었는데, KST 토요일 00:40은 **ET 금요일 11:40**이라 미국 정규장이
열려 있었다(서버 판정도 `us: regular`였다). 그 분당 29회는 정상 동작이다.

**그래도 필터는 필요하다.** 누락 자체는 사실이었고 — `tickFast`·`tickSlow`는 각각
실측 후 이 필터를 붙였는데 `tickPrice`만 없었다 — 양 시장이 정말로 닫히는 구간
(평일 KST 06:00~08:00, 주말 토요일 05:00 이후)에는 그 호출이 전부 버려지는 호출이
된다. 효과는 그 창에서 나타나고, 이 문서의 숫자로는 아직 확인되지 않았다.

마감 중 가격이 필요한 곳은 화면인데 그 값은 **이미 캐시와 DB에 있다**
(`symbol_last_prices`). 요청 경로의 `EnsurePrices`가 캐시가 비면 그때 한 번 받으므로
배경에서 1초마다 미리 받아 둘 이유가 없다 — 받아 봐야 이미 가진 것과 같은 값이다.

같은 누락이 다시 생기지 않게 시험이 **세 틱 모두** 필터를 부르는지 센다
(`TestAllPollerTicksFilterByLiveSession`).

### 결론

| 사용자 제안 | 검토 결과 |
|-------------|-----------|
| price API가 가성비 최고 | **맞다** — 200종목/호출이고 대체재가 없다 |
| 나머지를 최대한 안 쓰는 것이 최적 | **부분적으로만** — 한도가 그룹별이라 「안 쓰기」보다 **「버킷 분리」**가 효과가 크다 |
| 그것이 한도 방지의 핵심 | **핵심은 다른 곳이었다** — 실측 사고 셋 모두 「같은 버킷 안의 불필요한 호출」이었고, 그걸 뺀 것이 429를 90건→0건으로 만들었다 |

## 캐시·DB 계층 (API 호출 절감)

| 데이터 | MySQL | Redis |
|--------|-------|-------|
| 시장 지수 전체 | `market_indices` | `quotes:indices:snapshot:v2` (10분) |
| 지수 개별 | `market_indices` (행당 1 index_id) | `quotes:index:v6:{id}` (45일) |
| 금 spot | `market_indices` (`index_id=gold`) | `market:gold:spot:v1:{symbol}:{currency}` (45일) |
| Indices poller | — | **`MarketIndexStore` 1분** 주기, **provider batch** + `MinInterval` |

백그라운드 갱신: `internal/service/market_index_store*.go` — **batch** API 호출 → MySQL upsert + Redis 캐시. 상세 → [`docs/MARKET_INDICES_BATCH.md`](../../docs/MARKET_INDICES_BATCH.md).  
FE/HTTP 요청 경로는 **DB·캐시만** 읽음 (`QuotesService.ListIndices` → `MarketIndexStore.List`).

지수 갱신 흐름:

1. Poller / 사용자 요청 → 스냅샷 또는 per-index good cache 확인  
2. `MinInterval` 미경과 시 **외부 API 호출 생략**  
3. 캐시 miss 시 client 호출 → `apibudget.Check` → HTTP → `Record`  
4. 429·quota·budget 차단 시 **stale 캐시** 반환 (`market:gold:spot`, `quotes:index:v6`, sparkline/baseline)  
5. 스냅샷 저장 시 value=0 항목은 good cache로 **병합** — 0으로 덮어쓰지 않음

## 예상 월간 호출량 (단일 인스턴스, 정상 운영)

| Provider | 대략적 상한 | 근거 |
|----------|-------------|------|
| goldprice | ~480/월 | 90분 간격 × 1 symbol |
| fred | ~480 Record/일 × 6 HTTP ≈ 2,880 HTTP/일 (일 5k cap 이내) | **3분** batch (`fred:us`) |
| finnhub | indices 2/5분 + earnings 06:00 ~18 + 5분 당일 1 | 5분 간격(시세). 실적은 전용 버킷 |
| fnguide | ~60/월 | 하루 2회(당월·전월) × 30일 |
| upbit | ~720/월 | 2분 간격, ticker+candles |

goldprice는 이전에 indices poller **30초** 주기로 ~86k/월 호출되어 한도 초과가 발생했습니다.  
현재는 90분 캐시 + budget + poller 5분 + per-index skip으로 **900/월 이하**로 유지됩니다.

## US 뉴스 번역 — Azure Translator (**월 문자 수**로 셉니다)

해외 제목 EN→KO는 **Azure AI Translator**가 합니다(2026-09-11 전환). 자체 모델
(`translator` 컨테이너, llama.cpp + NiuTrans LMT-60)은 기본 기동에서 빠졌습니다 —
`docker-compose.yml`의 `legacy-translator` 프로파일로 남아 있고, 되돌리려면
`TRANSLATE_PROVIDER=local`과 그 프로파일입니다. 옮긴 이유는
`be/internal/translate/azure.go` 머리글에 있습니다(제목 한 건 0.9~2.8초, 상시
1.5코어·2GB, 모델 적재 중에는 그 주기가 통째로 영어로 남음).

**이 항목만 호출 수가 아니라 문자 수를 셉니다.** 상대 과금이 그렇게 되어 있어서이고,
`apibudget`은 같은 정수 카운터를 월 창으로 씁니다(`TokensPerMonth`, 키는
`apibudget:v1:azure_translate:tokens:month:YYYY-MM`).

| 항목 | 한도 | 근거 | 세는 것 |
|------|------|------|---------|
| `azure_translate` | **월 2,000,000자** · 분 400호출 | F0 무료 등급(월 200만 자, 일일 한도 없음) | 보낸 **원문 문자 수** |

실측(2026-09-08~11, 영어 원문만 · 한국어 소스 제외):

| 날짜 | 번역 제목 | 문자 |
|------|-----------|------|
| 09-08 (월) | 777 | 61,247 |
| 09-09 (화) | 1,253 | 97,038 |
| 09-10 (수) | 1,345 | 103,027 |
| 09-06 (토) | 234 | 19,161 |

평일 약 88K · 주말 약 29K → **월 약 2.17M자**로 무료 한도를 8%쯤 넘습니다. 수집량을
줄여 맞추기로 했습니다(2026-09-11 사용자 결정) — 그 조정은 `news_poll_settings`
쪽이고 이 한도는 조정이 어긋났을 때의 마지막 방어선입니다.

- 한 요청에 제목 **최대 100건**을 싣습니다(상대 상한은 1,000건/50,000자). 낮춘 이유는
  실패 단위를 작게 두기 위해서입니다 — 한 요청이 실패하면 그 안의 제목이 전부 영어로 남습니다.
- 문자는 **보내기 전에** 셉니다. Azure는 응답에 사용량을 담지 않고 과금이 보낸 원문
  기준이라, 우리가 보낸 길이가 곧 청구서입니다.
- 한국어 제목은 보내지 않습니다(`containsHangulText`). 그래서 해외 뉴스 1,558건 중
  실제 번역 대상은 1,345건이었습니다(09-10 실측).
- Reader 본문은 여전히 **번역하지 않습니다** — 기사 한 건이 2,000단어라 이 한도를
  혼자 다 씁니다. 이미 저장된 번역은 그대로 씁니다.
- 키가 없으면 엔진이 **안 붙습니다**(조용한 폴백 없음). 제목은 영어로 남고 기동 로그가
  그 사실을 찍습니다. 거절·실패 건수는 `translate.Chain.Snapshot()`이 셉니다.

Papago(NCP)는 2026-09-03에 걷어냈습니다 — 자격증명이 만료됐고 갱신하지 않기로 했습니다.
그때까지 `papago_text`(제목)·`papago_web`(리더 본문) 두 항목이 여기 있었습니다.

## 배치 ETP 동기화 (`cmd/sync-etp`)

운영 API와 **별도 budget bucket** 으로 카탈로그·로고 마이그레이션을 실행합니다.

| 단계 | 외부 호출 | 기본 throttle |
|------|-----------|---------------|
| 국내 카탈로그 | KRX ETF + ETN (2 HTTP) | 호출 간 **2초** sleep + `MinInterval` 2초 |
| 미국 카탈로그 | Finnhub `/stock/symbol` (**1 HTTP**) | `finnhub_migrate` bucket |
| 국내 로고 | Naver mobile JSON + PNG | **150ms**/symbol |
| 미국 로고 | FMP CDN HEAD (키 없음) | **4** workers |

```powershell
cd be
go run ./cmd/sync-etp -dry-run          # fetch only, no DB/logo writes
go run ./cmd/sync-etp -catalog          # DB insert-only (skip existing symbols)
go run ./cmd/sync-etp -logos            # delta logo migrate for ETP rows in DB
go run ./cmd/sync-etp                   # catalog + logos (full conservative run)
```

로고만 재실행: `go run ./cmd/migrate-logos -from-db -etp-only -delta-only -seed=false`

## 장애 대응

### goldprice 429 / monthly quota exhausted

1. `GET /health/api-quotas`에서 `goldprice.monthCount`, `blockedUntil` 확인  
2. Redis: `GET apibudget:v1:goldprice:count:month:YYYY-MM`  
3. 캐시된 금 시세: `GET market:gold:spot:v1:XAU:USD` — TTL 90분 동안 UI는 캐시값 표시  
4. 한도 리셋은 **달력 월(KST)** 기준 새 키로 자동 리셋  
5. 긴급 시 `DEL apibudget:v1:goldprice:blocked_until` (호출 재개는 월간 quota 확인 후)

### finnhub / FRED / Upbit rate limit

- `blocked_until` 만료까지 해당 provider HTTP 차단  
- 지수·실적은 good cache / snapshot으로 stale 표시 가능

### DART TLS (Windows)

- corp code는 `data/cache/dart_corp_code.zip` 로컬 fallback  
- budget은 live HTTP 성공 시에만 `Record`

## 설정

API 키는 `.env` / `be/.env.example` 참고:

- `GOLDPRICE_API_KEY`
- `FRED_API_KEY`
- `FINNHUB_API_KEY`
- `FINNHUB_NEWS_API_KEY` (optional — US news; RSS-only when empty)
- `KRX_API_KEY` (선택 — `sync-etp` 국내 ETP 카탈로그; 없으면 `-krx-etp-csv` fallback)
- `UPBIT_*` (선택, public quotation은 키 없이 동작)
- `DART_API_KEY`
- `SEC_EDGAR_USER_AGENT`
- `OPENAI_API_KEY` 또는 `GEMINI_KR_API_KEY` / `GEMINI_US_API_KEY` (LLM, 시장별). 레거시 `GEMINI_API_KEY`는 지역 키가 없을 때의 공유 폴백.

한도 값 변경은 `DefaultLimits` 수정 후 재배포. 운영 중 Redis 카운터는 배포와 무관하게 유지됩니다.

## LLM (`llm_kr` / `llm_us`, 공유 키일 때 `openai`) — 요청 + 토큰 이중 한도

국내와 해외는 **서로 다른 Gemini API 키**를 쓴다. 키마다 무료 티어 500 RPD가
따로 적용되므로, 로컬 `apibudget` 버킷도 `llm_kr` / `llm_us`로 나눈다. 두 키가
같거나 하나만 있으면 예전처럼 `openai` 공유 버킷이다.

AI 요약 배치가 시장별로 하루 백여 건을 순차 호출하므로, 이 provider만 **요청 수와
토큰을 분/일 두 축 모두**로 제한합니다. 한도 숫자는 키마다 동일합니다(500이
반으로 쪼개지는 것이 아니라 키당 500).

| 항목 | 기본값 | env override |
|------|--------|--------------|
| 일일 요청 (RPD) | 500 | `LLM_DAILY_QUOTA` |
| 분당 요청 (RPM) | 15 | `LLM_PER_MINUTE` |
| 분당 토큰 (TPM) | 250,000 | `LLM_TOKENS_PER_MINUTE` |
| 일일 토큰 (TPD) | 1,000,000 (자체 상한) | `LLM_TOKENS_PER_DAY` |

- 기본값은 Gemini 무료 티어 기준입니다. **TPD는 공표된 값이 아니라 자체 상한**이므로,
  실제 한도를 provider 콘솔에서 확인해 env로 맞추세요. 음수를 넣으면 해당 상한이 해제됩니다.
- `Check`는 사후 차단(이미 쓴 양이 한도에 닿으면 거부)이고, `CheckTokens`는 **사전 차단**입니다.
  프롬프트 길이로 토큰을 추정해(`llm.EstimateTokens`) 한도를 넘길 호출 자체를 막습니다.
  한글은 글자당 1토큰, 라틴 문자는 3자당 1토큰으로 과대 추정해 안전측으로 계산합니다.
- 응답에 `usage.total_tokens`가 없으면 추정치를 대신 기록합니다 — 미계측 호출이 한도를
  조용히 넘기지 않도록.

Redis 키: `apibudget:v1:llm_kr:tokens:*`, `apibudget:v1:llm_us:tokens:*` (공유 키면 `openai`). TTL은 분 창 2분, 일 창 48시간.

배치가 소비 가능한 양은 해당 시장 키의 `Headroom()`에서 예비분(`quota_reserve` / `token_reserve`)을 뺀 값입니다.
예비분은 그 시장 뉴스 브리핑·관리자 수동 생성 몫으로 남겨둡니다. 해외 제목 번역은 자체 모델이 하므로 LLM 예비분에 들어가지 않습니다.

새 LLM 기능도 국내/해외를 `llm.Pool.For(market)`로 나눠야 한다. 한 키에 몰면 500 RPD 충돌이 다시 난다.
