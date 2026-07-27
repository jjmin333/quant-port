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
| **fred** | 나스닥·S&P·다우·VIX 등 FRED 시리즈 | **120 req/min** (API key), 일일 제한은 계정별 | 5,000/일, 100/min | **3분** (batch 5 series = 1 Record) |
| **finnhub** | ETF 시세(QQQ, SOXX), 실적 캘린더 | Free: **60/min** ([finnhub.io](https://finnhub.io/docs/api/rate-limit)) | 50/min | 5분 |
| **upbit** | BTC/KRW, 스파크라인 | Exchange **10 req/sec**, Quotation **600/min/IP** ([Upbit](https://docs.upbit.com/docs/user-request-guide)) | 10/min | 2분 |
| **dart** | 공시·corp code | **20,000/일** ([Open DART](https://opendart.fss.or.kr/)) | 18,000/일, 60/min | 1초 |
| **edgar** | SEC XBRL 재무 | **10 req/sec** ([SEC fair access](https://www.sec.gov/os/webmaster-faq#developers)) | 8/min | 150ms |

**Toss Open API**는 별도 rate-limit 그룹(`MARKET_DATA` 등)과 429 백오프로 처리합니다. → [`README.md`](./README.md#rate-limit-그룹-spec-tags-참고)

## 캐시·DB 계층 (API 호출 절감)

| 데이터 | MySQL | Redis |
|--------|-------|-------|
| 시장 지수 전체 | `market_indices` | `quotes:indices:snapshot:v1` (10분) |
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
| fred | ~480 Record/일 × 5 HTTP ≈ 2,400 HTTP/일 (일 5k cap 이내) | **3분** batch (`fred:us`) |
| finnhub | indices 2 + earnings 1/day | 5분 간격 |
| upbit | ~720/월 | 2분 간격, ticker+candles |

goldprice는 이전에 indices poller **30초** 주기로 ~86k/월 호출되어 한도 초과가 발생했습니다.  
현재는 90분 캐시 + budget + poller 5분 + per-index skip으로 **900/월 이하**로 유지됩니다.

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
- `UPBIT_*` (선택, public quotation은 키 없이 동작)
- `DART_API_KEY`
- `SEC_EDGAR_USER_AGENT`

한도 값 변경은 `DefaultLimits` 수정 후 재배포. 운영 중 Redis 카운터는 배포와 무관하게 유지됩니다.
