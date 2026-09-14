# Market indices batch refresh

> 구현일: 2026-07-26 · 관련 코드: `be/internal/service/market_index_store_batch.go`, `index_batches.go`

## 배경

기존 per-index 폴링은 FRED `MinInterval`(15분)이 **provider 전역**이라 catalog 순서상 `nasdaq`/`nasdaq100`만 budget slot을 차지하고 `sp500`/`dow`/`vix`는 영구 `value: 0`이 되는 문제가 있었다.

## 개선 요약

| 항목 | 이전 | 이후 |
|------|------|------|
| FRED 갱신 | index별 15분, 1 series = 1 budget Record | **batch 1회 = 7 series, 1 budget Record** |
| FRED 간격 | 15분 | **3분** (일 5,000 cap 이내) |
| HTTP read | memory → Redis → MySQL (변경 없음) | 동일 |
| warm-up | 없음 | **기동 시 전 batch 1회** (최대 90s timeout) |
| API 응답 | `data`, `updatedAt` | + per-index `updatedAt`, `dataKind`, `groups` |

## Provider batch 그룹

| Batch ID | Source | Indices | Min interval | EOD 1회 |
|----------|--------|---------|--------------|---------|
| `toss:kr` | Toss | kospi, kosdaq | 1분 | ✅ (통합장 마감 후) |
| `toss:fx` | Toss | usd-krw | 1분 | — |
| `fred:us` | FRED | nasdaq, nasdaq100, sp500, dow, vix, us10y | **3분** | ✅ (미국 애프터마켓 종료 후) |
| `finnhub:us` | Finnhub | nq100, sox | 5분 (apibudget와 동일) | — |
| `upbit:crypto` | Upbit | btc | 2분 | — |
| `goldprice:spot` | goldprice.dev | gold | 90분 | — |

Poller 주기: **1분** (`marketIndexPollInterval`). batch `minInterval` 미경과 시 해당 batch skip.

## FRED quota (3분 간격)

- Budget Record: 최대 **480회/일** (24×60÷3)
- HTTP: batch당 7 series ≈ **3,360 req/일** (FRED 일 5,000 cap 이내)
- `apibudget.Check` → batch 전체 1× `Record` (`fred.go` `QuoteBatch`)

## EOD persist

장 마감 후 1회 추가 실행 (Redis 키 `indices:eod:v1:{batch}:{YYYYMMDD}`, TTL 48h):

- `toss:kr` — KRX 통합장 애프터마켓 종료 (`KRIntegratedAfterEnded`)
- `fred:us`, `finnhub:us` — 미국 애프터마켓 종료 (`USAfterMarketEnded`)

FRED 데이터 자체는 **일간 EOD**이므로 FE에서 `dataKind: "eod"` 배지 표시.

## API shape

`GET /api/v1/quotes/indices`

```json
{
  "data": [
    { "id": "sp500", "value": 6300.1, "dataKind": "eod", "updatedAt": "2026-07-26T01:30:00Z" }
  ],
  "updatedAt": "2026-07-26T01:30:00Z",
  "groups": {
    "fred:us": { "updatedAt": "2026-07-26T01:30:00Z", "source": "fred" }
  },
  "source": "toss"
}
```

## Side effects / 주의

1. **기동 지연**: warm-up이 모든 batch를 순차 호출 → 외부 API 지연 시 최대 ~90s (timeout).
2. **FRED 사용량 증가**: 15분→3분이지만 batch로 series당 Record 공유 → cap 여유 있음.
3. **Finnhub vs batch 간격**: batch `minInterval`은 apibudget `MinInterval`과 맞춤 (5분). 3분으로 맞추면 budget 거절 후 fallback만 반복.
4. **Partial failure**: batch 내 일부 series 실패 시 나머지는 persist, 실패 항목은 good cache / MySQL fallback (`applyIndexFallback`).
5. **Redis EOD 키**: 재시작해도 당일 EOD 1회 제한 유지.
6. **group 메타**: in-memory + `batch:{id}` lastRefresh; 재시작 후 첫 poll에서 group interval 리셋 (warm-up이 즉시 채움).
7. **FE**: FRED 지수는 “종가”, Finnhub ETF/Toss/Upbit는 “실시간” 배지.

## 추가 개선 제안 (별도 작업)

- **Finnhub index source 다변화**: S&P/Dow live는 FRED EOD만 — 유료 index API 또는 다른 provider 검토.
- **Snapshot TTL**: Redis snapshot 10분 vs poller 1분 — snapshot은 변경 시에만 갱신되므로 OK; cold start 시 DB fallback 유지.
- **SSE/push**: 지수는 30s FE poll; WebSocket/SSE로 전환 시 API 부하 감소 가능.
- **group 메타 Redis persist**: 재시작 후 group `updatedAt` FE 표시까지 즉시 복구.
- **Health**: `/health/api-quotas`에 batch별 last success 타임스탬프 노출.

## 진행 이력

| 일시 | 내용 |
|------|------|
| 2026-07-26 | Root cause: FRED global MinInterval + per-index budget → sp500/dow/vix 미갱신 |
| 2026-07-26 | Batch refresh + FRED 3분 + warm-up + EOD keys + API/FE metadata 구현 |
| 2026-07-26 | 문서화 (`MARKET_INDICES_BATCH.md`), 테스트 (`index_batches_test.go`) |
