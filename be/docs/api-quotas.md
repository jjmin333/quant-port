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
| **finnhub_news** | 미국 뉴스 (`/news`, `/company-news`) | 별도 API 키, free **60/min** | 50/min | 2분 |
| **papago_text** | US 뉴스 제목 EN→KO (Text Translation) | NCP **글자** 과금 (기본 일 500만 자, RPM 공표 없음) | **6000회/일, 60/min** | 150ms |
| **papago_web** | US Reader 본문 EN→KO (Website Translation, on-demand) | NCP 글자 과금 | **2000회/일** | 300ms |
| **krx** | 국내 ETF/ETN 카탈로그 (`etf_bydd_trd`, `etn_bydd_trd`) | Open API 일일 한도 | 500/일, 10/min | **2초** |
| **upbit** | BTC/KRW, 스파크라인 | Exchange **10 req/sec**, Quotation **600/min/IP** ([Upbit](https://docs.upbit.com/docs/user-request-guide)) | 10/min | 2분 |
| **dart** | 공시·corp code | **20,000/일** ([Open DART](https://opendart.fss.or.kr/)) | 18,000/일, 60/min | 1초 |
| **edgar** | SEC XBRL 재무 (조회 경로) | **10 req/sec** ([SEC fair access](https://www.sec.gov/os/webmaster-faq#developers)) | 8/min | 150ms |
| **edgar_bulk** | 내부자 거래 Form 4 적재 배치 (worker-filings) | 동일 (같은 User-Agent) | 240/min | **300ms** |
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

배치 페이스 300ms는 약 3.3 req/sec으로 공표 한도의 3분의 1이다. 상한 240/min을
그 위에 둔 것은 안전판이라서다: 두 값이 같으면 간격을 지켜도 분당 한도에 걸릴 수
있고, `Check` 실패는 그 종목을 통째로 건너뛰게 만든다.

실제 호출량은 **증분**이라 훨씬 적다. 이미 저장한 accession은 다시 내려받지 않으므로
평시 종목당 1~2호출이다(실측: 같은 8종목 재실행이 54초 → 9초, 신규 0행).

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

**Toss Open API**는 별도 rate-limit 그룹(`MARKET_DATA` 등)과 429 백오프로 처리합니다. → [`README.md`](./README.md#rate-limit-그룹-spec-tags-참고)

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

## US 뉴스 번역 (Papago) 예상 호출량

번역은 **DB 저장 후 Redis/DB 조회**로 중복 API 호출을 막습니다.

| 구분 | API | 트리거 | 예상 일일 호출 | 앱 한도 |
|------|-----|--------|----------------|---------|
| 제목 | `papago_text` | 2분마다 백필 24건 (제목만) | 신규 ~100–200/일 (백필 완료 후) | 6000/일 · 60/min |
| Reader 본문 | `papago_web` | US Reader **최초 1회** 클릭 | DAU×오픈률 (예: 50–200/일) | 2000/일 |

- US DB 전체 Reader 1회 오픈 ≈ **1,200 web 호출** (수일에 분산 가능, DB 재사용).
- NCP Papago 글자 한도(일 500만 자)는 위 패턴에서 **충분히 여유**. 분당 60은 NCP 공식 RPM이 아니라, 2분×24건 버스트가 우리 상한을 넘지 않게 둔 값이다.
- Papago `Check` 실패(일 6000·분당 60) 시 그 기사는 다음 2분 주기에 다시 잡힌다. **Gemini 번역 폴백은 없다.**

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
- `OPENAI_API_KEY` 또는 `GEMINI_API_KEY` (LLM)

한도 값 변경은 `DefaultLimits` 수정 후 재배포. 운영 중 Redis 카운터는 배포와 무관하게 유지됩니다.

## LLM (`openai` provider) — 요청 + 토큰 이중 한도

AI 요약 배치가 하루 수백 건을 순차 호출하므로, 이 provider만 **요청 수와 토큰을
분/일 두 축 모두**로 제한합니다.

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

Redis 키: `apibudget:v1:openai:tokens:min:*`(TTL 2분), `apibudget:v1:openai:tokens:day:*`(TTL 48시간).

배치가 소비 가능한 양은 `Headroom()`에서 예비분(`quota_reserve` / `token_reserve`)을 뺀 값입니다.
예비분은 뉴스 시장 브리핑·관리자 수동 생성 몫으로 남겨둡니다.
