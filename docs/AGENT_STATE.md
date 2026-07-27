# Agent State

> **세션 시작: 이 파일만 읽기. 종료: 이 파일만 갱신.** 상세 이력 → `docs/archive/sessions.md`

## Now
- **저장 정책 (07-25 확정)**: **MySQL = source of truth**, **Redis = hot cache** (유실 방지)
- **Redis 장애 (07-26)**: **기동 시 Redis 필수** (ping 실패면 BE 시작 불가). 런타임 Redis down 시 `/ready`는 `degraded` (200); apibudget/Toss token은 in-memory fallback
- **Market indices (07-26)**: provider **batch refresh** + FRED **3분** + warm-up + EOD persist → `docs/MARKET_INDICES_BATCH.md`
- Quotes: `candle_bars` + `quote_snapshots` / `orderbook_snapshots` / `trade_ticks` / `symbol_last_prices`
- 장 마감: DB-first → Redis warm → (miss) Toss 1× backfill → **MySQL + Redis write-through**
- Poller: 장중만 Toss live; 성공 시 항상 MySQL upsert

## Durable storage (MySQL)

| 테이블 | 내용 | 갱신 시점 |
|--------|------|-----------|
| `candle_bars` | 1m·1d 완성봉 | Poller tail, scroll backfill |
| `quote_snapshots` | QuoteDetail JSON | terminal refresh, detail fetch, backfill |
| `orderbook_snapshots` | OrderBook JSON | terminal refresh, backfill |
| `trade_ticks` | 최근 50 tick/symbol | terminal refresh, backfill (replace) |
| `symbol_last_prices` | 목록용 last price | GetPrices refresh, detail persist |
| `market_indices` | 지수 스냅샷 | index poller |
| `stocks` | 종목 마스터 | catalog backfill |

Migration: `011_quote_terminal_persist.sql` (auto via `MigrateExtendedSchemaIfNeeded`)

## Read chain (모든 terminal 데이터)

```
Redis (short TTL) → Redis last-good → MySQL → Toss 1× backfill (장 마감/cold) → MySQL + Redis
```

## Quotes 데이터 정책

| 데이터 | 장중 | 장 마감/주말 | persist |
|--------|------|--------------|---------|
| 캔들 | Poller→Toss→DB+Redis | DB tail → backfill | `candle_bars` |
| detail | Poller+Toss | DB → backfill | `quote_snapshots` |
| 체결/호가 | Poller+Toss | DB → backfill | `trade_ticks`, `orderbook_snapshots` |
| 목록 가격 | Toss 20s | DB `symbol_last_prices` | 동일 |

## 개발 원칙 (크리티컬 데이터)

1. **사용자에게 보이는 마지막 상태**는 Redis만 두지 않는다 → MySQL write-through
2. **Redis는 가속/폴링 TTL**용; cold start는 MySQL에서 복구
3. **Toss 1× backfill**은 DB·Redis 둘 다 miss일 때만; 성공 시 반드시 persist
4. 새 market-data 기능 추가 시 AGENT_STATE 표에 행 추가 후 구현
5. **Redis 장애 시**: API 500 금지 — nil-safe read + 기존 MySQL persist / Toss backfill / in-memory fallback (apibudget, Toss OAuth token). 이미 fallback 있는 경로에 별도 DB 캐시 레이어 추가하지 않음

## Market data (Phase 0–4)

| Phase | 내용 | 상태 |
|-------|------|------|
| 0–4 | Redis+Poller+SSE+candle_bars | ✅ |
| 5 | Terminal persist MySQL (`011`) | ✅ 07-25 |

## Decisions
- 07-18: 과거 캔들=MySQL; Poller+Redis live
- 07-25: 장 마감 Toss live 금지; cold miss=1× backfill
- 07-25: **체결/호가/가격/detail → MySQL + Redis** (Redis-only 폐기)
- 07-26: Redis required at startup (ping); runtime degraded health + apibudget/Toss in-memory fallback

## TODO
- [ ] Redis Pub/Sub + 다중 BE SSE fan-out
- [ ] Toss 429 `Retry-After` 파싱
- [ ] FE: `source=db` / 장 마감 배지
- [ ] prod migration (goose)

## Last
- 07-26: Market indices batch refresh (FRED 3분, warm-up, sp500/dow/vix fix, API `groups`/`dataKind`) → `docs/MARKET_INDICES_BATCH.md`
- 07-26: FE 계좌 연동 메뉴 비노출 (`ACCOUNT_FEATURES_VISIBLE=false`, `docs/FE_ACCOUNT_FEATURES.md`); BE/storage 유지
- 07-25: Naver/Kakao news apibudget → 일일 cap만 (MinInterval/PerMinute 제거, tick당 키워드 일괄 검색)
- 07-25: Naver 뉴스 → NAVER API HUB (`naverapihub.apigw.ntruss.com`, NCP 인증 헤더)
- 07-25: 뉴스 `stripHTML` → `html.UnescapeString` (&#34;/&#39; 등 숫자 엔티티 디코딩)
- 07-25: MySQL terminal persist (011) + write-through/read chain; AGENT_STATE durable policy
