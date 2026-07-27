# Backend API docs

## 토스증권 Open API

- **Spec:** [`toss-openapi.json`](./toss-openapi.json) (OpenAPI 3.1, v1.2.4)
- **Base URL:** `https://openapi.tossinvest.com`
- **인증:** `POST /oauth2/token` — OAuth 2.0 Client Credentials
  - `TOSS_CLIENT_ID` → `client_id`
  - `TOSS_CLIENT_SECRET` → `client_secret`
- **계좌 API:** `X-Tossinvest-Account` 헤더 (`GET /api/v1/accounts`의 `accountSeq`)

백엔드 연동 시 `internal/toss/` 클라이언트 패키지에서 이 spec을 기준으로 엔드포인트·스키마를 맞춥니다.

### Rate limit 그룹 (spec tags 참고)

| 그룹 | 용도 |
|------|------|
| `AUTH` | 토큰 발급 |
| `MARKET_DATA` | 현재가, 호가, 체결, 상하한가 |
| `MARKET_DATA_CHART` | 종목 캔들 |
| `STOCK` | 종목 정보, 유의사항 |
| `MARKET_INFO` | 환율, 장 캘린더 |
| `MARKET_INDICATOR` | KOSPI/KOSDAQ, 국채, 투자자별 매매 |
| `ACCOUNT` / `ASSET` / `ORDER` | 계좌·보유·주문 |

429 응답 시 `Retry-After`, `X-RateLimit-*` 헤더를 따릅니다.

## 외부 API 호출 한도

FRED, Finnhub, goldprice.dev, Upbit, DART, SEC EDGAR 등 서드파티 API의 공식 한도·적용 정책·Redis 모니터링 키:

- **[`api-quotas.md`](./api-quotas.md)**
- 운영 점검: `GET /health/api-quotas`
