# Principles

> 저장·캐시·시세·LLM·목록 API·예시 데이터를 **건드릴 때만** 읽는다. 세션 시작 때 읽지 않는다.
> 원칙의 원본은 이 파일 하나다(코드 주석이 여기를 가리킨다). 바뀌면 여기만 고친다.

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
| `stamps` | 사용자 스탬프(불변 스냅샷 + **예상 주가·만기**) | 폼 제출 시 1회. 스냅샷·`target_price`·`due_at`는 이후 UPDATE 금지(5분 유예 제외). 수정 가능: `note`·`tags`·`conviction` |
| `holding_trades` | 매수/매도 이력(원본) | 사용자가 기록 추가·삭제할 때. `user_holdings`의 수량·평단은 여기서 파생된 캐시 |
| `user_cash_balances` | 수동 입력 현금(KRW·USD) | 사용자가 보유종목 레일 `현금` 카드에서 저장 |
| `earnings_events` | 실적 발표 이벤트 — 국내 FnGuide 잠정실적발표(컨센서스 `_E` + 발표치 `_P`), 해외 Finnhub 캘린더(EPS·매출 추정/실적 + 장전·장후) | 국내: 시즌 10:00–20:00 /2h (20시만 전월), 비시즌 18:30. 해외: 매일 06:00 KST(−49d~+76d, ~18호출) + 5분마다 어제~오늘(1호출, 전체 창 중 skip). 둘 다 기동 catch-up. **정정은 별 행이 아니라 병합** — 날짜는 이른 쪽(`LEAST`), 수치는 `source_date` 큰 쪽만. **국내 「추정」 발표일은 저장하지 않고 읽을 때 계산**. **해외 어제보다 오래된 「예정」은 DB에 두고 조회에서만 숨김** (`DropStaleOverseasScheduled`) |
| `insider_transactions` | 내부자 거래 — DART 소유보고(국내) · SEC Form 4 비파생 거래(미국) | 해외 07:00 · 국내 20:00 KST (`worker-filings`). 해외는 accession 증분이라 평시 종목당 1~2호출 |
| `financial_periods` | 분기 재무 — 재무제표 상세 모달과 실적 탭 「분기 실적」이 **같은 표**를 읽는다 | 해외 09:00 · 국내 22:00 KST. **`basis`로 분기값·누적값을 둘 다 담는다** — 국내는 DART가 「반기」·「연간」 누적으로 보고하고 실적 탭이 그대로 쓰는데, 상세 모달은 4분기−3분기누적으로 분기값을 만든다. 한 축으로 합치면 한쪽 화면 숫자가 조용히 바뀐다 |
| `stock_disclosures` | DART 접수 공시 | 매일 19:30 KST. **종목별이 아니라 날짜별 스윕** — corp_code 없이 부르면 그날 전체가 페이지로 온다(실측 397건/일 = 2~4호출). 종목별로 돌면 4,400호출 |
| `stock_warnings` | 투자주의·경고·거래정지 등 지정 상태 | 매일 01:30 KST **전 종목**(약 17,900 · 2시간). 종목별 **교체**다 — upsert만 하면 해제된 지정이 영원히 남는다. Toss 초당 5회 상한(250ms 간격) |
| `symbol_sync_state` | 위 3종 + 내부자의 종목별 적재 상태(kind로 구분) | 배치가 매 종목마다 기록. 대상 선정(등급제)과 "수집 전 vs 내역 없음" 구분이 전부 여기서 나온다 |
| `stock_aliases` | 종목 별칭 — `kind='brand'`(한글 읽기→영문 표기, 「에스케이」→sk)와 `kind='symbol'`(한글 별칭→심볼, 「구글」→GOOGL). **DB 어디에도 없는 한글 읽기**를 채운다 | 사람이 넣는다(배치 없음). 씨앗 198행은 migration 070이 넣었고 원문은 `stock_alias_seed.go`(시험이 두 벌의 동일성을 센다). 읽기는 **10분 TTL 프로세스 스냅샷** — 기사 제목마다 훑는 자리라 SQL을 넣으면 느려진다. DB 실패 시 씨앗으로 버틴다 |
| `news_briefings` | 뉴스 화면 AI 시장 브리핑(시장별 이력) | 스케줄러 — **장중 30분 · 그 외 1시간**, 직전 실행 이후 새 기사 0건이면 생략. `/admin/batch` 수동 실행은 무조건 생성 |
| `journal_notes.price_at_write` | 메모 작성 시점 종목가(+`price_date`·`tags_json`) | **저장 시 1회**. 이후 UPDATE 금지 — 본문·태그만 수정 가능 |
| `user_price_targets.base_price` | 목표가 등록 시점 주가(+`base_date`) | 최초 등록 1회(`COALESCE`로 재등록 시 보존). `stop_price`·`target_price_2`는 수정 가능 |

Migration: `011_quote_terminal_persist.sql` (auto via `MigrateExtendedSchemaIfNeeded`); `041_user_cash_balances.sql` (auto via `MigrateUserCashBalancesIfNeeded`); `042_news_briefings.sql` (auto via `MigrateNewsBriefingsIfNeeded`); `047_journal_note_snapshot.sql` / `048_price_target_levels.sql` / `049_research_thumbnail.sql` (auto, 컬럼 존재 여부가 마커 + `isSchemaRaceError` 무시); `052_insider_transactions.sql` (auto via `MigrateInsiderTransactionsIfNeeded`); `053_reference_ingest.sql` (auto via `MigrateReferenceIngestIfNeeded`); `054_drop_broker_linking.sql` (auto via `MigrateDropBrokerLinkingIfNeeded` — 토스 계좌 연동 테이블 DROP)

## LLM 지출 규칙 (모든 LLM 기능 공통)

호출 진입점은 **예약 배치와 관리자 수동 실행 둘뿐**이다. 일반 조회는 캐시가 아무리 낡았어도 LLM을 부르지 않는다 — 하루 비용이 방문자 수가 아니라 배치가 계획한 건수와 같아야 예산을 지킬 수 있다. 해당 기능: 종목 AI 요약(`stock_summary`), **뉴스 시장 브리핑**(`news_briefing`). 해외 뉴스 제목 번역은 Papago만 쓴다.

한도는 `apibudget` 공유 — 일 500콜 / 100만 토큰(env로 조절). 두 시장 브리핑이 연달아 나가면 제공자 최소 간격(4초)에 걸려 두 번째가 규칙 기반으로 떨어지므로 스케줄러가 시장 사이에 간격을 둔다.

**503(용량 부족)은 한도에 잡히지 않는다** — 그래서 시장 브리핑은 503이면 대체 모델(`OPENAI_FALLBACK_MODEL`, 기본 gemini-3.5-flash-lite)로 한 번, 그리고 슬롯 안에서 5분마다 다시 묻는다. 우리 예산은 503을 세지 않지만(Record는 HTTP <400만) **Google 무료 한도가 503을 세는지는 문서에 없어** 시장별 브리핑 날(개장 준비 시각 기준)당 재시도 60회 상한 — 최악에도 키당 모델별 60요청. 대체 모델은 브리핑만 쓴다(대량 배치가 붙으면 그 모델의 무료 한도를 브리핑 몫까지 먹는다).

**LLM 버킷의 하루는 태평양 자정에 시작한다**(다른 provider는 KST 자정 그대로). 상대가 그렇게 세기 때문이고, 우리 헤드룸이 상대의 잔여와 다른 숫자를 가리키면 상한을 올릴 수 없다 — `apibudget.quotaZone`. 국내 배치는 그 경계 직후에 돌므로 **뒤따르는 23시간치 브리핑이 같은 창에 들어간다.** 배치 상한(350)이 아니라 `TokenReserve`(200K)가 그 몫을 지키는 안전판이다.

## Read chain (모든 terminal 데이터)

```
Detail: quotes:terminal (1s hot) → MySQL quote_snapshots → Toss refresh → terminal + MySQL
Book/Trades: Redis envelope (freshness check) → MySQL → Toss 1× backfill → persist envelope + MySQL
```

Legacy `quotes:detail:*` Redis keys purged on startup; detail is terminal + MySQL only.

## Quotes 데이터 정책

| 데이터 | Toss enabled | Toss disabled | persist |
|--------|--------------|---------------|---------|
| 캔들 | Poller→Toss→DB+Redis | DB tail | `candle_bars` |
| detail | Poller+Toss (연속) | DB last-good | `quote_snapshots` (Redis `quotes:detail` 제거) |
| 체결/호가 | Poller+Toss (연속) | DB last-good | `trade_ticks`, `orderbook_snapshots` |
| 목록 가격 | Toss batch | DB `symbol_last_prices` | 동일 |

## 개발 원칙 (크리티컬 데이터)

1. **사용자에게 보이는 마지막 상태**는 Redis만 두지 않는다 → MySQL write-through
2. **Redis는 가속/폴링 TTL**용; cold start는 MySQL에서 복구
3. **Toss 1× backfill**은 DB·Redis 둘 다 miss일 때만; 성공 시 반드시 persist
4. 새 market-data 기능 추가 시 이 문서의 Durable storage 표에 행 추가 후 구현
5. **Redis 장애 시**: API 500 금지 — nil-safe read + 기존 MySQL persist / Toss backfill / in-memory fallback (apibudget, Toss OAuth token). 이미 fallback 있는 경로에 별도 DB 캐시 레이어 추가하지 않음.
   **폴백이 있다는 것으로 충분하지 않다 — 제때 닿아야 한다**(2026-09-09 실측: 폴백은 멀쩡했는데 종목 요약이 502를 10초 걸려 돌려줬다. 전역 요청 예산이 먼저 끝난 것이다). 그래서 Redis 클라이언트에 **짧은 타임아웃(500ms)과 회로 차단기**가 붙어 있다(`repository/redis_breaker.go`). 새 Redis 경로를 만들 때 폴백만 확인하지 말고 **Redis를 실제로 내려 보고 응답 시간을 재라.**

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

## 「내 투자」 빈 화면 · 예시 데이터 (08-16)

5개 메뉴(내 포트폴리오·관심 종목·스탬프·메모장·목표가)는 **로그인 + 0건**이면 카드가 전부 "없습니다"만 말했고, **비로그인**이면 로그인 패널만 떴다. 둘 다 이 메뉴가 무엇을 해주는 곳인지 알려주지 못한다.

- **예시 데이터는 서버가 준다** — `internal/sample` 고정 픽스처 + **인증 밖** `GET /api/v1/sample/*`. 이 화면들의 진짜 엔드포인트가 전부 로그인 뒤에 있어, 공개 경로 없이는 비로그인 방문자에게 보여줄 방법이 아예 없다. 데모 계정을 DB에 심지 않은 이유는 그 행이 모든 집계·배치·관리자 화면에 섞이기 때문
- **버튼을 눌렀을 때만** 채운다(`?sample=1`, URL에 두어 새로고침·뒤로가기·공유가 된다). 기본은 안내 카드
- **비로그인도 같은 안내 카드**를 본다 — `AuthRequiredShell`에 `menu` 키를 주면 로그인 패널 대신 0건 회원과 **같은 컴포넌트·같은 문구**를 쓰고 버튼만 「로그인하고 시작하기 / 예시로 둘러보기 / 회원가입」으로 바뀐다. 설명이 필요한 쪽은 오히려 아직 안 써 본 사람이다
- 아이콘은 `components/common/menu-icon.tsx` 한 곳 — 로그인 전후로 다른 그림이 뜨면 같은 메뉴를 두 번 처음 만나게 된다
- 문구는 `lib/menu-copy.ts` 한 곳 — 헤더 한 줄과 빈 화면 안내가 같은 약속을 해야 한다
- **예시 모드에서는 실시간 시세를 덮어쓰지 않는다.** 평단·목표가가 픽스처 값에 맞춰져 있어 현재가만 실제로 갈아끼우면 평가손익이 +231% 같은 숫자가 된다
- `--sample` 보라색 토큰 신설 — 빨강·파랑은 이 앱에서 상승·하락이라 예시 표시에 못 쓴다
- **08-18 race 수정**: `useSampleMode`가 `useSearchParams`만 쓰면 `?sample=1` 직후·새로고침 첫 프레임에 sample=false로 남아 **인증 API**를 치고 빈 화면·오류가 났다 → `useSyncExternalStore`로 `window.location.search` 동기 읽기 + fetch 직전 `isSampleModeActive()` 재확인. `useSearchParams` 쓰는 「내 투자」 페이지는 `<Suspense>`로 감쌈

## 응답 속도 원칙 (08-17 확립)

출시 전 실측으로 찾은 병목은 전부 같은 모양이었다 — **한 요청 안에서 항목마다 같은 일을 다시 한다**. 새 목록 API를 만들 때 이 넷을 먼저 본다.

1. **항목당 I/O 금지.** 목록을 만들며 기사·종목마다 Redis GET이나 DB 조회를 하면 항목 수만큼 왕복이 곱해진다(뉴스 목록 한 요청에 Redis 342회, portfolio 조회 200회였다). 호스트·심볼처럼 종류가 적은 값은 프로세스 메모리에 이고, 관심·보유 목록처럼 느리게 바뀌는 값은 TTL 캐시를 둔다
2. **조회 경로에서 외부 HTTP 금지.** `ResolveEmbedPolicy`가 캐시 미스마다 남의 사이트를 8초 타임아웃으로 받아 왔다. 지금은 배경으로 조사하고 이번 응답은 unknown으로 답한다
3. **항목당 전수 스캔 금지.** `extractUSSymbols`가 기사마다 이름 인덱스 4만 항목을 훑어 요청 하나에 800만 회였다. 인덱스가 그대로면 같은 본문은 같은 답이므로 **세대(gen) + 본문**을 키로 메모한다
4. **목록 응답에 상세용 데이터를 싣지 않는다.** 리포트 목록이 건당 200봉 OHLCV(19KB)를 보냈는데 카드는 종가만 그렸다. 목록은 `closes`(60점), 상세는 `bars`

5. **만료가 곧 대기가 되지 않게.** 방문이 드문 서비스에선 TTL 만료·무효화 직후의 「첫 요청」이 거의 모든 요청이다. 「신선한 시간」과 「사는 시간」을 나눠 낡은 값을 주고 뒤에서 만들고(SWR), 무효화하는 쪽이 곧바로 다시 채우고(적재 직후 예열), 주기 재구성은 내용이 같으면 메모를 비우지 않는다(09-25 후속2)

측정은 추측보다 싸다. `curl -w "%{time_total} %{size_download}"` → 느린 엔드포인트 특정 → `EXPLAIN` / `redis-cli monitor` / limit별 스케일링으로 원인 분리. 「limit을 줄여도 안 빨라지면 고정 비용(집계 쿼리), 비례하면 항목당 비용」이 가장 빨리 갈리는 지점이다.
