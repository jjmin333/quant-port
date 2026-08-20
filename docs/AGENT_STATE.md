# Agent State

> **세션 시작: 이 파일만 읽기. 종료: 이 파일만 갱신.** 상세 이력 → `docs/archive/sessions.md`

## Now
- **08-21**: **v0.9.0 — 뉴스 브리핑 LLM 입력.** 실적 압축(국내 확정 6 / 해외 ±7일 10, 서버 `imp`)·국내 Toss 수급(억원, 5분 캐시)·FRED DGS10(`us10y`). 이력 10→3 + 직전 watchPoints. 헤드라인 40 점수순(속보·언급종목 우선, 재산공개/팟캐스트 감점). 페이지 로드 LLM 0. 사전할당 토큰은 이력 축소로 호출당 약 −530. Docker Tesseract OK, 지수 API에 us10y=4.65. `docs/releases/v0/v0.9/v0.9.0.md`
- **08-21**: **어제보다 오래된 해외 「예정」은 조회에서 숨김.** `DropStaleOverseasScheduled` — US+scheduled+날짜 < KST 어제. 8/20은 시차로 유지. DB·적재는 그대로(실적 오면 확정). 배지 신설·날짜 +1 안 함. BATCH_SCHEDULE `earnings_overseas` 메모·절 추가
- **08-21**: **해외 실적일은 DB에서 KST로 밀지 않는다.** 23:55에 실적 없는 행을 +1일 하면 AMC(아직 안 남)와 소형주 공백이 같은 신호가 된다. 저장은 미국 세션일 유지. 한국 「오늘」에 장후가 빠지는 문제는 조회/표시에서 amc만 +1일(읽기 전용)이 후보
- **08-21**: **해외 실적 발표일은 KST 환산이 아님.** Finnhub `date`는 미국 세션 달력일(YYYY-MM-DD, TZ 없음). 우리는 그 문자열을 DATE로 그대로 저장·표시. KST를 쓰는 것은 조회 창·배치 시각(06:00, 라이브=KST 어제~오늘)뿐. AMC 8/19 US = KST 8/20 아침인데 화면은 8/19
- **08-21**: **해외 실적 TGT 중복.** Finnhub가 같은 날·같은 분기를 회계연도만 다르게 2행(TGT 8/19 = 2026 2Q 실적만 + 2027 2Q 컨센서스). unique key에 연도가 있어 DB에 둘 다 저장됨. 의미 없는 중복. 적재 시 `(symbol, date, quarter)` 병합(컨센서스 있는 연도 우선), 조회 시 `CollapseOverseasFiscalYearDup`. 2Q+FY 같은 날은 유지
- **08-21**: **Finnhub 실적은 시계열이 아님.** 캘린더 필드는 date+hour(bmo/amc/dmh/공백)뿐, 시각 타임스탬프 없음. 5분 폴링은 Finnhub가 숫자를 넣은 뒤 최대 5분 지연이지, 회사 발표 시각과 동기 아님. NVDA 다음 일정 11/17 `amc`만
- **08-21**: **해외 실적 표.** 매출을 실적만 쓰지 않고 EPS와 같이 「실적 / 예측」. 기본 정렬을 날짜가 아니라 시가총액 내림차순(국내와 동일)
- **08-21**: **중·대형주도 Finnhub 누락 있음.** 8/12–19 과거일 기준 ≥10B 예정 잔류 2건(ROST 76B 날짜 8/19 vs 실제 8/20, PS 16B 8/12 발표인데 8/13 예정·실적 null). 중형(≥2B) 15건 — XP는 8/17 발표·8/18 잔류, VIPS·BZ는 실제 8/25인데 추정일이 이미 지나 예정으로 남음. 소형 전용 현상 아님. ADI급은 대체로 채워짐
- **08-21**: **차선 3소스 필드 대조.** Finnhub 3종(시각·EPS실적/예측·매출실적/예측) 기준. Nasdaq JSON: EPS 실적+예측 있음, 매출 필드 없음, `time`은 8/19 58건 중 1건만 채워짐
- **08-21**: **Finnhub 소형주 공백 + 무료 차선.** 대형주만 막는 규칙은 아님. 8/19는 실적 없는 행 시총 중앙값 2.9억 vs 있는 행 24억. Nasdaq은 4종 실제일에 EPS 있음. SEC 8-K 2.02도 있음
- **08-21**: **Finnhub는 4종의 Q2 실적치를 어디에도 안 줌.** 캘린더 8/11·12·13에 AARD/ACRV/ADGM/ALMS 0건, 8/19만 추정+null. `/stock/earnings`·`/stock/financials-reported` 최신도 전부 2026 Q1. ADI는 같은 API에서 Q3 실적 있음
- **08-21**: **스크린샷 8종 실제 발표일.** 예정 4종은 8/19 발표가 아님 — AARD 8/11, ACRV 8/12, ADGM 8/11, ALMS 8/13(IR·8-K). 확정 4종은 8/19가 맞음
- **08-21**: **해외 실적 「과거인데 예정」.** 화면/적재 버그 아님. `dateStatus`는 날짜가 아니라 Finnhub `epsActual`·`revenueActual` 유무. 8/19 API 64행 중 예정 37·확정 27, 라이브 Finnhub도 실적 0건 동일(lag 0). 5분 라이브는 KST 어제~오늘만(8/21 00:05에 95→36행으로 8/19 이탈)이나, 지금은 소스가 비어 06:00 전체창을 돌려도 안 채워짐
- **08-20**: **배치 시간표에 메뉴 열.** `BATCH_SCHEDULE.md` 시간표가 시각 | 메뉴 | 잡 …. 사이드바 이름(국내·해외 실적 일정 등). 종목 탭은 「종목 · 탭」
- **08-20**: **배치 시간표 메모 + 관리자 편집.** `BATCH_SCHEDULE.md` 시간표 맨 오른쪽 메모 열. 관리자 「배치 관리」가 `PUT /api/v1/admin/batch/schedule`로 같은 파일을 저장(디스크 우선, embed 폴백). Docker는 파일을 바인드 마운트
- **08-20**: **국내 실적 배치.** 시즌 10·12·14·16·18·20 KST. 오늘 16:00까지 성공(273행 upsert). 18시 발표는 같은 슬롯 레이스면 20:00. 저장 경로는 정상
- **08-20**: **토스 계좌 연동 제거.** 시세/캔들/랭킹/경고용 글로벌 Toss Open API는 유지. 제거: OAuth 키 연동, `/broker`·`/portfolio`·`/insights/dashboard`·`/accounts`·`/orders`·브로커 `/holdings`, 인사이트 LLM 스케줄러(유저 수 비례). 폴러·뉴스 심볼 추적은 관심종목 + 수동 `user_holdings`. 마이그레이션 `054`가 연동 테이블 DROP. FE 플래그 `ACCOUNT_FEATURES_VISIBLE` 삭제, 옛 URL은 `/`로 리다이렉트. `/holdings` OCR·수동은 유지
- **08-20**: **실적 홈·종목탭 설계.** 캘린더 적재는 충분. 빈 칸은 소스 부재(국내 예정일, 8/17– 국내 0건)보다 **이미 있는 서프라이즈·반응·적중을 홈/탭이 안 그림** + 해외 −45일 백필 공백. 유료 소스보다 preview·탭 히어로·해외 2년 1회 백필이 먼저
- **08-20**: **LLM 사용처 2종.** 종목 AI 요약(레이더 상위, KR+US ≈210/거래일)·뉴스 시장 브리핑(장중 30분/그외 1시간×2시장, 새 기사 있을 때만). 홈 포트폴리오 브리핑 LLM은 제거. **지수는 규칙 문장, LLM 0.** 번역은 Papago만
- **08-20**: **Gemini 503 1회 재시도 + 번역은 Papago만.** 공식 503=용량(쿼터 아님), 로컬 Record는 HTTP<400만. 재시도 1회·4초·429 제외. 제목 번역 Gemini 폴백 제거, papago_text 분당 20→60(NCP는 글자 과금·RPM 공표 없음). 일 6000은 유지
- **08-20**: **이전 실패 10건 재처리 없이 판정.** 새 파서가 살리는 건 JSON 꼬리 `}` 2건(EHGO·AEON)뿐. 원문 completion은 로그에 없어 과거 기록을 성공으로 고칠 수는 없음. 503 7건·간격 1건은 동일 응답이면 여전히 실패
- **08-20**: **LLM JSON 파싱 — 꼬리 `}` 복구.** Gemini가 완전한 객체 뒤에 `}`를 붙이면 `json.Unmarshal`이 전체를 버려 EHGO·AEON이 실패했음. `decodeLLMJSON`이 첫 값만 Decoder로 읽고 펜스·선행 문구도 건너뜀. 재호출 없음. 종목 요약·브리핑·뉴스 브리핑 공통. 유닛 통과, Docker 재기동(Tesseract OK)
- **08-20**: **AI 요약 수동 배치 실패 10건 분석.** KR 08-20 105/108 · US 08-19 107/114, 둘 다 stop=completed. 실패는 재시도 없이 기록되고 carry-over에도 안 실림. 원인 3종: Gemini 503 high demand 7건(KR 실리콘투·알트·LG이노텍 / US SPCX·LITE·TGT·LRCX), 응답 JSON 꼬리 `}` 파싱 2건(EHGO·AEON), 공유 버킷 4초 간격에 번역이 끼어든 CAT 1건. JSON만 코드 수정
- **08-19**: **국내 실적 시즌 폴링 C안.** 시즌 10–20시/2h · 비시즌 18:30. 국내(FnGuide)와 해외(Finnhub)는 버킷이 달라 **기동 catch-up을 동시에** 돌리고, 같은 잡의 정규 슬롯만 catch-up 뒤에 이어진다. `main()`은 어느 쪽도 기다리지 않음. 실측 8/14 DB 6→88, 달력 87 confirmed
- **08-19**: **LLM 500 소진 내역** — 지수는 LLM 0(규칙). 랭킹은 국내만이 아니라 KR+US ≈200콜/일. 국내 기사는 건당 LLM이 없고 시장 브리핑(국내+해외, 장중 30분)만. **숨은 소비는 해외 제목 번역**: 신규 US 기사마다 Papago 20/min 초과분이 Gemini로 넘어감(8/18 Papago 3,631 · openai 카운터 500). 설계 예비 80콜로는 야간 번역을 못 막음. KST 일자로는 282가 500 안이지만 PDT 일자에는 전날 저녁 US 배치+밤새 번역+아침 KR 배치가 한 날에 겹침. 미수정
- **08-19**: **조회 경로에서 외부 HTTP 호출 0.** 내부자·분기재무·공시·종목경고·인사이트 실적을 전부 배치 적재로 옮겼다. `worker-filings`가 6개 잡을 돌리고 조회는 표만 읽는다. 시세/호가/체결만 예외(폴러 구조가 따로 있음). **API 프로세스의 `MarketDataStore.Run`도 이제 달력만 갱신한다** — 실적·경고·공시 배경 폴링은 전부 워커로 갔다
- **08-20**: **배치 시간표 = `be/docs/BATCH_SCHEDULE.md`.** 새 배치를 추가할 때 이 문서를 먼저 고친다. 관리자 「배치 관리」가 `GET`/`PUT /api/v1/admin/batch/schedule`로 읽고 저장한다. 디스크 파일이 우선이고 embed는 파일이 없을 때의 폴백. 시간표 맨 오른쪽은 운영 메모 열
- **08-19**: 종목 단위 적재 배치의 대상 선정·진행 상태는 전부 `symbol_sync_state`(kind로 구분) 하나가 맡는다. 등급제 = 관심·보유 → 미수집 → 오래된 순. `warnings_sync`만 전 종목 매일(staleAfter=0). **게이트(`providerGate`)는 실제 HTTP 하나만 감싼다 — 재진입하면 교착한다**
- **08-19**: AI 추천 랭킹 「실시간 시세」버튼 — `/market`으로 떠나지 않고 **일봉 차트 모달**. `/quotes` 개요와 같은 `QuoteDailyChart`(캔들·실시간 오버레이). 열 때만 시세·캔들 구독. 푸터에서 기존 실시간 차트로 나갈 수 있음
- **08-19**: AI 추천 랭킹(`/analysis`) 기본 종목 — URL `?symbol=` 없을 때 **항상 005930(삼성전자)**. 예전엔 포트폴리오 최대 보유 종목(예: ARM)으로 덮어써서 기본 화면이 달라졌음
- **저장 정책 (07-25 확정)**: **MySQL = source of truth**, **Redis = hot cache** (유실 방지)
- **Redis 장애 (07-26)**: **기동 시 Redis 필수** (ping 실패면 BE 시작 불가). 런타임 Redis down 시 `/ready`는 `degraded` (200); apibudget/Toss token은 in-memory fallback
- **Market indices (07-26)**: provider **batch refresh** + FRED **3분** + warm-up + EOD persist → `docs/MARKET_INDICES_BATCH.md`
- Quotes: `candle_bars` + `quote_snapshots` / `orderbook_snapshots` / `trade_ticks` / `symbol_last_prices`
- 장 마감: DB-first → Redis warm → (miss) Toss backfill → **MySQL + Redis write-through**
- Poller: **Toss enabled 시 세션 무관 연속 live poll**; 성공 시 항상 MySQL upsert

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
| `news_briefings` | 뉴스 화면 AI 시장 브리핑(시장별 이력) | 스케줄러 — **장중 30분 · 그 외 1시간**, 직전 실행 이후 새 기사 0건이면 생략. `/admin/batch` 수동 실행은 무조건 생성 |
| `journal_notes.price_at_write` | 메모 작성 시점 종목가(+`price_date`·`tags_json`) | **저장 시 1회**. 이후 UPDATE 금지 — 본문·태그만 수정 가능 |
| `user_price_targets.base_price` | 목표가 등록 시점 주가(+`base_date`) | 최초 등록 1회(`COALESCE`로 재등록 시 보존). `stop_price`·`target_price_2`는 수정 가능 |

Migration: `011_quote_terminal_persist.sql` (auto via `MigrateExtendedSchemaIfNeeded`); `041_user_cash_balances.sql` (auto via `MigrateUserCashBalancesIfNeeded`); `042_news_briefings.sql` (auto via `MigrateNewsBriefingsIfNeeded`); `047_journal_note_snapshot.sql` / `048_price_target_levels.sql` / `049_research_thumbnail.sql` (auto, 컬럼 존재 여부가 마커 + `isSchemaRaceError` 무시); `052_insider_transactions.sql` (auto via `MigrateInsiderTransactionsIfNeeded`); `053_reference_ingest.sql` (auto via `MigrateReferenceIngestIfNeeded`); `054_drop_broker_linking.sql` (auto via `MigrateDropBrokerLinkingIfNeeded` — 토스 계좌 연동 테이블 DROP)

## LLM 지출 규칙 (모든 LLM 기능 공통)

호출 진입점은 **예약 배치와 관리자 수동 실행 둘뿐**이다. 일반 조회는 캐시가 아무리 낡았어도 LLM을 부르지 않는다 — 하루 비용이 방문자 수가 아니라 배치가 계획한 건수와 같아야 예산을 지킬 수 있다. 해당 기능: 종목 AI 요약(`stock_summary`), **뉴스 시장 브리핑**(`news_briefing`). 해외 뉴스 제목 번역은 Papago만 쓴다.

한도는 `apibudget` 공유 — 일 500콜 / 100만 토큰(env로 조절). 두 시장 브리핑이 연달아 나가면 제공자 최소 간격(4초)에 걸려 두 번째가 규칙 기반으로 떨어지므로 스케줄러가 시장 사이에 간격을 둔다.

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

> **출시 전 점검서: `docs/PRE_LAUNCH_OPTIMIZATION.md` (08-19)** — 실측 기반 P0/P1/P2. P0 6건(레이트 리밋 없음 · 요청 타임아웃 없음 · Redis `noeviction` · 8프로세스×50 커넥션 · 마이그레이션 8중 동시 실행 · FE 배포 형태 없음)이 배포 차단 항목이다.

- [x] `candle_bars.bar_time` 시간대 정규화 — 08-08 완료 (쓰기 경로 + 149,803행 보정)
- [x] 토스 계좌 연동 제거 — 08-20. 시세 Toss는 유지. 연동 LLM·동기화·FE 메뉴 삭제 (`054_drop_broker_linking.sql`)
- [ ] 점수: 섹터 컬럼 추가(DART `induty_code` / SEC `sic` — 내부자 조회 응답에 이미 포함) → 피어그룹 교체
- [ ] 점수: 예측력 검증(score(D) vs return(D+1) IC) — 40거래일 축적 후
- [x] 일별 종가 전 종목 백필 — 08-08 완료. 커버리지 1,469 → **16,225/17,901** (436,199행). 잔여 1,676종목은 재실행으로 해결 불가(미국 우선주 `^`/`.` 티커가 Toss 캔들 API `400 invalid-request`, 소수는 상장폐지 `404`)
- [ ] btc·gold 지수 이력 백필 — 날짜 있는 소스 없음(Upbit/goldprice 이력 API 확인 필요). 오늘부터는 일별 배치로 누적
- [ ] usd-krw 지수 이력 — Toss 지표캔들이 `USD/KRW` 심볼에 `400` 반환, 다른 심볼/소스 확인 필요
- [ ] Redis Pub/Sub + 다중 BE SSE fan-out
- [x] Toss 429 `Retry-After` 파싱 — 08-06
- [ ] FE: `source=db` / 장 마감 배지
- [ ] prod migration (goose)
- [~] 추적 심볼 캐싱 — 08-17에 **호출자 쪽**(`NewsStore.trackedSymbols`, 1분 TTL). 08-20부터 관심+수동보유만 (브로커 포트폴리오 경로 제거)
- [ ] 폴러 리더 선출/샤딩 — 다중 BE 인스턴스 대비
- [x] 계좌/잔고 조회 사용자수 비례 한계 — 08-20 토스 계좌 연동 제거로 해당 경로 없음

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

측정은 추측보다 싸다. `curl -w "%{time_total} %{size_download}"` → 느린 엔드포인트 특정 → `EXPLAIN` / `redis-cli monitor` / limit별 스케일링으로 원인 분리. 「limit을 줄여도 안 빨라지면 고정 비용(집계 쿼리), 비례하면 항목당 비용」이 가장 빨리 갈리는 지점이다.

## Last
- 08-20: **토스 계좌 연동 제거** — 시세 Toss 유지. 브로커 포트폴리오·인사이트 LLM·주문 API 삭제. 추적 심볼=관심+수동보유. `054` DROP. `extendedMigrations`에서 `007` 재적용 제거(없으면 매 기동 CREATE→DROP). FE tsc PASS, `go test ./...` PASS, Docker healthy, Tesseract OK
- 08-19: **국내 실적 C안 구현** — 시즌 10/12/14/16/18/20시 당월, 20시만 전월. 비시즌 18:30. 기동 catch-up(국내 동기·해외 비동기). 배포 후 8/14 6→88행, 달력 87 confirmed. 17–19는 소스 0건
- 08-19: **LLM 지금 차단 여부** — 현재(22:38 KST) Google 차단 아님(`blocked_until` 없음, 마지막 성공 22:30 브리핑). 아침 07:46만 PDT 전날 500으로 429. 16:00 KST 리셋 후 US 92 성공. 우리 KST 오늘 174/500. 대시보드 분당 바는 호출 없는 분이면 0
- 08-19: **LLM 500 내역** — 지수 LLM 0. 랭킹 KR+US ≈200. 뉴스 LLM은 기사 단위가 아니라 시장 브리핑 2시장. 실제 초과분은 해외 제목 번역(Papago 20/min 초과 → Gemini). 8/18 KST openai 500인데 US 랭킹 배치 0·KR 기록 없음 → 번역+브리핑만으로 500. PDT/KST 일자 불일치로 아침 KR이 전날 쿼터에 부딪힘. 미수정
- 08-19: **LLM 요약 공백 조사** — 화면이 비는 게 아니라 Gemini 무료 500 RPD 소진. KR 배치 1/106 llm(005930), SK하이닉스 포함 나머지는 `source:rule`. 우리 카운터(KST)와 Google 카운터(PDT)가 어긋나 국내 배치가 한도 밖에서 출발. US 92/96은 리셋 이후. 번역은 2분마다 24건을 간격 없이 밀어 4s 버킷을 태움. 미수정
- 08-19: **국내 실적 달력 14일 이후 공백 조사** — 마이그레이션(8/17 수동 2,791행) 이후 스케줄은 8/18 18:30 한 번(358행). 오늘 워커가 19:24에 재시작돼 18:30을 건너뜀 — `daily_close`는 catch-up이 있고 `worker-earnings`는 없다. FnGuide 라이브 8월은 14일이 마지막(89건, 거의 개별)인데 DB는 6건. 17·18일은 소스에도 없음. 제안: 기동 catch-up + 시즌 중 당월 재폴링(별도 결과 API는 없음, `_E`/`_P`가 같은 월 응답). 미구현
- 08-19: **펀더멘털 배치 vs 실적/재무 탭** — `stock_fundamentals` 9,993행(EDGAR 7,121). 미국 fetched_at은 **07-20 04:32가 마지막**(시드 ~4h46m). 24h 폴러 로그 없음. 실적 분기표·재무제표 차트는 이 표가 아니라 탭 열 때 companyfacts(Redis 24h)
- 08-19: **EDGAR 사용처 전수** — URL 4종(`company_tickers`·`companyfacts`·`submissions`·Form4 XML). 재무/실적/이력은 같은 companyfacts를 각자 호출. 내부자만 submissions+XML. 8/min은 SEC(10/s)가 아니라 자체 예산
- 08-19: **EDGAR 분당 8회는 우리 한도** — SEC 공식은 **초당 10회**(fair access). `apibudget` 8/min이 Form 4 XML 상한 3의 원인. 내부자 공백(CRWV)은 SEC 한도가 아니라 자체 예산+Table I만 파싱
- 08-19: **CRWV 내부자 탭 공백 원인 조사** — SEC는 Form 4 **431건**(최근 1000건 중) 제공. 우리는 조회 시 EDGAR를 **온디맨드**로 부르며 MySQL 적재/폴링 없음. 최근 3건만 파싱+보통주 Table I만 읽어, 최신 3건이 Class B 증여·워런트 매도(Table II)라 0건 → Redis `insider:tab:us:v1:CRWV=[]` 24h 캐시. 바로 다음 신고(8/14 Magnetar)에 Class A 매도 24건이 있음
- 08-19: **AI 추천 랭킹 실시간 시세 모달** — 「실시간 시세」가 `/market`으로 화면을 떠나 랭킹 맥락이 끊기던 것을 일봉 캔들 모달로 바꿈. `/quotes` 개요 차트를 `QuoteDailyChart`로 추출해 모달과 공유. 헤더(실시간가)·일봉·시세 그리드, 열 때만 `useQuoteDetail`/`useQuoteCandles`. 분봉·호가는 푸터 「실시간 차트에서 더 보기」. tsc PASS, eslint 신규 0건. FE only
- 08-18: **일별 종가 2026-01-01~ + 실적 반응 백필** — 국내 `-fetch-history` **14,213/14,217** 성공. 실적 반응 **2월·5월 100%**, 8월 **96%+**. **해외**: `backfill-daily-closes -market=us` + fetch 함수 asset_type 필터 추가, 배치 `daily_close_fetch_history`(market=us) 추가 — **실행 대기**(MySQL80 중지로 CLI 중단, 재시작 후 아래 명령)
- 08-18: **실적 화면 — 정렬 테이블 · 성과형 달력 · 레일 전면 교체** (목업 `docs/mockups/earnings-table-v2.html` → 사용자 선택: 표 확정 · C2 · R1·R2·R3 + R5 변형 · 현재가 열 포함)
  - **카드형 행 → 표.** 예전에는 한 행이 2줄짜리 카드라 세로로 훑는 것밖에 못 했고, 「영업이익 큰 순」·「시가총액 순」으로 줄을 세울 수 없었다. 실적 화면에서 사람이 실제로 하는 일이 그 줄 세우기다. 9열 전부 정렬 가능하고, **칩을 없애고 글자로** 적는다(연결/개별·장전/장후는 배지 대신 작은 글자)
  - **열 구성은 시장이 정한다** — 국내 영업이익 / 해외 EPS + **시간(장전·장후) 열**. investing.com에 있지만 우리가 못 채우는 **시간외거래·EPS 개정**은 빈 열로 두지 않고 **뺐다**
  - **신설 데이터 3종**: ① **시가총액** = `stocks.shares_outstanding` × 최근 종가 — 커버리지 **100%**, 저장하지 않고 읽을 때 곱한다(매일 바뀌는 파생값을 하나 더 만들지 않으려고). 심볼별 MAX(trade_date) 조인 한 방 ② **매출 서프라이즈**(국내 93%·해외 95%) — 이익이 기대를 넘었는데 매출이 밑돌면 비용 절감이지 성장이 아니라는 뜻이라 별개 열로 둔다 ③ **현재가** — 보이는 행 심볼을 모아 `quotes/batch` **1회**(상한 120), 폴링 없음
  - **C2 성과형 달력** — 칸 색이 **그날 상회 비율**을 말한다(50% 위 빨강 / 아래 파랑, 50%에서 멀수록 진하게). 예전 중립 회색 농도는 「많은 날」과 「좋았던 날」을 구별하지 못했다. **발표 전인 날은 색을 칠하지 않는다**(없는 성과를 말하는 셈) — 중립 농도로 건수만. 실측: 64%→빨강 α0.18 · 46%→파랑 α0.11 · 17%→파랑 α0.31
  - **레일 전면 교체** — 「상회/하회 TOP」은 **서프라이즈 탭이 이미 더 길게** 보여주는 목록이라 6줄을 반복하는 셈이었다. 새 넷은 전부 **표에 없는 것**: ① **다가오는 대형 발표**(시가총액 순) ② **기대와 주가가 엇갈린 발표** — 「증권사 예측 → 실제」를 글자로 밝히고 그 아래 주가를 둔다(퍼센트 둘만 나란히 두면 어느 쪽이 실적이고 어느 쪽이 주가인지 알 수 없다) ③ **꾸준히 기대를 넘는 기업**(최근 8분기 적중률, 표본 4분기 이상) ④ **내 종목 최근 발표**(지난 2주~앞으로 한 달 고정 창)
  - `/earnings/highlights` **한 요청**으로 셋을 준다 — 카드마다 부르면 레일 하나에 세 왕복인데 셋이 서버에서 같은 표를 읽는다. Redis 30분
  - **심볼 필터가 있으면 구간 상한을 31→120일로 푼다.** 31일 상한은 스캔을 묶기 위한 것인데 심볼 목록(≤100)이 이미 그 일을 하고 있어 같은 제약을 두 번 걸 이유가 없다 — 레일의 44일 창이 필요해져 드러났다
  - **국내 R1이 빈 카드였다** (실측): 국내 실적이 2·5·8·11월에 몰려 08-18 기준 향후 2주가 0건이다. 2주가 비면 **100일로 넓혀** 「다음에 올 큰 발표」를 보여준다 — 75일로 잡았다가 다음 추정 발표(11-02)가 76일 뒤라 하루 차이로 놓쳤다. 지금은 삼성전자 1,604조·SK하이닉스 1,217조가 뜬다
  - **캐시 버전을 올려야 한다** — 계산이 바뀌었는데 `earnings:highlights:v1` 키가 그대로라 5ms 캐시 히트로 옛 빈 결과가 계속 나왔다. v2로 올려 해결
  - 검증: go build/vet/test·tsc·eslint(신규·수정 파일 0건)·`next build` green, Docker 재기동. **브라우저 DOM 실측** — 9열 헤더·현대해상 「5050억원/3690억원 +36.9% · 시총 3.3조원 · 44,650 −1.43%」, 시가총액 정렬 클릭 시 3.3→1.5조 내림차순, 달력 색 6칸, 레일 4카드(옛 TOP 사라짐). ⚠️ 달력 색을 처음 읽었을 때 전부 회색으로 나왔는데 **`transition-colors` 진행 중 샘플링**이었다 — `transition:none` 후 재측정해 확인. 스크린샷은 여전히 패널 미표시로 불가
- 08-17: **실적 메뉴 국내·해외 분리 + 위젯 4종** (목업 `docs/mockups/earnings-widgets-v1.html` → 사용자 선택: W1~W4 전부 · 월/주/일 토글 · 연결 우선 + 개별 접기 · 사이드바 2항목)
  - **사이드바 「실적」 = 국내 실적 일정 / 해외 실적 일정**(뉴스 그룹과 같은 결), 화면 안 시장 토글 제거. 두 시장은 한 목록에 못 들어간다 — 국내는 예정일 소스가 없어 앞일이 전부 추정이고 지표가 **영업이익**(연결/개별), 해외는 예정일이 정확하고 **발표 시각**이 있으며 지표가 **EPS**다. `/earnings`·`/calendar` → 국내 리다이렉트
  - **W1 발표 후 주가 반응** — 서프라이즈 옆 「↗ +2.4%」. 값어치는 **둘이 엇갈린 줄**에 있다(실측 한국금융지주 +27.8% → −7.3%, 엘앤에프 −58.2% → **+7.3%**) → 「상회·하락」 배지. 세션별로 다른 종가 쌍: **장전(bmo)은 발표 당일**, 장후·국내는 다음 거래일 — 안 나누면 반응을 하루 잘못 읽는다
  - **⚠️ 목업 초안의 「클리오 +28.2%」가 틀린 값이었다** (구현 중 발견). `stock_daily_closes`의 **최근 세션 커버리지가 얇다** — 08-07까지 16,190종목/일인데 **08-10부터 200~700**(candle_bars도 1,017→692). 가드 없이 「다음 거래일」을 찾으면 08-13 발표에 **다음 주** 종가가 잡혀 7일치 등락이 그 이름으로 나간다. `maxReactionGapDays = 5`로 인접성을 강제하고 멀면 값을 내지 않는다 → 화면은 **「집계 중」**(0%로 그리면 "반응이 없었다"는 다른 말). 최근 열흘이 대부분 「집계 중」인 것은 이 기능의 버그가 아니라 **일별 종가 파이프라인의 공백**이다
  - **W2 실적 전용 상세 모달** — 예전엔 범용 시세 모달이 떴다. 헤더(서프라이즈 + 반응 나란히) · 매출/영업이익/순이익 3타일 + **QoQ** · **분기 추이 막대(기대 vs 발표 겹침)** · 적중 이력 · 다음 발표 추정 · **개별 기준 접기**. 목록이 준 값으로 헤더는 즉시, 이력만 열 때 한 번 부른다(실측 보령: 서프 +13.8% · 반응 +2.4% · QoQ 3종 · 막대 4개 · 1/2분기 상회)
  - **W3 상단 스코어보드** — **평균과 중위를 함께** 적는다: 실측 08-06 국내가 중위 **+4.3%**인데 평균 **−3.3%**였다(LG헬로비전 −69.7% 한 건이 끌어내림). 하나만 보여주면 그날 시장을 반대로 읽는다
  - **W4 해외 장전·장중·장후·미공개 타임라인** — 해외에서만 가능하다(국내 소스엔 시각이 없다). 미공개를 따로 두는 이유: 장전·장후 중 하나로 찍으면 거짓말이다
  - **`CollapseDomesticBasis` 중복 구현 정리** — 이번 작업에서 같은 일을 하는 `ConsolidateEarnings`를 새로 만들었고, `Calendar`·`Surprises`에서 **둘이 연달아 돌았다**. 앞선 것이 개별 행을 **버려서** 뒤의 `alternates`가 늘 비었고, 사용자가 고른 「개별은 접어둔다」가 **죽은 코드**였다(API에서 `alternates` 0건으로 드러남). 하나로 합치고 **진 행을 버리지 않고 `Alternates`로** 넣는다. 키에 `Date`가 들어가는 것은 유지 — 같은 분기를 다른 날 두 번 발표한 것은 별개 사건이다. 실측: 미래에셋생명 2026 2Q → 1행(대표 연결 +54.6% · 접힘 개별 −60.8%)
  - **월/주/일 토글** — 실적은 주 단위로 몰린다(8월 국내 190건이 3~14일에 전부). 주간 보드는 월~금 5칸(주말은 영원히 비어 격자 2/7 낭비) + 칸마다 **서프라이즈가 가장 컸던 종목**. 보기 단위는 `lib/earnings-view.ts` + **`useSyncExternalStore`** — 이펙트에서 localStorage를 읽어 setState하면 캐스케이드 렌더고 렌더 중 직접 읽으면 하이드레이션이 어긋난다(`targets-view.ts`와 같은 형태)
  - **기본 날짜를 「데이터가 있는 날」로 자동 이동** (브라우저 실측에서 발견): 국내는 2·5·8·11월에 몰려 **「오늘」이 1년 중 대부분 빈다** — 처음 들어온 사람이 빈 목록과 「—」 다섯 개를 봤다. `days`가 오면 오늘 이전 마지막 유효일로 옮긴다(**파생 값**으로 계산 — 이펙트+setState는 캐스케이드 렌더). **한 번 고른 뒤에는 덮지 않는다**(`dateTouched`) — 고른 빈 날짜를 옮기면 클릭이 안 먹는 것처럼 보인다
  - **보류**: 모달의 「관련 뉴스」 — `news_articles.symbols_json LIKE`가 **0.83초 무인덱스 스캔**(75,522행)이라 모달 열 때마다 못 쓴다. 심볼→기사 인덱스(조인 테이블/생성 컬럼)가 별도 작업으로 필요
  - 검증: go build/vet/test(신규 시험 — 연결 병합 5케이스·적중 이력·스코어보드 하한·세션별 반응·인접성 가드)·tsc·eslint(신규·수정 파일 0건)·`next build`(40페이지) green, Docker 재기동(Tesseract OK). 엔드포인트 하루 2.7KB/0.023s · 달 14.7KB/0.070s · stats 299B/0.005s. **브라우저 DOM 실측** — 국내 자동 이동 8/14(상회 80%·중위 +19.8%), 08-06 전환(중위 +4.3% vs 평균 −3.3%), 보령 반응 ↗+2.4%, 해외 장전·장후·미공개 3레인, 상세 모달 전 섹션. ⚠️ **스크린샷 불가**(Browser 패널 미표시 → compositing 없음)
- 08-17: **실적 캘린더 BE — 연결·개별 중복 행 collapse** — FnGuide가 같은 분기를 `(연결)`·`(개별)` 두 줄로 주는데, unique key `(symbol, fiscal_year, fiscal_period, basis)`라 DB 중복은 아니었다. 시장 캘린더·히트맵·서프라이즈 랭킹이 같은 종목을 두 번 세·그려 React duplicate key(FE)와 목록 2배(8/7 NAVER·삼성증권 실측)를 유발. `model.CollapseDomesticBasis()` — 같은 (symbol, date, fiscalPeriod)면 **연결 우선** — 를 Calendar·Summary·Surprises·EstimatedDomestic(작년 근거)에 적용. 종목 탭 announcements는 개별·연결 둘 다 유지. 검증: go test·Docker 재기동(Tesseract OK), 035420 8/7 API 2행→1행(연결)
- 08-17: **실적 캘린더 FE — React duplicate key 경고 수정** — dev 콘솔에 `Encountered two children with the same key`가 반복(예: `035420-2026-08-07`, `016360-2026-2Q-confirmed`). API가 같은 종목·분기·날짜를 **서로 다른 수치로 두 줄** 내려주는데(NAVER 8/7 실측: 영업이익 4390억 vs 5203억), 목록 key가 `symbol-date`·`symbol-fiscalPeriod-dateStatus`만 써서 충돌. `earningsEventKey()`(수치 앵커 포함)를 `earnings-shared.tsx`에 두고 캘린더 본문·레일·서프라이즈·종목 탭 announcements에 적용. FE만 — Docker 재기동 없음. 검증: tsc·eslint(만진 파일 0건), `/earnings` 8/7 선택 후 콘솔 duplicate key 0건
- 08-17: **실적 메뉴 신설 — 시장 전체 실적 캘린더 + 컨센서스·서프라이즈** (`/earnings`, 사이드바 「실적」 그룹)
  - **앱이 세 곳에서 "없다"고 말하고 있었고, 그 중 하나는 사실이 아니었다.** 종목 실적 탭은 「무료 소스(DART/Finnhub/EDGAR)에서는 컨센서스·서프라이즈를 제공하지 않습니다」를 화면에 적고 있었는데, **Finnhub 응답에 `epsEstimate`/`epsActual`이 처음부터 실려 오는데 파서가 `date`·`symbol`·`quarter`만 읽고 버리고 있었다**(과거 2주 1,446행 중 1,135행에 `epsActual`). 국내는 FnGuide가 추정치·발표치를 함께 준다. `/calendar`는 실체 없이 리다이렉트만 하는 빈 라우트였다
  - **저장** `earnings_events`(`051`, 자동): 한 행 = 한 종목의 한 회계기간. 확정·예정만 저장하고 **국내 추정 발표일은 읽을 때 계산**(저장하면 실제 발표가 들어온 순간 지우러 다녀야 하고, 놓치면 같은 분기가 두 줄로 남는다 — 목표가 도달일과 같은 이유)
  - **국내 예정일은 어떤 소스로도 얻을 수 없다** (사용자 결정: 작년 동기 추정). DART에 예정 API 없고 FnGuide 캘린더도 미래 달은 `{"dataset":[]}`(202609·202610 실측), `IR|실적발표` 타입도 0건. 그래서 「작년 같은 분기 발표일 +1년 → KRX 거래일 스냅」이고, 근거를 `estimateBasis`로 같이 내려 화면이 「추정 · 작년 2Q는 8/3 발표」로 밝힌다. 규칙은 `EstimateNextAnnouncement` **한 곳** — 캘린더와 종목 탭이 다른 날짜를 말하면 안 된다
  - **실측이 다섯 번 설계를 뒤집었다** (전부 추측이 아니라 호출해 보고 알았다):
    ① **단위 = 억원** — 삼성 2025 결산 매출 `3,336,059` ↔ DART 확정치 333,605,938,000,000원(=3,336,059.38억) **6자리 일치**. 저장은 원 단위(×1e8)로 눕혀 `formatFinancialAmount` 하나로 끝낸다
    ② **`_G`는 YoY가 아니라 QoQ** — 삼성 3Q25 매출 86.1조·G=15.4로 역산한 Q2가 74.6조로 실제와 맞았다. 처음엔 "확정 불가라 쓰지 않는다"로 적었던 값
    ③ **`FREQ`에 `"D"`(연간 결산)가 있다** — 2월 발표분 480건이 `D`. 분기 코드만 받으면 결산 시즌을 통째로 놓친다 → `FY`로 정규화
    ④ **`"적전"`(적자전환) 같은 한국어가 숫자 자리에 온다** — `json.Number`가 디코드에서 거부해 **한 행이 그 달 400행을 통째로 날렸다**(202507·202508 유실). `flexNumber`로 숫자·숫자문자열·라벨을 모두 받고 라벨은 없는 값으로 둔다
    ⑤ **Finnhub `/calendar/earnings`는 1,500행 상한이고 잘릴 때 구간의 뒤쪽만 남긴다** — 4개월을 한 번에 부르니 10월 3주치(1,295행)만 들어왔다. **7일 조각**으로 나눠 받아 5,620행·전 구간 확보
  - **정정은 별 행이 아니라 병합** — FnGuide는 정정을 원 발표와 함께 준다(삼성 2Q: 07/07 + `[정정]`07/30). **날짜는 이른 쪽(`LEAST`), 수치는 `source_date` 큰 쪽만** 덮고 `revised_at`을 남긴다. 정정일로 덮으면 "삼성전자는 7/7에 발표했다"가 7/30이 된다. 실측 확인: `event_date 2026-01-08 · revised_at 2026-01-29 · 매출 333.61조`
  - **서프라이즈는 BE에서 한 번만 계산**(`model.ComputeSurprise`) — 화면에서 다시 구하면 홈·목록·탭이 다른 숫자를 말한다(`toTargetPct` 선례). 국내=영업이익, 해외=EPS(시장 관행). **기준치 0 이하면 퍼센트를 내리지 않고** 「흑자전환·적자전환·적자지속」 라벨로 답한다 — 적자 예상에서 흑자가 나면 −800% 같은 수가 된다
  - **랭킹에 유의성 하한**(영업이익 50억원 · EPS 0.1달러): 없이 두니 1위가 **+5,498%**였고 컨센서스가 1억원인 종목이었다. 하한 뒤에는 대한항공 +319.9%·SK이노베이션 +123.8%로 바뀐다. 하한 아래 행은 캘린더 본문에는 그대로 나온다(감추는 게 아니라 줄 세우기에서만 뺀다)
  - **지난 추정일은 내지 않는다** (브라우저 실측에서 발견): 8월 14일 목록이 **173건**인데 실제 발표는 20건이고 129건이 「추정 8/14」였다 — 빗나간 추측이 진짜 발표를 묻었다. 과거 칸은 일어난 일만 말한다
  - **응답 크기**: 월 상세를 그대로 내리면 3,089행·gzip 136KB·DOM 3천 줄이라 기본을 **오늘 하루**(2.1KB)로 두고 달 전체는 300행 상한 + 「N건 중 300건」 표기. 히트맵은 날짜별 건수만(292B). 실측 summary 0.026s · 하루 0.011s · 서프라이즈 0.041s
  - **한도 버킷 두 개 신설** — `fnguide`(자체 설정 60/일·30초, 공개 API가 아니라 게시된 한도가 없다)와 `finnhub_earnings`. 후자는 `ProviderFinnhub`의 5분 간격이 **라이브 ETF 폴러용**이라 배치가 폴러 때문에 실패한 실측에서 나왔다(`finnhub_migrate`와 같은 이유). 여러 달·여러 조각을 받는 경로는 `Manager.MinInterval`로 **스스로 쉰다** — 15개월 백필이 첫 달만 넣고 14개월을 잃은 뒤 고쳤다
  - **적재는 워커 전용**(`worker-earnings`, 국내 시즌 10–20시/2h · 비시즌 18:30 · 해외 06:00 KST, 기동 catch-up). 남의 사이트 두 곳을 부르는 유일한 워커라 API 프로세스와 분리한다. 종목 탭의 `usCalendar`도 조회 중 Finnhub 호출을 없애고 표에서 읽는다
  - **FnGuide는 공개 API가 아니다** (사용자가 약관 리스크 감수 결정). 되돌릴 수 있게: `FNGUIDE_ENABLED` 킬 스위치 · 수집을 `marketdata/fnguide` 한 패키지에 가둠 · 하루 2호출 · 화면 상시 출처 표기
  - **화면**: 월 히트맵(중립 농도 — `--brand`가 `--loss`와 같은 값이라 파란 농도는 "하락한 날"로 읽힌다) → 일자 목록 → 서프라이즈 보기 토글, 우측 레일(상회/하회 TOP·내 종목 다음 발표). **확정·예정·추정 세 배지를 절대 같은 모양으로 그리지 않는다**. 종목 클릭 = 기존 `SymbolQuickLookModal`
  - 검증: go build/vet/test·tsc·eslint(신규·수정 파일 0건)·`next build`(38페이지) green, Docker 재기동(Tesseract OK), `051` 적용 로그 확인. **브라우저 DOM·네트워크 실측** — 08-06 셀 「발표됨 511·예정 49·추정 3」, 08-14 현대해상 「기대 3690억원 → 5050억원 +36.9%」, 추정 행 「작년 2Q는 8/14 발표」, 서프라이즈 50행·국내 전환 시 영업이익 기준, 종목 탭 삼성전자 `consensus.available:true`·다음 발표 추정 2026-10-14. ⚠️ **스크린샷은 불가** — Browser 패널이 표시되지 않아 compositing이 안 된다(DOM·네트워크로는 전부 확인). ⚠️ 읽을 때 `main`이 **둘**이다(모바일 셸이 width 0으로 함께 존재) — `main[1]`이 실물이다
- 08-17: **기사 모달 — 종목·지수 없는 기사에서 메모·스탬프 버튼 제거 + 우측 빈 칸 숨김** — `NewsArticleContext`가 `!hasEntities`일 때 기사 단위 `NewsGatedActionBar`(메모·스탬프)를 띄우던 폴백을 제거했다. 08-15에는 "종목 없으면 기사 단위로 남긴다"였으나 실제로는 `/journal`·`/stamps`만 열리고 종목·기사 링크가 저장되지 않아 거시 기사(금리 등)에서 오동작으로 보고됨. 관심·메모·스탬프는 **`StockMiniCard`에만**. iframe 모달 우측 칸은 `articleModalContextHasContent`(관련 종목·지수 또는 같은 이슈 묶음)가 없으면 **아예 그리지 않음** — FE만, Docker 재기동 없음
- 08-17: **뉴스 시장 분류 — 국내 경로 해외 종목 기사 → 해외 탭** — `inferNewsMarket`의 `inTitle`이 티커(NVDA)만 보고 한글명(엔비디아)은 제목 증거로 안 셌다. `symbolsMentionedInTitle`로 제목에서 이름·별칭 매칭. 기존 domestic 행은 기동 1회 `RemigrateDomesticToUS`(`news:remigrate-market:v1:`). 예: 「중국 헤지펀드, 엔비디아·AI 인프라…」→ us — `lib/dashboard-news-meta-fit.ts`로 측정 분리, `queueMicrotask`+`ResizeObserver`. 뉴스 FE eslint 7건( set-state-in-effect·exhaustive-deps ) — 모달 `key={article.id}` 프레임 분리·mock 목록 effect 제거
- 08-17: **홈 「오늘의 뉴스」 행 높이 축소** — `LatestNewsCard`에서 언론사 배지를 빼고 지수·종목 칩(`NewsArticleChips`)을 제목 아래 메타 줄(속보·시각 사이)로 옮겼다. 칩이 별도 줄이면 옆 「오늘의 시장 브리핑」과 높이를 맞추느라 빈칸이 커졌다. **시간은 칩 바로 옆** — `flex-1`로 시간이 행 끝까지 밀리던 것을 칩+시간을 `inline-flex`로 묶어 수정(이후 08-17 wrap 수정으로 1줄 고정)
- 08-17: **출시 전 응답 속도 개선 — FE·BE 실측 기반** (사용자 요청: 기능 축소 트레이드오프까지 포함)
  - **먼저 쟀다.** 느린 엔드포인트 다섯을 특정하고 원인을 하나씩 분리했다 — 추측으로 고쳤으면 전부 엉뚱한 데를 짚었을 것이다. 실제로 「Redis 라운드트립」으로 지목한 뉴스 병목은 고쳐 놓고 보니 **시간이 그대로**였고, 진짜 원인은 CPU(전수 스캔)였다
  - **인덱스 2종** (`050_perf_indexes.sql`, 부팅 시 자동): `idx_stock_scores_rank(trade_date, score DESC, peer_pct DESC)` — 랭킹 쿼리가 `stocks` 17,070행을 풀스캔+filesort 하던 것을 없앤다(**2.10초 → 0.036초**). `idx_news_market_updated(market, updated_at)` — 목록 응답의 `meta.updatedAt` 한 칸을 채우는 `MAX(updated_at)`이 3만여 행을 읽었다(**727ms → 0.45ms**). 인덱스 이름이 마커라 테이블/컬럼 방식과 달리 `indexExists`를 새로 뒀고, 6개 컨테이너 동시 부팅은 1061을 `isSchemaRaceError`로 삼킨다. **드롭 후 재기동으로 재적용까지 실측 확인**
  - **`STRAIGHT_JOIN`으로 구동 테이블 고정** — 옵티마이저는 자유롭게 두면 `asset_type` 조건이 있는 `stocks`를 먼저 잡는다. `stock_scores`부터 읽으면 정렬이 인덱스에서 공짜로 나오고 LIMIT에서 스캔이 끝난다(해외 탭은 전체의 75%라 스무 줄에 서른 행 남짓)
  - **gzip 미들웨어**(`chimw.Compress(5)`) — 그동안 **압축이 아예 없었다**. 98KB가 그대로 나갔다. SSE(`text/event-stream`)는 chi 기본 목록 밖이라 압축되지 않고 Flush도 그대로 위임된다
  - **뉴스 목록 = 이번 작업의 절반.** 세 층이었다: ① 기사마다 `ResolveEmbedPolicy`가 호스트별 Redis GET(요청당 342회) → 호스트 단위 in-process 캐시로 **0회**, 미스는 배경 조사 ② `extractUSSymbols`/`extractSymbols`가 기사마다 이름 인덱스 전수 스캔 → **세대+본문 메모** ③ `trackedSymbols`·`usIndexSymbols`가 기사마다 watchlist+포트폴리오 조회 → 1분 TTL. 마지막에 목록 자체를 15초 캐시(ingest 시 즉시 무효화). **us 1.35초 → 0.09초(cold) / 0.027초(warm), domestic 0.42초 → 0.05초**
  - **홈 전용 뉴스 엔드포인트**(`/news/{market}/dashboard`, 사용자 선택) — 홈은 100건을 받아 여덟 줄만 그리고 나머지 92건은 키워드 집계·속보 수에만 쓴다. **앞 8건은 온전히, 나머지는 제목·태그·키워드만**. 집계 규칙을 서버로 옮겨 적지 않은 이유는 그러면 같은 규칙이 두 언어에 두 벌 생기기 때문이다. 98KB(gzip 29KB) → **35KB(gzip 10KB)**. `fullCount` 밖 기사로는 모달을 열지 않는다(본문·링크가 없다)
  - **리포트 목록 차트**(사용자 선택) — `chart.bars` 200봉 OHLCV(건당 19KB) → `chart.closes` 종가 60점. 둘 중 하나만 채워 보내고 **상세는 `bars` 그대로**. 응답 72KB(무압축) → **21KB(gzip 6.5KB)**
  - **실적 배치 엔드포인트**(사용자 선택) — `/stocks/earnings/batch?symbols=` 신설(상한 20). 홈이 종목당 1요청씩 8개를 쏘던 것을 1개로. 한 종목 실패는 그 종목만 빠진다(나눠 부르던 때와 같은 결과)
  - **FE 3단 waterfall 제거** — 「랭킹 → 배치 시세 → 실적」이었다. 일정에 필요한 건 종목 코드뿐이라 랭킹 직후 `onSymbols`로 올려 **시세와 일정이 나란히** 나간다. 이름은 늦게 와도 되므로 행에 담지 않고 그릴 때 찾는다(담으면 그때의 빈 이름이 굳는다). 실측 네트워크 로그에서 두 요청이 같은 밀리초에 출발
  - **FE 요청 캐시**(`lib/fetch-cache.ts`) — 화면마다 카드가 자기 fetch를 하는 구조는 그대로 두되(한 카드가 죽어도 나머지가 뜬다), **진행 중인 같은 요청을 합치고** 짧은 시간 재사용한다. 라이브러리는 안 들였다. 랭킹·점수·실적 5분, 뉴스 30초, 배치 시세 5초(시세를 캐시하려는 게 아니라 같은 순간 두 카드가 같은 목록을 부르는 것만 거른다). 로그인·로그아웃 시 `clearFetchCache()` — 키에 사용자 정보가 없다
  - 홈 API 요청 **17개 → 11개**, 최장 체인 3단 → 2단
  - 검증: go build/vet/test green, tsc·eslint(신규·수정 파일 0건)·`next build`(37페이지), Docker 재기동(Tesseract OK), 마이그레이션 드롭→재적용 실측. **브라우저 실측** — 홈에서 `news/domestic/dashboard`·`earnings/batch` 200 + `quotes/batch`와 동시 출발 확인, 응답 모양 확인(뉴스 100건 중 fullCount 8·50번째는 url 없음 / research `closes` 60·`bars` 없음 / earnings 심볼별 calendar), 리포트 화면 스파크라인 3개가 `viewBox="0 0 320 48"`에 60점 좌표로 실제 렌더, 뉴스 화면 정상. **화면 육안(스크린샷) 검증은 미완** — Browser 패널이 표시되지 않아 compositing이 안 된다(DOM·네트워크로는 전부 확인)
- 08-16: **홈 대시보드 개편 — 클릭 = 모달 4종 + AI 시장 브리핑 + 브리핑 우선 배치** (목업 `docs/mockups/dashboard-widgets-v2.html` → 사용자 선택: I2 · W1 · L3 · 모달 4종 전부)
  - **홈만 앱의 규칙 밖에 있었다.** 관심·보유·뉴스·리포트는 이미 「모달로 훑고 마음에 들 때 넘어간다」인데 첫 화면은 지수→`/market`, 종목→`/analysis`, 뉴스→`/news/domestic`, 리포트→`/reports/{id}`로 **전부 이탈**했다. 홈에서 하는 일은 하나를 정독하는 게 아니라 여럿을 견주는 것이라 셋을 비교하려면 뒤로가기를 세 번 눌러야 했다
  - **모달 4종 중 셋은 이미 있던 것을 연결만** 했다(`StockQuickLookModal`·`NewsArticleModal`·`ResearchQuickLookModal`). 신규는 **지수 모달 하나** — 앱에서 유일하게 모달이 없던 대상이었다(`news-dashboard-indices.ts`에 `index modal (future)` 주석이 남아 있었다)
  - **`SymbolQuickLookModal` 신설** — 공용 종목 퀵룩은 이름·통화·현재가가 채워진 객체를 요구하는데 홈은 그게 없다(점수 랭킹은 종가만, 브리핑 언급 종목은 심볼·이름뿐). 목록이 시세를 들고 있으면 `seed`로 **요청 0**, 없으면 열 때 배치 시세 1회. 화면마다 "시세를 어디서 구해 오나"를 다시 짜면 같은 종목이 자리마다 다른 정보로 열린다
  - **지수 모달(I2)**: 일별 종가 + **그날의 점수 열** + 추세점수/AI 한 줄 + 같은 시각 다른 지수 + 이 지수를 다룬 뉴스. 점수 열이 핵심 — 「63점」이 좋은 건지는 어제 61, 그제 57을 봐야 알 수 있고 그 값은 `daily-closes` 응답에 **이미 실려 온다**
  - **기간 버튼을 거래일 수로 바꿨다** (실측 중 발견): `limit=66`을 줘도 코스피는 **31행**만 온다(07-02부터). 캘린더 기간으로 이름 붙이면 「3개월」과 「1년」이 **똑같은 차트**를 그린다 → 최대치를 **한 번만** 받아 클라이언트에서 자르고, 앞 버튼보다 실제로 더 많은 봉을 그리는 단계만 남긴다(31행이면 「20일」·「전체」 둘, 하나뿐이면 토글 자체를 안 그림)
  - **`aiSummary`가 툴팁에 갇혀 있었다** — `/indices/scores`는 지금도 문장을 주는데 화면은 `title` 속성에만 넣어 **모바일에서는 존재 자체가 안 보였다**. 금·비트코인은 `score: null`인데 문장은 오므로 **점수 유무로 블록을 감추지 않는다**(감추면 있는 문장까지 사라진다)
  - **AI 시장 브리핑(W1)** — 서버에 있었지만 **뉴스 화면에만** 있었다. `/news/{market}/briefing`은 **읽기 전용**(생성은 배치·관리자만)이라 방문자가 늘어도 LLM 비용은 그대로. 뉴스 카드는 가로 띠라 문단을 3줄로 자르고 나머지를 모달로 뺐지만, 홈은 세로 한 칸을 통째로 써서 **핵심 사건·지켜볼 것까지 펼친 채로** 둔다. 언급 종목 칩 = 종목 모달
  - **지수 스트립 = 한 줄 자동 흐름 + 직접 밀기**(`IndexTickerStrip`, 사용자 결정). 카드 5장 그리드와 아래쪽 「전체 지수」 접힘을 **둘 다 없애고** 열두 종을 한 줄에 흘린다 — 접힌 것은 아무도 펼치지 않아 홈이 실제로 보여주던 지수는 다섯이었다. **hover·focus에서 멈추는 것은 장식이 아니라 기능**: 셀이 모달을 여는 버튼이라 움직이는 동안에는 조준할 수 없다. 목록을 **세 벌** 이어 붙이고 가운데 벌에서 시작한다(두 벌이면 뒤로 밀 때 곧장 시작점에 부딪힌다), 사본은 `aria-hidden`+`tabIndex=-1`. `prefers-reduced-motion`이면 흐르지 않는다
  - **흐름을 CSS transform이 아니라 `scrollLeft`로 돌린다** (사용자 요청: 멈춘 채로 직접 밀 수 있게). transform으로 밀려면 컨테이너를 `overflow:hidden`으로 잠가야 해서, 멈춰 놓고 볼 수 있는 것이 화면에 걸린 일곱 칸뿐이 된다. 스크롤바는 `.hide-scrollbar`로 감추고 기능만 남긴다
  - **터치 팬은 포인터 이벤트로 잡을 수 없다** — 팬이 시작되면 브라우저가 `pointercancel`·`pointerleave`를 쏘아 손가락이 아직 화면에 있는데 멈춤이 풀리고, 띠가 손가락과 싸운다. 그래서 포인터는 **마우스 hover와 키보드 포커스만** 받고, 사용자 조작은 **스크롤이 우리가 지정한 위치와 어긋났다는 사실**로 판정해 1.2초 유예한다(휠·트랙패드·손가락·관성이 한 규칙으로 잡힌다). 멈춘 동안 모은 이동분(`carry`)은 버린다 — 안 그러면 놓는 순간 훌쩍 건너뛴다
  - **이음매 간격**: 벌마다 `pr-2`를 들고 있으므로 바깥 flex에 `gap`을 주면 안 된다. 주면 **벌과 벌 사이만 16px**이 되어 무한히 이어져야 할 띠에서 그 자리가 눈에 띈다
  - **점수 배지에 「AI」를 붙였다**(사용자 요청) — 처음 온 사람에게 코스피 옆의 `60`은 등락률도 순위도 아닌 정체불명의 수다. 다만 **툴팁은 정확해야 한다**: 이 점수는 LLM이 아니라 20일선·RSI·최근 등락으로 계산한 규칙 기반 값이다(`index_score.go:116` — *"Written from computed values only — no LLM"*). 모달 섹션도 「AI 추세 점수」로 맞춰 배지 뜻이 화면마다 갈리지 않게 했다
  - **스파크라인이 12종 중 9종에만 있다**(원/달러·나스닥100 선물·필라델피아 반도체는 빈 배열, 실측). 없는 칸에 빈 상자를 두면 구멍으로 보이므로 **그림 자리를 빼고 값을 한 단계 크게** 적는다. 점수도 9종만 있다(금·비트코인·원/달러 없음) — 배지를 안 그릴 뿐
  - **배치**: 지수 스트립(전폭) → [브리핑 | 오늘의 뉴스] 2단 → 4열 [점수 TOP · 거래대금 · 실적·공시 · 오늘의 키워드] → 리포트. 브리핑 미생성 시간대에는 2단을 만들지 않고 **뉴스가 전폭**으로 늘어난다
  - **「다가오는 일정」을 「실적·공시」로 바꿨다** (구현 중 실측으로 뒤집힘): BE 주석이 그대로 말한다 — *"DART has no forward schedule API, so domestic symbols only ever produce `filed` events"*. 거래대금 상위 국내 8종목의 `upcoming`은 **0건**이라 원안대로면 **영원히 빈 카드**다. 대신 예정(미국 Finnhub)과 최근 제출(국내 DART)을 한 목록에 두되 **날짜 표기로 구분**한다(`D-9` ↔ `2일 전`). **종목당 1건** — SK스퀘어 하나가 기재정정까지 세 줄을 먹어 일곱 줄이 전부 같은 공시였다
  - **일정 날짜 형식이 출처마다 다르다** → `lib/earnings-date.ts` 신설. DART는 `2026-08-14`(ISO), Finnhub는 **`8/26`(연도 없는 M/D)**. 지금까지 화면들이 `date.slice(5)`로 잘라 써서 미국 일정이 **「6」**으로 떴다. 연도 없는 값은 올해로 읽되 반년 이상 과거면 내년으로 넘긴다. 종목 퀵룩의 같은 버그도 함께 교정
  - **「오늘의 키워드」는 추가 요청 0** — 기사는 `DashboardOverview`가 한 번만 받아 목록 카드와 키워드 카드에 내려준다(각자 부르면 「기사 100건 중」과 목록이 다른 스냅샷이 된다). 칩 클릭 → `/news/domestic?keyword=`. 이 파라미터가 **없어서 새로 뚫었다** — 초기값으로만 읽고, `[market]` 초기화 이펙트가 마운트에서 돌지 않게 `prevMarket` ref를 뒀다(안 그러면 뜨자마자 스스로 지운다)
  - **리포트 카드 = 리포트 화면과 같은 가로형**(R2). 목록 응답에 `chart.bars`(200포인트)·`evidence.breakdown`·`progress`·`toTargetPct`·조회·좋아요가 **이미 전부 실려 온다**(실측) — 화면이 부족했던 게 아니라 받아 놓고 안 쓴 것이었다. **추가 요청 0**. 좌(근거: 표지·발행시점 스파크·5축) / 우(주장: 제목·발췌·진행축)이고 양쪽 다 `flex-1`·`mt-auto`로 **바닥까지 채워** 빈자리를 남기지 않는다. 목표까지 거리는 서버의 `toTargetPct`를 쓴다(화면에서 다시 구하면 목록과 상세가 다른 숫자를 말한다)
  - **모달 닫기 버튼이 헤더 글자와 겹쳤다** — 닫기는 `top-4 right-4`(16px)인데 헤더 여백이 `px-4`(16px)라 정확히 포개졌다. 같은 구조의 **모달 7종** 헤더에 `pr-10`. `DialogHeader` 기본값으로 못 넣는 이유는 소비자가 주는 `px-4`가 tailwind-merge에서 나중에 와 `pr`을 덮기 때문
  - `MiniToggle`을 `components/ui`로 승격(대시보드·브리핑 공용), `MoodBadge`를 뉴스 카드에서 export. `MarketOverview`(그리드)·`MarketHeroStrip`·`AllIndicesSection`·`DashboardTopSection` 삭제, `MarketOverviewCompact`만 남김
  - **제안했으나 채택 안 됨**: 장 상태 바(`/market-calendar` — 동작은 확인, 단 **다음 개장일이 응답에 없어** "다음 개장까지 N시간"은 불가) · 이슈 종목 스트립 · 비로그인 온보딩(`/sample/*`) · 리포트 의견 분포. **만들 수 없는 것**: 급등·급락 TOP — 랭킹 API가 거래대금·거래량만 지원하고 등락률 정렬이 없다
  - 검증: tsc·eslint(신규·수정 파일 **0건**)·`next build`(37페이지) green. **API 실측** — 브리핑 국내·해외 둘 다 `source:"llm"`, `indices/scores` 12종, `kospi/daily-closes` 31행 중 6행에 점수, 국내 기사 100건에서 키워드 14개(실적 23·SK하이닉스 16·반도체 8), 실적·공시 24건 중 종목당 1건으로 7행. **화면 육안 검증은 미완** — Claude Browser 패널이 표시되지 않으면 `requestAnimationFrame`이 안 불려 React가 커밋을 못 한다(fetch는 전부 200인데 DOM은 스켈레톤에서 정지, 연결된 Chrome도 없음). 패널을 띄운 뒤 **자동 흐름·hover 정지·4열·리포트 카드**를 재확인해야 한다
- 08-16: **메모장·목표가·리포트 3화면 개편** (목업 `docs/mockups/journal-widgets-v1.html`·`targets-widgets-v1.html`·`reports-widgets-v1.html` → 사용자 선택)
  - **메모장 = 기록이지 예측이 아니다.** 스탬프처럼 적중/빗나감을 매기지 않고 **작성 시점 가격만 박제**한다(`047`: `price_at_write`·`price_currency`·`price_date`·`tags_json`). 「삼성전자 실적 좋아 보임」이라고 적어두고 두 달 뒤에 열면 그때 주가가 없어서 내가 옳았는지를 이 화면 안에서는 알 수 없었다. **`Update`는 가격을 건드리지 않는다** — 문장을 고쳤다고 그때 가격이 오늘 값으로 바뀌면 박제가 아니라 재구성이다
  - 메모장 **L2 2단 인박스**(목록 \| 본문). 예전엔 작성 폼이 항상 목록 맨 위에 펼쳐져 있었고 본문은 클램프가 없어 긴 메모 하나가 화면을 다 먹었다. 수정을 누르면 폼이 목록 위에 또 떠서 **고치는 중인 행은 저 아래** 남았다. `<lg`에서는 목록만 남고 행 클릭 = 모달
  - **태그**(`매수근거`·`리스크`·`관찰`·`복기` 프리셋 + 자유). 지금까지 분류 축이 종목뿐이라 **종목 미지정 메모**(환율·시장 관찰)는 시간순으로만 쌓였다. 레일에 「**다시 볼 메모**」(30일 지남 + 그동안의 등락) 신설 — 메모장을 다시 열게 만드는 건 새로 쓰고 싶은 마음이 아니라 옛 메모가 맞았는지 궁금한 마음이다
  - **목표가 v2** (`048`: `base_price`·`base_date`·`stop_price`·`target_price_2`). 예전 행은 「목표까지 +11.4%」 숫자 하나라 **가까운 건지 먼 건지가 안 읽혔다**. 등록가→현재가→목표가(+손절)를 한 축(`TargetAxis`)에 좌표로 올린다. 재등록 시 `base_price`는 `COALESCE`로 보존 — 목표를 올릴 때마다 등록가가 리셋되면 「등록 후 +14.9%」가 영원히 0 근처를 맴돈다
  - **도달일·최고/최저는 저장하지 않는다** — 스탬프 조기 적중과 같은 방식으로 `candle_bars` 일봉 고/저를 `base_date` 이후로 훑어 **읽을 때 계산**한다. 저장하면 목표가를 수정한 순간 옛 목표 기준의 도달일이 그대로 남아 어긋난다. 방향(상방/하방)은 **등록가 기준**으로 판정 — 하방 목표를 고가로 기다리면 영원히 판정되지 않는다
  - 목표가 목록은 **두 보기**(사용자 결정): 진행축 행 `row` ↔ 카드 그리드 `card`, `targets-view.ts`(`holdings-view.ts`와 같은 형태)에 저장. 상단 4타일(클릭 = 상태 필터) + 상태 칩 + 정렬 3종. 레일에 「**목표가 없는 보유 종목**」 — 이 화면을 실제로 채우게 만드는 유일한 진입점
  - **리포트: 이미 응답에 있는 데이터를 목록이 안 쓰고 있었다.** `chart.bars`·`evidence.breakdown`·`toTargetPct`/`toStopPct`가 전부 실려 오는데 카드는 텍스트뿐이었다 → **가로형 카드**(좌: 표지·발행시점 스파크·5축 / 우: 제목·발췌·진행축), **추가 요청 0**. 스파크는 발행 전 회색·발행 후 검정 + 발행 지점 점 하나, 목표·손절 파선
  - **리포트 퀵룩 모달** — 예전엔 카드를 누르면 곧장 페이지를 떠나, 셋을 비교하려면 뒤로가기를 세 번 눌러야 했다. 근거만 훑고 마음에 들 때 「전문 읽기」. 레일에 「곧 판정됩니다」 D-N·「목표·손절에 근접」·「의견 분포」 추가, 「많이 다룬 종목」은 이제 **목록을 거른다**(종목 분석 링크는 건수 쪽에 유지)
  - **썸네일**(`049`: `thumbnail_url`) — 기존 `/api/v1/media/images` 재사용(키가 내용 해시). 작성 폼에서 ① 업로드 ② **본문 첫 이미지 가져오기** ③ 안 함, 기본은 ③이고 화면이 **종목 로고**로 대신한다. 모든 작성자에게 그림을 요구하지 않으면서 목록이 들쭉날쭉해지지 않는 유일한 방법. `normalizeThumbnailURL`로 `javascript:`·`data:`·프로토콜 상대(`//`) 차단 — 이 값은 공개 카드의 `<img src>`로 직행한다. 표지 변경은 `body_rev`를 올리지 않는다(논거가 아니라 표현)
  - **예시 모드에 실시간 캔들을 섞지 않는다** (실측 중 발견): 목표가 카드가 「현재 214,500원」 옆에 실제 캔들 범위 **1,359,000~2,959,000원**짜리 차트를 그렸다. 한 카드가 서로 다른 두 종목을 말하는 셈. `useSparklineCandles(symbol, !sample)`로 차단(메모 상세 모달도 동일 — 픽스처 기준선이 실제 범위 밖이라 선이 사라진다)
  - 레일 카드 이름 충돌 교정: 상단 타일 「도달 임박」(목표 쪽만)과 레일 「도달 임박」(손절 근접 포함)이 같은 화면에서 1과 2를 동시에 말했다 → 레일을 「**곧 결판납니다**」로
  - 검증: go build/vet/test green, tsc·eslint(**신규 파일 0건**, 잔여 5건은 전부 기존 fetch-on-mount 패턴)·`next build`(37페이지), Docker 재기동(Tesseract OK). **DB 실측**으로 `047`·`048`·`049` 컬럼 3종 확인 — `047`은 로그가 안 찍혔지만(다른 컨테이너가 레이스에서 이겨 `isSchemaRaceError`로 삼킨 것) 컬럼은 정상. **브라우저 실측** — 메모장 예시 5건(삼성 +8.5%·NVDA +8.3%·카카오 −7.5%, 태그 4종 집계, 작성 후 평균 +3.1%), 목표가 예시 5건(도달/임박/손절근접 배지, 카드↔행 토글 localStorage 유지, 레일 5카드), 리포트 실데이터 2건(5축 막대·목표까지 +10.0%/손절 −29.9%·퀵룩 모달 전 섹션·곧 판정됩니다 D-84/D-85)
- 08-16: **종목 헤더 액션 통일 — 메모장·목표가 관리**
  - **FE**: `StockSymbolActionBar` 공통 컴포넌트 — 관심·스탬프·리포트 작성·메모장·목표가 관리 5종
  - 실시간 차트(`StockWorkspaceHeader`)와 AI 추천 랭킹(`AnalysisClient`) 우측 상단에 동일 배치
  - `useWatchlistToggle` 훅으로 분석 화면 관심 토글
- 08-16: **뉴스 종목·지수 매칭 개선**
  - **US 종목**: `ListUSNameCatalog`로 전 미국 종목 이름 인덱스 구축(기존 ~30종 한정 → 전체 마스터). 한국어 별칭(`마이크론`→MU, `애플`→AAPL 등) + possessive 헤드라인(`Micron's …`) ingest
  - **지수**: 금(`gold`/`금`)·BTC 규칙 확장, Goldman 오탐 방지. US 번역 후 한국어 headlineKo로 **지수 태그 재판정**
  - **재태깅 v3**: symbols + `index:*` tags 동시 소급 (`news:retag:v3:`)
  - **FE**: 기사 카드 하단에 **지수 칩**(금·비트코인·코스피 등) + 종목 칩 함께 표시 — `NewsArticleChips`
  - 검증: go test service green, Docker 재기동. **재기동 후 v3 retag 스윕이 자동 실행**되어 기존 기사 태그 갱신
- 08-16: **뉴스 품질 개선 — 국내 암호화폐 오탐 차단 + 해외 Google News 게이트**
  - **국내**: `strongMarketTerms`에서 BTC·가상자산 등 crypto 단독 키워드 제거 → `isDomesticCryptoOnlyArticle`으로 순수 코인 기사 차단(ingest·list 공통). 코인 전문 RSS(tokenpost·blockmedia·coinreaders)는 **증시 앵커**(코스피·증시·상장 등) 또는 **국내 종목 심볼** 없으면 통과 불가 — 코인 매매 맥락의 매수·매도·배당만으로는 더 이상 통과하지 않음. **코인리더스 RSS 폴링 제거**(DB에 남은 기존 건은 list 필터로 제외). `domesticNewsQualityScore`로 한경·연합·네이버 등 프리미엄 매체 우선 정렬
  - **해외**: Google News(`google_news`) ingest·list 모두 **제목에 시장 앵커**(`hasUSTitleMarketAnchor`) 필수 — hostname `news` 등 저품질 출처의 비증권 기사 차단
  - 검증: go test `TestIsStockRelated*`, `TestPassesStockRelevance*`, `TestPassesUSStockRelevance_googleNewsRequiresTitleAnchor`, Docker 재기동(Tesseract OK). **DB에 쌓인 기존 코인리더스 기사는 list 필터로 즉시 제외** — 완전 삭제는 `/admin` cleanup 또는 `CleanupIrrelevantArticles` 수동 실행
- 08-15: **스탬프 v2 — 예상 주가 필수 + 자유 만기 + 화면 전면 개편** (목업 `docs/mockups/stamps-widgets-v1.html`·`stamps-widgets-v2-target-price.html` → 사용자 선택)
  - **스탬프가 "북마크"에서 "예측"이 됐다**. 예상 주가가 필수가 되면서 **저장 전에 폼**을 받는다 (v1은 원클릭 저장 후 가설 부착). 저장된 스탬프는 전부 채점 가능한 완성본
  - **`stance`는 더 이상 고르지 않는다** — `target_price` vs 당시가에서 파생(`StanceForTarget`, ±3% = 관망). "약세인데 목표가는 위" 같은 모순이 구조적으로 불가능해졌다. 밴드 상수는 `model.StampNeutralBandPct` 하나만 두고 판정(`neutralBandPct`)이 그걸 재사용 — 두 개면 관망으로 접수하고 다른 밴드로 채점하는 일이 생긴다
  - **`horizon_days` → `horizon_kind` + `due_at`** (`046`). 「이번 장 종료」·「다음 장 종료」는 일수로 표현이 안 된다. 만기는 **생성 시점에 캘린더로 확정 저장** — 나중에 계산하면 휴장일 목록이 갱신될 때 과거 판정이 흔들린다. 6종: `session_close`(마감 후면 다음 장) `next_session_close` `1w` `1m` `3m` `custom`. 전부 **장 마감 시각**에 착지하므로 판정에 항상 실제 종가가 있다. 시장별 캘린더(KRX 15:30 / US 16:00 ET)는 `scheduleForStamp`가 고르고, `benchmarkForStock`·`marketTradeDate`와 **US/ETF 분기가 같아야** 만기·벤치마크·거래일이 서로 다른 시장으로 흩어지지 않는다
  - **판정 = 방향 + 정확도 2단** (사용자 결정). 적중/빗나감은 종전대로 **방향**으로 가려 기존 적중률 통계를 잇고, `targetAccuracy`(|만기가−목표|÷목표)를 **별도 축**으로 더했다. NVDA가 목표 +18%에 실제 +0.4%로 「적중」이 되는 구멍을 지표가 드러낸다
  - **장중 도달 = 조기 적중 종료** (사용자 결정). `candle_bars` 일봉 고/저로 `[stamped_at, due_at]` 구간을 훑어 강세는 `high ≥ target`, 약세는 `low ≤ target`이면 그 날짜로 확정하고 **즉시 개봉 대기**로 올린다. 만기 종가만 보면 "닿았다가 되돌아온" 예측이 빗나감으로 기록된다. **스탬프 당일은 제외** — 그날 고가는 스탬프보다 앞설 수 있어 이미 지나간 가격을 적중으로 쳐주게 된다. **관망은 조기 적중 없음**(닿을 방향이 없다)
  - **예측은 5분(=쿨다운)만 수정 가능** — `PATCH /stamps/{id}/prediction`, 이후 `ErrStampPredictionLocked`. 오타는 고치되 결과를 보고 목표를 옮기는 건 막는다. `note`·`tags`·`conviction`은 계속 열려 있다(`PATCH /stamps/{id}`). 리포지토리도 `UpdateAnnotation` / `UpdatePrediction` 둘로 갈랐다
  - **기존 스탬프 9건 전량 삭제** (사용자 승인) — 예상 주가를 사후에 만들어 낼 방법이 없다. 백업 `docs/archive/stamps-pre-046.sql`
  - **마이그레이션 레이스**: API+워커 6개가 동시에 부팅해 `046`을 함께 실행 → 진 쪽이 `Can't DROP 'horizon_days'`(1091)를 뱉었다. 스키마 최종 상태는 정상이지만 6개 컨테이너가 매 재기동마다 에러를 찍는다 → `isSchemaRaceError`(1060·1061·1091)로 무시. **이 배포 형태에서는 정상적인 결과**지 실패가 아니다
  - **FE**: A-1 성과 타일 4개(적중률 타일 → **성적표 모달**) / B-1 행 = **도장 실링**(적중·빗나감·개봉·검증 D-N, 테두리로 상태 구분) + 목표가 게이지 + 이후 경로 스파크(목표가 파선 동반) + 4구간 막대 / C-1 상세를 **모달로 전환**(관심·보유·뉴스와 통일) / D **개봉 모달** — 대기분을 한 장씩 넘기며 결과 공개 + 회고 한 줄(기존 `note` **뒤에 덧붙임**, 지우지 않음) / 레일 E-2 개봉 예정 타임라인 · E-3 종목별 성적 · E-6 스탬프 후 최대 등락
  - **E-6은 BE가 계산한다** — 조기 적중 판정이 어차피 일봉 고/저를 읽으므로 `peakReturn`·`troughReturn`을 같은 스캔에서 실어 보낸다(FE 추가 요청 0). "맞았지만 도중에 -6%를 버텼다"가 종가만 보는 표에서는 통째로 사라진다
  - 집계는 전부 `lib/stamp-stats.ts`에서 **이미 받아둔 배열로** 계산 — 관점별·기간별·확신도별·종목별·누적 알파·구간 비교 전부 서버 재요청 없음
  - 검증: go build/vet/test(신규 `stamp_horizon_test.go` 8건 포함), tsc·`next build`(37페이지)·eslint(만진 파일 잔여 1건은 기존 fetch-on-mount 패턴), Docker 재기동(Tesseract OK, 마이그레이션 에러 0). **브라우저 실측** — 목록 6행·실링 5종·게이지·레일 3카드, 성적표 모달 4탭, 개봉 모달(적중 카드 1/2), 상세 모달 전 섹션, 생성 폼(삼성전자 274,500원 → 목표 310,000 입력 시 **관망 0.0% → 강세 +12.9%** 파생, 「이번 장 종료」가 토요일이라 08-17(월), 「직접」은 날짜 전까지 저장 비활성). 로그인 게이트라 목록은 fetch 스텁으로 실측
- 08-15: **기사 모달 관련 종목/지수 카드 통일 + 미니 차트 정렬 버그**
  - **미니 차트가 현재가와 안 맞던 버그**: 애플 카드가 $305를 적고 차트는 작년 10월($266~286)을 그렸다. 원인은 `useSparklineCandles.takeCloses`의 `slice(-40)` — **일봉 응답 순서가 출처마다 다르다**(DB 경로는 오름차순으로 뒤집지만 **Toss 일봉 tail은 최신이 앞**, BE `newestCandleIndex` 주석이 같은 사실을 적고 있다). 그래서 최신 40개가 아니라 **가장 오래된 40개**를 거꾸로 그렸다. `candleTimeSortKey`로 **정렬 후 자르기**(캔들스틱 차트가 이미 쓰던 방식). 실측 06-18~08-14 / $275~$340로 교정
  - **`StockMiniCard`·`IndexMiniCard` 신설** (`components/stocks`·`components/market`) — 같은 껍데기. 액션(관심·메모·스탬프)이 **카드 안으로** 들어갔다. 예전 모달 하단 한 줄은 종목이 여럿이어도 **첫 번째 종목에만** 작용했다(아마존·애플 기사에서 애플만 담김). ~~종목이 없는 기사에서만 기사 단위 메모·스탬프를 남긴다~~ → **08-17 제거**: 종목·지수 없는 기사에는 액션 없음
  - **`StockScoreChip` 신설** — 숫자만 있던 배지에 `종합점수 30/100 C` + ⓘ 팝오버(5축 이름·"매매 신호 아님"·축별 보기 링크). 퀵룩 모달의 사설 배지도 이걸로 교체
  - **탭 → 카드 전부 펼침 + 지수 카드**: `NewsRelatedEntityPanel`이 종목 최대 6장 + 기사에 걸린 지수(`articleDashboardIndexIds`)를 지수 먼저 세운다. 점수는 `useStockScores` 배치 1요청, 일정은 종목당 1요청(상한=카드 수)
  - **시세 상한 구멍**: 화면 공유 배치는 60종목 상한이라 그 밖의 종목 카드가 "시세 못 불러옴"+통화 원화 폴백이 됐다(아마존 실측). 패널이 **빠진 것만** 한 번 더 부른다
  - **종목 클릭 목적지 통일** → `analysisSymbolHref(symbol,"ai")`. 뉴스 칩·대시보드 랭킹·보유 미리보기·목표가 카드·관심 레일. 「터미널」·「일봉」이라고 **적힌** 링크만 `/stocks/{symbol}` 유지
  - **BE 태깅 두 건** — ① 미국 이름 인덱스가 도메인·클래스 꼬리를 못 뗐다: AMZN 마스터명이 `Amazon.com`이라 `amazon`이 키에 없어 **"Amazon rising"이라 쓴 기사는 전부 놓쳤다**(Apple·MSFT·META는 멀쩡해 더 안 보였다). `trimHeadlineNameTail`(`.com`·`alphabet a`) 추가 → 재태깅 스윕 **us 13,503건 중 335건 수정**, 문제의 Fortune 기사 `["AAPL"]` → `["AAPL","AMZN"]`. 표식 `news:retag:v1:` → **v2** ② 코스피 규칙에 「서울 증시」·「seoul stocks」 추가(BE·FE 양쪽)
  - **지수 판정이 번역문을 안 봤다**: 해외 기사는 화면에 한국어가 뜨는데 규칙은 영어 원문만 봤다 → FE `articleText`에 `headlineKo`·`summaryKo` 포함
  - **지수 차트 금액 표기**: `PriceHistoryChart`에 `valueFormat="plain"` — KOSPI 7,096.89가 「7,097원」으로 찍히고 있었다
  - 검증: go build/test, tsc·eslint(만진 파일 0건)·`next build`, Docker 재기동(Tesseract OK). **브라우저 실측** — 문제의 기사에서 KOSPI·애플·아마존 3카드, 링크 전부 `/analysis?symbol=…&tab=ai`, 점수 팝오버 동작. 주의: 브라우저 창이 숨겨져 있으면 `useMarketIndices`가 `isPageVisible()`에 막혀 지수가 안 뜬다(검증 시 `visibilityState` 강제 필요)
- 08-15: **뉴스 코인리더스 필터 + 지수 카드 필터 통일**
  - **코인리더스 RSS**: 가상자산 키워드만 있으면 통과하던 crypto RSS 예외 제거 → **주식·증시 키워드**(`hasStockMarketKeyword`, crypto 단독 제외) 없으면 ingest 차단. 토큰포스트·블록미디어는 기존 crypto RSS 규칙 유지
  - **지수 pulse 카드**: `/market` 이동 제거 → 이슈 종목과 동일하게 **기사 N건** 표시 + 클릭 시 해당 지수 키워드로 목록 필터(코스피·나스닥 등 pill 매핑, 원/달러 등은 `index:` 태그·텍스트 매칭). 국내·해외 공통
  - 검증: go test `TestPassesStockRelevance*`, tsc 통과
- 08-15: **뉴스 화면 레이아웃 정리 (국내·해외 공통)**
  - **「내 종목」 pill**: 브랜드 전용 색 제거 → 다른 pill과 동일 스타일. 순서를 **「전체」 바로 다음(2번째)** 으로 이동
  - **지수 흐름**: 우측 레일 `IndexPulseCard` 제거 → 중앙 상단 `NewsIssueStrip`에 이슈 종목과 **같은 카드 형태**로 5개 지수 선행 배치(클릭 시 `/market`). `pickMarketIndices`를 `index-pulse-card.tsx`에서 공유
  - **레일 순서**: 오늘의 키워드 → 다가오는 일정 → 많이 본 뉴스 → 내 종목 뉴스 → 출처 분포 → 최신 투자 리포트. 해외 전용 `NewsMacroCard`는 레일에서 제거(국내·해외 동일 구성)
  - 검증: tsc 통과
- 08-15: **종목 모달 통일 + 헤더 검색 전역화 + 뉴스 레일 정리**
  - **퀵룩 모달 공용화**: `components/stocks/stock-quick-look-modal.tsx` 신설 — 헤더·40일 종가 차트·52주 위치·5축 점수·다가오는 일정·뉴스가 공통이고, 화면별 차이는 `contextSection`(맥락 지표) / `extraSections`(폼) / `footerActions`(삭제·제거) 세 슬롯으로만 들어온다. 관심종목·보유종목 모달은 얇은 래퍼가 됐고, 보유 점수 카드가 들고 있던 사설 `ScoreDetailModal`은 지우고 같은 모달을 연다 — **같은 종목을 어디서 누르든 같은 화면**이어야 한다
  - **하단 CTA 목적지**: `/stocks/{symbol}` 워크스페이스 → `analysisSymbolHref(symbol, "ai")`. 목록에서 종목을 눌러 처음 궁금한 건 호가창이 아니라 "이 종목이 지금 어떤가"다
  - **헤더 검색 전역화**: 감추는 경로는 `/login`·`/signup`·`/onboarding`뿐이고 나머지 기본 모드가 `stock`. 카탈로그 자동완성에서 고르면 AI 추천 랭킹으로 간다. `/analysis`·`/quotes`·`/market`은 종전대로 자기 화면 안에서 종목을 바꾼다(`quotes` 모드, 페이지가 등록한 핸들러). **엔터는 종목코드·티커로 읽히는 입력만** 이동시킨다 — `?symbol=삼성전자`는 존재하지 않는 심볼이라 한글 이름은 화면 안 필터로 남긴다. 모드 판정은 `supportsStockSuggestions()` 한 곳(provider와 입력 필드가 갈라지지 않게)
  - **기사 모달을 닫으면 뉴스 목록까지 닫히던 버그**: 기사(포털 `z-[70]`) 위에서 목록 Dialog를 `modal={false}`로 두니 기사 백드롭 클릭이 목록까지 흘러갔다. 목록을 **기사 보는 동안만 닫고**(`open={open && !selectedArticle}`) 스크롤 위치를 ref로 복원하는 쪽으로 전환 — 기사·필터·페이지 상태는 Dialog 바깥에 있어 언마운트돼도 재조회가 없다
  - **뉴스 레일 모달**: 종목당 25 → **15건**, 비중 상위 8 → **10종목**
  - **총 자산 카드**: 원화·달러 현금을 **같은 글자 크기**로 나란히 표기. 내부 Metric 칸(테두리·회색 배경)은 제거하고 **흰 카드 안에 텍스트만** 나열
  - **`set-state-in-effect` 정리** — 이번에 만진 파일은 전부 무경고. 세 가지 패턴으로 바꿨다: ① 열림/닫힘에 딸린 초기화는 이펙트가 아니라 **렌더 중 조정**(`wasOpen !== open`) — 이펙트로 하면 닫힌 프레임이 한 번 더 그려진다 ② 자동완성 결과는 **자기 key를 들고 다니고** 로딩은 `key` 불일치로 판정 ③ 헤더 `activeQuery`는 상태가 아니라 `debouncedQuery`에서 **파생** — 화면이 바뀌면 첫 프레임부터 비어 있다
  - 검증: tsc·`next build`(37페이지)·eslint(만진 파일 0건, 저장소 잔존 90건은 전부 기존 파일). **브라우저 실측** — 홈에서 `삼성` 입력 → 자동완성 8건 → 엔터 → `/analysis?symbol=102780&tab=ai`. 관심종목·보유종목은 로그인 게이트라 **모달 육안 미확인**
- 08-15: **내 포트폴리오·목표가 UI 수정**
  - **뉴스 기사 모달 겹침**: `SymbolNewsModal` 위에서 기사를 열면 `NewsArticleModal`(z-50, 비포털)이 Radix Dialog 포털 뒤에 깔림. body 포털 + `z-[70]` + 기사 열면 목록 Dialog `modal={false}` (포커스 트랩이 기사를 먹지 않게)
  - **평가금액 추이 일괄등록**: `BulkRegisterDialog`가 `onOpenChange(true)`에서만 drafts를 채워, 부모가 `open`을 true로 켜면 목록이 빈 채로 뜸 → candidates에서 바로 파생
  - **총 자산 현금**: 하단 10px 줄 → 총 매입금액·평가손익·종목 수와 같은 중앙 Metric. **일괄 추가**는 총 자산에서 빼고 `종목 직접 추가` 우상단으로
  - **목표가**: 「이 브라우저에만 저장됩니다」 문구 삭제(이미 서버 저장)
  - 검증: tsc 통과. FE만 — Docker 재기동 없음. `add-holding-card` JSX 닫힘 태그 어긋남으로 런타임 오버레이가 뜬 건 파일 전체 재작성으로 해소
- 08-15: **목표가 `useSyncExternalStore` 무한 루프 수정**
  - `/targets` 런타임 오류: `getServerSnapshot`이 `() => []`로 매 렌더 새 배열 반환 → `lib/price-targets.ts`에 모듈 스코프 빈 배열 캐시(`getServerSnapshot`)
- 08-15: **매매 이력 가격 불일치 정리**
  - `trade_date` **당일 또는 직전** 종가(`stock_daily_closes`) ≠ 기록 단가(평단)인 종목의 **매매 이력 전체 삭제** — 평가금액 추이 오점 원인. `MigrateHoldingTradesPriceMismatchCleanupIfNeeded` (API 기동 시 불일치 있을 때마다 실행). 1차(당일 종가만) 8종목·16건, 2차(직전 종가 fallback) **추가 8종목·16건** 삭제, `user_holdings.source` → `manual`. `cmd/cleanup-holding-trades` (`-dry-run`)
- 08-15: **메뉴 IA 개편 + 내 포트폴리오·목표가·일괄등록 개선**
  - **사이드바 IA**: `핵심/리포트/기록` → **`순위`(실시간 차트·AI 추천 랭킹) · 뉴스 · 내 투자(내 포트폴리오·관심 종목·스탬프·메모장·목표가) · 커뮤니티(투자 리포트) · 관리`**. 활성 메뉴 재클릭 시 `window.location.reload()`로 새로고침
  - **뉴스 모달 버그**: `SymbolNewsModal` fetch effect가 `targets` 배열 참조에 걸려 필터 칩 클릭 시 **"전체"로 리셋** → `key`만 의존. 기사 링크 `/news/{id}`(404) → **`NewsArticleModal` 팝업**
  - **평가금액 추이 일괄등록**: 오늘 날짜 고정 → **`BulkRegisterDialog`**(종목별 매수일 지정). BE `POST /user/holdings/trades/bulk-register`(기존 일괄등록 trade **업데이트**, 동일 키 **중복 insert 거부`). `044_holding_trades_dedup.sql`로 기존 중복 행 정리
  - **총 자산 카드**: `현금 ($)`+`달러 현금` 분리 제거 → 하단 **`$5,000(7,000,000원)`** 통합 표기. **현금·일괄 추가** 버튼을 `HoldingsSummaryCard`로 이동(`BulkAddHoldingsDialog` 신설)
  - **목표가 서버화**: `user_price_targets` + `GET/POST/PUT/DELETE /user/price-targets`. FE `lib/price-targets.ts` → API + localStorage **1회 마이그레이션**. `/targets`·`HoldingsTargetsCard` 전부 API CRUD
  - 검증: go build, tsc, Docker API 재기동(Tesseract OK)
- 08-15: **국내주식·해외주식(뉴스) 화면 개편 + 종목 태깅 파이프라인 수정**
  - **뉴스 기사의 종목 태그가 망가져 있었다** (A1·C1의 전제). 원인 셋이 겹쳤다 —
    1. 국내 이름 인덱스를 `trackedSymbols()`(관심·보유에 담긴 종목)로만 만들어, **아무도 담지 않은 종목은 영원히 태깅 불가** → `ListDomesticNameCatalog()` 전 종목(4,406개)으로 교체 + 30분 TTL
    2. 해외는 **티커 자체를 이름 인덱스 키**로 넣고 경계 없는 `strings.Contains` → 티커 `V`가 알파벳 v 든 모든 기사에 붙음(실측 73건 중 51건 Visa). 티커 매칭을 `usTickerTokenMatch`로 분리, **3자 미만 티커는 `$V`·`NYSE: V` 형태만**
    3. **가장 결정적** — `marketdata.MergeTickerSymbols`가 **1~5자 영문 대문자만** 통과시켜, 이름 매칭이 찾아낸 국내 6자리 코드를 바로 다음 줄에서 전부 버림. 수집 경로도 원래 이랬다 → `mergeTickerCandidates`(기존 심볼 재검사 안 함)로 교체
  - 한국어 특유의 보정 둘: **조사 화이트리스트**(`링크드`가 "링크드인"에 붙던 것 차단), **최장 일치**(`두산`이 "두산에너빌리티"에 붙던 것 억제 — "에"가 조사이기도 해서 경계 검사로는 못 잡는다)
  - 결과(최근 100건): 국내 **0 → 18건**, 해외 Visa 오탐 **51 → 1건**
  - **재태깅**: 기동 시 1회(`news:retag:v1:*` Redis 표식). **MySQL만 고치면 화면이 안 바뀐다** — `loadFromStorage`가 Redis 스냅샷을 먼저 보므로 스윕 후 스냅샷 무효화 필요. 실측 59,663건 스캔 / 29,215건 수정
  - **AI 뉴스 브리핑**(A3): `news_briefings` + `GET /news/{market}/briefing`(읽기 전용) + 스케줄러 + admin 배치 2종. 입력은 헤드라인 40(매체당 최대 5) + 지수 + 언급 상위 10(시세 포함) + 속보수 + 세션 + **직전 브리핑 10건 압축본(150자)**. 출력은 길게 받아 카드에서 3줄 클램프 + 모달 전문
  - **FE**: A1 이슈 종목 스트립 · A2 24시간 뉴스 흐름 · A3 브리핑 카드+모달 / B1 「내 종목」 pill(비로그인은 인기 종목으로 **실제 동작** + 예시 배지·안내 띠) · B2 2단 필터 / C1 종목 칩+등락률 · C2 시간 구분선 / D1 모달 관련 종목 패널 · D2 같은 이슈 묶음 · G4 액션 팝오버(`?next=`에 기사 id) / E1~E4 레일 카드
  - **시세 요청은 화면당 1회** — 스트립·행 칩·레일·모달이 `useSymbolQuotes` 하나를 공유한다
- 08-14: **관심종목 행 — 카드 전폭 사용 + 52주 위젯 원복** (사용자 지적)
  - **카드의 40%가 비어 있었다**: 칸을 전부 고정 폭으로 잡아 남는 폭이 현재가 오른쪽에 몰렸다. `WL_COL`을 **`flex-[n]` 비율 + `min-w` 바닥**으로 바꿔 어느 폭에서든 카드를 다 쓴다(1200px에서 종목 187·52주 209·기간 각 110·현재가 143)
  - **52주 위치는 3줄 블록으로 원복**(사용자 결정) — 저·고가 값 / 그라데이션 트랙+마커 / 「상단 63%」. 한 줄로 눌렀더니 정보는 맞아도 보기 싫었다. 행 높이 48→**66px**(사용자가 허용)
  - **정렬 함정**: 칸 상수의 `@2xl:block`이 `WatchlistCell`의 `flex`를 이겨 `justify-center`가 무효가 되고 있었다 — 전일·전주·전월·담은 뒤만 값이 6px 위로 붙음. 가운데 정렬을 **패딩(`py-[6px]`)** 으로 바꿔 display와 무관하게 만들었다. 점수 배지도 22→19px `WatchlistScoreCell`로 값 줄에 앉힘
  - 검증: 컨테이너 300~1200px 전 구간 **정렬 어긋난 칸 0 · 오버플로 0 · 행 66px**, 값 줄 전부 top 6px, 375px 카드 351px에 빈 공간 0
- 08-14: **관심종목 행 정돈(A안) + 종목명 깜빡임 수정 + 모달 차트 공용화**
  - **종목명이 5초마다 종목코드로 튀던 버그**: BE `zeroQuoteSummary`(`quotes_overlay.go`)가 `toss.GetStock` 실패 시 **`Name`에 심볼을 넣어** 응답한다. 빈 문자열이 아니라 FE `mergeQuoteSummary`의 `!next.name` 가드를 통과 → 폴링 한 번만 실패해도 이름이 코드로 덮어써짐. `isPlaceholderName`(빈 값 또는 `name === symbol`)로 판정해 **마지막 정상 이름 유지**. `market`·`logoUrl`도 같은 이유로 가드 추가. **BE는 손대지 않음** — 다른 소비자가 `Name` 비어 있음을 가정하지 않는다
  - **행 A안**: 라벨을 행에서 걷어내고 목록 맨 위 **헤더 한 줄**로. 행 높이 62→**48px**. 폭·접힘 규칙은 `watchlist-columns.tsx`의 `WL_COL` 한 곳 — 헤더와 행이 같은 상수를 봐야 표가 어긋나지 않는다
    - 헤더 좌우 패딩은 `px-[11px]` = 행 `px-2.5`(10) + **카드 테두리 1px**. 10px면 라벨이 값보다 1px 왼쪽에 선다
    - 현재가는 `min-w`가 아니라 **고정 폭**(`w-24`/`@2xl:w-28`) — 내용에 맞춰 늘리면 행마다 앞 칸들이 밀려 헤더와 안 맞는다
    - `WatchlistCell`이 값 18px·보조 12px로 **높이 고정** — 보조 줄 없는 칸도 자리를 비워야 값이 한 선에 온다
    - 접힘: 전월(@5xl) → 전주(@4xl) → 담은 뒤·핸들(@3xl) → 전일·52주 막대(@2xl)
  - **레인지 바**: 파랑→빨강 그라데이션 제거(이 앱에서 그 두 색은 하락·상승이라는 뜻이 있다). 중립 트랙 + 브랜드색 채움. `variant="inline"`(행) / `"block"`(모달)
  - **`PriceHistoryChart` 신설** (`components/charts/`) — 관심종목 퀵룩·보유종목 상세 **공용**. 최고·최저 **금액 축** + 위아래 여백 + 면적 + 시작가 기준선 + 끝점 + 기간 변화율. 보유는 여기에 `markerPrice=평단` 점선(140×44 미니 스파크라인 대체)
    - 금액 라벨은 SVG `<text>`가 아니라 **옆 HTML 칸** — `preserveAspectRatio="none"`로 가로를 늘리면 `<text>`도 찌그러진다. 세로 배율은 1이라 뷰박스 y를 px로 그대로 쓴다
    - 하단 라벨은 **"종가 N개"**(❌ "N거래일") — 커버리지 성근 종목은 40봉이 8개월에 걸친다(실측 12-16~08-14)
  - `useSparklineCandles`가 `times`도 반환(차트 축 날짜). 이참에 `set-state-in-effect` 제거 — `{key, points, times}` 패턴으로 loading 파생
  - 검증: tsc·eslint 통과. 임시 하네스 실측 — 컨테이너 278~1198px 전 구간 행 48px·오버플로 0, **헤더/행 전 칸 좌우 좌표 완전 일치**, 값 8칸 top 92px 동일, 375px 헤더 `종목·점수·52주·현재가`, 모달 486×931에 실데이터 차트(274,500원/94,400원 축)
- 08-14: **관심종목 탭 UI 개편** (목업 `docs/mockups/watchlist-widgets-v1.html` → 사용자 선택 C안(스파크라인 제외)+D+E+F)
  - **행 재설계**: 5칸 동일 무게 지표 → `점수 배지 · 52주 레인지 바 · 전일/전주/전월 3칸 · 담은 뒤 · 현재가 · 임박 일정 점`. 행 높이 100→62px
  - **컨테이너 쿼리 필수**: 칸 접힘 기준은 뷰포트가 아니라 **본문 컬럼 폭**(`@container` + `@2xl/@3xl/@5xl`). 1280px 화면도 사이드바+레일 300px을 빼면 목록이 600px대라 `sm:`류로 짜면 데스크탑에서 가로 오버플로. 모바일 포함 **전 구간 1줄 유지**(사용자 결정)
  - **`addedPrice`·`createdAt` 사용 시작**: `GET /watchlist`가 내려주던 값을 FE가 버리고 있었다 → 「담은 뒤 수익률」 열 + 레일 `WatchlistSinceAddedCard`. 옛 항목은 `addedPrice` 없어 `—`
  - **퀵룩 모달** `WatchlistQuickLookModal`: 행 클릭 시 페이지 이탈 대신 모달(40일 종가·52주 위치·담은 뒤·5축 점수·일정·뉴스·보유에 추가 폼). 데이터는 **열 때만** 조회
  - **툴바** `WatchlistToolbar`: 내 순서/등락률/52주 위치/담은 뒤/점수 + 국내·미국·상승·하락. 정렬은 **저장하지 않고**, 「내 순서」에서만 드래그 허용(정렬 상태 드롭이 내 순서를 덮어쓰는 것 방지)
  - **점수**: `GET /stocks/scores/batch` 목록 전체 1요청, 5초 폴링에 태우지 않음
  - **일정**: `useUpcomingEvents`(종목당 1요청·상한 12) 결과를 행 표식과 레일 카드가 **공유**. 타깃은 `visibleRows`가 아니라 **내 순서 기준**(등락률 정렬 시 5초마다 12요청 재발 방지)
  - **뉴스 모달 공용화**: `HoldingsNewsModal` 본체 → `components/stocks/symbol-news-modal.tsx`(보유는 얇은 래퍼)
  - 검증: tsc·eslint 통과. 로그인 게이트라 임시 하네스로 실측 — 컨테이너 278~1198px 전 구간 행 62px·가로 오버플로 0, 375px 1줄, 모달 486×602에 실데이터(캔들·실적·뉴스 200)
- 08-14: **보유종목 탭 UI·기능 개선**
  - **현금**: 레일 `CashBalanceCard` 제거 → `AddHoldingCard` 우상단 「현금」→ `CashBalanceDialog`. 원·달러·달러 환산은 `HoldingsSummaryCard` 지표에 항상 표시(주식+현금 동시)
  - **평가금액 추이**: 본문 상단 → **보유 점수 하단**으로 이동. 매매 이력 없는 보유 종목에 **일괄 등록**(수량·평단·오늘 매수 기록) CTA
  - **보유 종목 뉴스**: 「더보기」→ `/news` 이동 대신 **`HoldingsNewsModal`**(비중 상위 8종목 × 25건 선로드, 클라이언트 페이지·무한 스크롤). 카드는 최신 6건 유지
  - **목표가**: `HoldingsTargetsCard` 보유 종목 선택 **빠른 추가** 다이얼로그. `/targets` 입력은 **`StockCatalogSearchField`**(종목명·코드). **저장소 확인**: `quant-port-targets` localStorage만 — BE `user_*`·API 없음(투자 리포트 `research_calls.target_price`와 별개)
  - 검증: tsc 통과
- 08-13: **보유종목 현금(원·달러) 관리** — `user_cash_balances` + `GET/PUT /user/cash-balances`; FE 레일 `CashBalanceCard`(종목 추가 바로 아래), `holdings-calc` 총자산·비중, 요약 「총 자산」, 구성 도넛 **자산** 탭
  - **설계**: 매매 이력·Toss 예수금과 분리 — 사용자가 직접 입력한 예수금·달러 잔고만 합산. 달러 현금·달러 종목 모두 환율 없으면 합산 보류(기존 `canTotal` 규칙과 동일)
  - **검증**: go build/test, tsc, Docker API 재기동(Tesseract OK)
- 08-13: **매매 기록 입력 UX** — `TradeDateField`(한 줄 전체·한글 날짜·오늘/어제), 첫 기록 추가 시 현재 수량·평단 자동 채움
  - **증상**: 평가손익·현재가가 장중인데도 전 정규장 종가(또는 `symbol_last_prices` 배치 캐시)에 고정
  - **원인**: 보유/관심 목록 API(`GET /quotes?filter=held|watchlist`)의 `applyLivePrices`가 `marketStore.EnsurePrices` 배치값만 `currentPrice`에 넣고 **`ResolveLiveQuotePrice`(체결·호가·forming bar)를 거치지 않음** — 종목 상세·`/quotes/batch`와 경로 불일치. FE 보유 화면도 **최초 1회만** `filter=held` 조회(5초 폴링 없음)
  - **수정**: `enrichSummaryLivePrice` 신설 — poller Touch + microstructure warm + `ResolveLiveQuotePrice` + `applyQuoteChange`(등락률). FE 보유종목은 **`/quotes/batch`(EnsureQuoteDetail 경로)** + 5초 폴링(관심종목과 동일)
  - **검증**: go test `./internal/service`(LivePrice), tsc, Docker API 재기동(Tesseract OK)
  - **증상**: NAVER 19주는 정상인데 **안트로젠(065660)** 이 같은 수량·평단으로 추가 생성
  - **원인**: NAVER 손익 컬럼 OCR에서 `안트` 같은 **2글자 hangul debris**가 파싱·해석되며 `안트로젠`과 prefix score 1.0으로 auto-match
  - **수정**: resolver `shortHangulExactOnlyScore`(3글자 미만 hangul은 exact만), `isPureShortHangulDebris` fragment 필터, preview `dedupeResolvedPhantomRows`(065660+035420 동일 qty)
  - **검증**: live MySQL `안트`→no match; 3-fixture Docker OCR+resolve PASS; Docker API 재기동
- 08-13: **3장 스크린샷 업로드 — LG전자 누락·이상 행(phantom) 수정**
  - **증상**: 국내+해외 3장 업로드 시 LG전자 미인식, ARM 552주·`-SPOL…` 등 garbage 행 추가
  - **원인**: `$`만 보고 `overseas` locale → `16전자`(2글자 hangul) 탈락; OCR 수량 502·5020류 합산; finance 컬럼 debris 통과
  - **수정**: `원`+`$` 혼합 locale; `16전자`/`L6전자`→`LG전자` 보정·mergeKey `lg`+hangul; qty 5배↑ OCR 오인 시 작은 값 유지; `-`/`#`/SPOL·금액 포함 이름 필터
  - **검증**: `toss_domestic_7`+`toss_overseas_4`+`toss_mixed` fixture Docker OCR — LG전자 7주·ARM 50주; unit tests PASS
- 08-13: **스크린샷 OCR 평단가 — XOVR·SK하이닉스 계산 실패 수정**
  - **증상**: 토스 혼합 화면에서 XOVR 평단 `—`, SK하이닉스 ~2,867,955원(비정상)
  - **원인**: XOVR ~0.04% 수익률이 materiality 0.1% 미만으로 스킵; SK에 OCR garbage 수익률(45%)이 `reconcileParsedFinance`에서 PnL을 ~29M으로 부풀림 + KRW 평가액에 USD 태그
  - **수정**: `costestimate` 소액 USD·미세 수익률 허용, KRW/USD eval magnitude 추론, garbage return% vs tiny PnL 분리; `satellite_finance` 통화별 per-share 매칭·포트폴리오 요약행(`가나다순`) 차단; `SK하이닉스` OCR 이름 보정
  - **검증**: `overseas_toss_tsmc_xovr.png` Docker OCR — XOVR avg ~20.42, SK avg ~1,576,295(KRW); unit tests PASS
- 08-13: **API 패닉 방어 + graceful shutdown 강화**
  - **스크린샷 임포트 테스트 중 크래시 상세 원인**: import job(OCR) 자체가 아니라 **동시에 돌던 SSE stream hub race** — 보유종목 화면·다른 탭의 종목 차트가 quote-stream/candle-stream을 구독한 상태에서 market poller(1s)가 Publish, 업로드 직후 페이지 이탈 시 Subscribe cancel이 inner map을 수정 → `fatal error: concurrent map iteration and map write`(recover 불가, 프로세스 전체 종료). import는 HTTP 202 후 **백그라운드 goroutine**이라 미들웨어 Recoverer가 못 잡음
  - **근본 수정(이전 턴)**: streamHub Publish RLock을 순회·send까지 유지
  - **추가 방어**: `internal/safego`(백그라운드 goroutine panic recover+stack), import job processJob/이미지별 recover→job failed, market poller 심볼 refresh·batch worker tick에 safego.Call
  - **Graceful shutdown**: `Shutdown` 순서 HTTP drain → bgCancel → **import job WaitIdle** → pool close; shutdown timeout 30s; Recoverer에 method/path+stack 로그
  - **원인**: import job이 OCR 없이 `domestic` locale 재필터 → `TSMC(ADR)`(괄호)·`XOVR`(latin debris) 탈락
  - **수정**: `InferHoldingsLocale`(국내+해외 혼합 시 필터 생략), ADR 이름 domestic 허용, resolver `TSMC→TSM` 심볼 lookup
  - **검증**: `overseas_toss_tsmc_xovr.png` fixture + 기존 kiwoom/overseas 회귀 PASS
- 08-13: **스크린샷 임포트 — 비증권 이미지 방어 (강화)**
  - **증상**: 커뮤니티/채팅·엉뚱한 사진도 종목 미리보기까지 진행 (`7시간 전`, `검색/코드` 등 garbage row)
  - **수정**: `RejectNonBrokerageScreenshot` 강화(채팅 UI·UI 아티팩트·qty-only 무finance 거부), `RejectPreviewAfterResolve`(해석 후 garbage 비율·broker UI 없음), FE failed→upload 복귀+경고
  - **메시지**: `증권사 보유 화면을 스크린샷해 주세요.`
  - **검증**: `non_brokerage_chat.png` + 기존 kiwoom/overseas/watchlist fixture Docker OCR PASS; `screenshot_guard_test.go` 10건
- 08-13: **스크린샷 임포트 — 비증권 이미지 방어**
  - **증상**: 풍경·문서 등 엉뚱한 사진도 OCR·미리보기까지 진행
  - **수정**: `RejectNonBrokerageScreenshot`(보유/시세 화면 UI·금액·수량 신호 검증), `ParseHoldingsImage`→`ImageParseResult`(OCR 텍스트 반환), import job per-image + preview 신뢰도 게이트
  - **검증**: `screenshot_guard_test.go` 6건 + 기존 import 테스트 PASS; Docker API 재기동
- 08-13: **시세/관심종목 OCR — 종목명·코드만 (수량·평단 비움)**
  - **증상**: `$` 시세 화면(보유 수량 없음) 업로드 시 phantom qty·평단 또는 0건
  - **수정**: `InferQuoteListOnly` + quote list parser; name-only rows; import preview qty/avg skip
  - **검증**: `overseas_watchlist_quotes.png`; 기존 overseas holdings 회귀 PASS
- 08-13: **스크린샷 임포트 붙여넣기 UX** — 업로드 영역 클릭 시 폴더만 열리던 문제
  - **수정**: `Ctrl+V`는 Dialog/document 레벨 paste; **사진 선택**은 별도 버튼
- 08-13: **키움 삼성전자 OCR — 평단 5원 보정 + phantom 종목 제거**
  - **증상**: 삼성전자 109주 평단 359,262원(5원 단위 아님) + `키움 KPOP ETN`/`KO 키움자산` phantom 1~72주
  - **수정**: KRW 평단 5원 반올림(≥1,000원), broker UI noise 필터(키움자산/다른자), latin OCR debris(HIGHS 등) domestic 필터
  - **검증**: `domestic_kiwoom_samsung.png` → 삼성전자 109주 avg 359,260 단독; `go test ./internal/import/...` PASS
  - **증상**: 삼성전자 109주 평단 359,262원(5원 단위 아님) + `키움 KPOP ETN`/`KO 키움자산` phantom 1~72주
  - **수정**: KRW 평단 5원 반올림(≥1,000원), broker UI noise 필터(키움자산/다른자), latin OCR debris(HIGHS 등) domestic 필터
  - **검증**: `domestic_kiwoom_samsung.png` → 삼성전자 109주 avg 359,260 단독; `go test ./internal/import/...` PASS
- 08-13: **국내 ETF OCR — Kiwoom 평단가 계산 수정** (KODEX 21,476원 / TIGER 52,094원)
  - **증상**: KODEX 평단 15,198원(손실률 부호 누락·eval+return만으로 계산), TIGER 평단 `-`(finance 미연결)
  - **원인**: TIGER `..` 이름이 OCR fragment로 탈락; PnL `-1,225,.870` 등 comma·dot 혼합 OCR; 좌측 컬럼만 OCR 시 finance 컬럼 누락; KRW 손실 %에서 마이너스 누락
  - **수정**: inline name+eval / qty+pnl 파싱, finance 컬럼 OCR pass, domestic에도 `AttachSatelliteFinance`, mixed PnL amount parser, KRW unsigned return→loss 보정
  - **검증**: `domestic_etf_kiwoom.png` → KODEX avg 21,476 + TIGER avg 52,094; `go test ./internal/import/...` PASS; Docker API 재기동
- 08-13: **국내 ETF OCR — KODEX/TIGER 미국나스닥100 분리**
  - **증상**: 키움자산 스크린샷에서 KODEX 785주만 인식, TIGER 100주 누락 또는 동부건설 등 오매칭
  - **원인**: `mergeKey`가 hangul만 써서 KODEX/TIGER 둘 다 `미국나스`로 합쳐짐; fuzzy merge도 같은 지수명을 한 종목으로 처리
  - **수정**: latin issuer prefix(KODEX/TIGER) 유지, ETF OCR 보정(006×→KODEX), UI 노이즈(더보기) 필터
  - **검증**: `domestic_etf_kiwoom.png` → KODEX 785 + TIGER 100; resolver TIGER→133690; `go test ./internal/import/...` PASS
- 08-13: **해외주식 OCR — 맵러빈→앱러빈 해석 + 5종목 평단가**
  - **증상**: `맵러빈` "확인 필요"(APP 미해석), 평단가 4/5만 표시(앱러빈·오라클 등 `—`)
  - **원인**: OCR PnL `-6234.74(4.65%` 오인; `*러빈`+`app*` 규칙이 Applied/Apple 등 17k 카탈로그와 **동점(0.86)** → fuzzy margin 실패
  - **수정**: PnL return 역산; `*러빈`은 **applovin/applov** 전용; live MySQL `맵러빈`→APP(앱러빈) 검증
  - **검증**: `overseas_holdings.png` fixture 5종목 avg 418/37.2/217.7/335.77/143.5; `go test ./internal/import/...` PASS; Docker API 재기동
- 08-13: **해외주식 스크린샷 OCR — 5종목 파싱** (Applied/IREN/ARM/AppLovin/Oracle)
  - **증상**: `마미렌`/`맵러빈`/`머플라이드`, phantom `티비씨`/`나무가`, ARM 누락, 아이렌 50주(실제 55)
  - **원인**: `.404홀딩스(600)`·`50%`가 금액/다른 블록 qty로 오인; UTF-8 byte suffix 매칭 오류; 해외 `$` 목록인데 hangul 다수 → domestic으로 locale downgrade
  - **수정**(`textparse/` + `pipeline/ocr_passes.go`): hangul 이름은 money 스킵, `주`/`%` qty lock, ADR listing 허용, `FuzzyUnifyParsedLines`(해외 전용), overseas locale 필터(2글자 phantom 제거)
  - **검증**: `overseas_holdings.png` fixture — 5종목 exact; `go test ./internal/import/...` PASS; Docker API 재기동
- 08-13: **보유종목 OCR 평단가 계산 수정** — `1.852.010원` 등 dot thousands·`~` 손실 부호·괄호 손익 파싱
  - **증상**: 평단가 8/16/1/108원 등 비정상 값 (eval=383 등 OCR 금액 꼬리만 파싱)
  - **원인**: `krwAmountPattern`이 `[\d,]+`만 허용 → `1.852.010원`에서 `010`→10원; `~` 손실을 양수 PnL로 처리
  - **수정**(`textparse/finance_line.go` + `costestimate/estimator.go`): `parseMoneyAmount`, PnL/return **부호 재조정**(SDI `89,9908!`→손실, NAVER `(18.06%)`→마이너스), 천원 단위 스냅
  - **검증**: Docker OCR fixture — 삼성SDI 485,500·NAVER 260,000·삼성전자 364,500원; `go test ./internal/import/...` PASS
- 08-13: **보유종목 OCR 종목명 해석(resolver) — `삼성5미`/`16전자` 자동 매칭**
  - **증상**: OCR 7종목·수량은 맞지만 `삼성5미`·`16전자`가 "확인 필요"(종목코드 미해석). 단위 테스트는 통과했으나 **전체 catalog(17k+)** 에서 동점·오매칭으로 fuzzy 실패
  - **원인**: `ocrHangulCore`/`chosung`이 `삼성5미`→`삼성`, `16전자`→`전자`로 축소 → 수백 종목 1.0점 동점; `si` vs `sdi`는 prefix가 아니라 subsequence(`s…i`) 매칭 필요
  - **수정**(`import/resolver/fuzzy.go`): OCR 패턴별 전용 스코어링(선행 digit→`ocrLatinPrefixScore`, embedded digit→`ocrHangulLatinTailScore`+latin subsequence), 우선주 hangul 꼬리 페널티, embedded-digit fallback cap
  - **검증**: live MySQL `삼성5미`→006400, `16전자`→066570; `go test ./internal/import/...` PASS; Docker API 재기동
- 08-13: **보유종목 스크린샷 OCR — 이상 종목(TEADS/ALDEL) 제거 + 7종목 파싱** (하드코딩 없이 구조적 휴리스틱만)
  - **증상**: TEADS/ALDEL 등 해외 노이즈 종목 유입, 한화시스템 19주 오인, 삼성SDI·현대차 누락
  - **수정**: row-band OCR 제거, locale/fragment 필터 강화, `CollapseSimilarRows`, 다수결 qty 병합, orphan qty, mergeKey hangul 우선, 범용 OCR letter→digit confusion, 좌측 컬럼 upscale-only(전처리 패스 제거 — HAVER 노이즈 원인)
  - **검증**: Docker Tesseract 통합 테스트 PASS(7종목), `go test ./internal/import/...` 전체 통과
- 08-13: **보유종목 스크린샷 OCR 정확도 개선** — 사용자 제공 국내주식 7종목 테스트 이미지 기준으로 종목명·수량 파싱 정확도 대폭 개선.
  - **증상**: Tesseract PSM 11 단일 패스만 사용 → `삼성SDI`→`삼성5미`, `LG전자`→`16전자`, `4주`→`ae`/`a`/누락, `한화시스템` 23→19 오인식
  - **OCR 다중 패스**(`import/pipeline`): 전체 프레임(raw+2×전처리) + 좌측 55% 컬럼(2×업스케일) + 7행 밴드 OCR(PSM 6/7/11) → **다수결 병합**(`MergeMultiOCRPass`)
  - **전처리**: 2× 업스케일 + 다크테마 반전 + 이진화(140) — dim gray `4주` 텍스트 분리
  - **파서**: `ae`/`a` 스킵 제거 → OCR qty 힌트(4주 오인식), `주` 단독→LG 7주, `NormalizeBrokerOCRName`(삼성5미/16전자→정규명), garbage 필터(`. 삼성전자`, `e NAVER`)
  - **퍼지 매칭**: `삼성5미`·`16전자` resolver 테스트 추가
  - **픽스처**: `be/testdata/import/domestic_holdings_samsung.png` + Docker Tesseract 통합 테스트(`pipeline/ocr_passes_test.go`) + `cmd/ocr-debug`
  - 검증: go test 전체 통과, Docker Tesseract 통합 테스트 PASS(7종목 exact match)
- 08-12: **보유종목 직접 추가 — 종목 선택 UI** — `변경` 텍스트 버튼 제거, X로 선택 취소, 검색창(선택된 종목 영역) 클릭 시 다시 검색.
- 08-12: **보유종목 스크린샷 가져오기 → 우측 레일로 이동** — 본문 목록 아래에 있어 발견성이 떨어졌다. `종목 직접 추가` 바로 아래에 같은 `SectionCard` 크기의 진입 카드를 두고, 본문은 Dialog로 연다(`placement="rail"`).
- 08-12: **보유종목 화면 전면 개편 + 매입 이력(`holding_trades`) 신설** (목업 `docs/mockups/holdings-widgets-v1.html`로 옵션 11개 제시 → 선택받고 진행)
  - **🔴 데이터는 있는데 화면에 없던 것 2건**: ① 스크린샷 임포트가 `avgPrice`를 저장해 왔는데 목록이 안 썼다 → **수익률·평가손익이 화면에 아예 없었다** ② `deleteUserHoldingClient()`가 정의만 되고 **호출부 0곳** → 잘못 인식된 종목을 지울 방법이 스크린샷 재업로드뿐이었다. 둘 다 이번에 연결
  - **🔴 또 하나의 "정의 안 된 토큰"**: `@theme inline`이 `--color-chart-1: var(--chart-1)`을 매핑하는데 **`--chart-1`~`--chart-5`가 정의된 적이 없었다**(08-11 `--success`/`--danger`와 같은 사고). 그대로 쓰면 도넛 조각이 전부 상속색 한 덩어리가 된다 → 라이트/다크 양쪽 정의
  - **매입 이력 신설**(사용자 선택 3-C — "합성 곡선 말고 진짜 이력으로"): `039_holding_trades.sql` + repo/service/handler + `GET/POST/DELETE /user/holdings/trades`, `GET /user/holdings/valuation?days=`. 날짜별 실제 보유수량을 복원해 그날 종가 × 수량으로 평가금액 시계열을 낸다(**BE에서 계산** — FE로 하면 종목 수만큼 fetch에 기기 시계마다 답이 달라진다)
    - 평단은 **이동평균법**: 매도는 수량만 줄이고 평단을 재계산하지 않는다. 전량 매도 후 재매수는 평단이 **새로 시작**한다(옛 원가를 끌고 오면 안 낸 돈으로 값을 매기는 셈). 과매도는 공매도가 아니라 오타로 보고 0에서 막는다. 유닛 6건(`holding_trades_test.go`)
    - `user_holdings`는 이력이 있는 종목에 대해 **파생 캐시**(`source='trades'`)가 된다 — 기존 리더(quotes `held` 필터·대시보드·스탬프)를 하나도 안 건드리기 위해서다. 전량 매도 시 보유행만 지우고 **이력은 남긴다**(실현손익이 거기 있다). 보유 삭제는 이력까지 지운다(재추가 시 옛 원가 상속 방지)
  - **점수 배치 API 신설** `GET /stocks/scores/batch?symbols=` — 보유 종목 점수를 종목 수만큼 요청하던 걸 1회로. `MAX(trade_date)` 조인이라 점수가 끊긴 종목도 마지막 값이 나온다(목록에서 사라지지 않는다)
  - **손익 계산은 `lib/holdings-calc.ts` 한 곳**. 위젯마다 따로 더하면 같은 화면에서 합계가 어긋난다. 규칙 3가지는 전부 "모르는 값을 0으로 그리지 않는다"의 변형 — ① 평단 없는 종목은 손익 `null`이고 **합계 수익률의 분자·분모에서 함께 빠진다** ② 환율 없는데 달러 종목이 있으면 원화 합계 **보류** ③ 시세 없으면 평가금액 `null`
  - **본문**: 요약 큰 카드 1장(2-B) · 평가금액 추이(실선 평가 / 점선 매입) · 구성 도넛 종목+통화(4-B) · 오늘 손익 기여도(5-A, **등락률이 아니라 금액** 순 — 비중 큰 종목이 조금 움직인 게 더 큰 일이다) · 보유 점수 카드(클릭 시 5축 레이더 모달) · 표↔카드 전환 목록(1-C) · 스크린샷 마법사는 **접힌 채로**(10-A, 진행 중이면 자동으로 펼침)
  - **레일에서 뺀 것**: 보유 요약·비중 상위·많이 움직인 종목 — 전부 본문이 같은 말을 하게 됐다. 특히 **총 평가금액을 레일에 두면 안 된다**(레일은 `xl` 미만에서 본문 **밑**으로 쌓여 제일 중요한 값이 맨 아래로 간다). 남은 자리는 직접 추가·다가오는 일정·목표가·보유 종목 뉴스·지수
  - **목표가 저장소를 `lib/price-targets.ts`로 승격**(`targets-client.tsx` 사설 → 공용) — 보유 화면도 같은 목표가를 읽는다. 저장 키가 두 파일에 있으면 한쪽만 고쳤을 때 조용히 갈라진다. `useSyncExternalStore` 구독이라 목표가 화면에서 저장하면 보유 화면 카드도 함께 갱신된다(그 김에 기존 `set-state-in-effect` 1건도 사라짐)
  - **`set-state-in-effect` 회피 패턴**(이번에 반복해서 필요했음): 로딩 boolean을 이펙트에서 동기로 켜지 말고 **"어떤 요청에 대한 결과인지"를 데이터와 함께 들고** `loaded?.key === key`로 판정한다. 목록 새로고침도 `load()` 호출이 아니라 **버전 번호를 올려 이펙트를 다시 돌린다** — 이펙트에서 부른 함수 안의 setState는 린트가 이펙트 본문 것으로 본다
  - **실측 중 고친 것**: 도넛 상위 6개 + 기타를 쓰면 6번째 조각과 기타가 **둘 다 회색**이라 구분이 안 됐다 → 이름 있는 색은 5개까지, 회색은 **기타 전용**
  - 검증: go build/vet/test 전체 통과(신규 유닛 6건 포함), tsc·`next build`(37페이지) 통과, eslint 신규 파일 무경고(잔존 2건은 전부 기존 `screenshot-crop-canvas.tsx`). Docker 8컨테이너 재기동(Tesseract OK), `039_holding_trades.sql` 적용 확인(worker-scores 로그 + 13컬럼 실조회). `scores/batch` 실호출 200(5개 요청 → 4개 응답, 미존재 심볼은 빠짐). **브라우저 실측** — 로그인 게이트라 임시 하네스로 7종목 픽스처(평단 없는 종목 1개 포함) 렌더 후 삭제: 총 평가 24,108,903원 / 매입 19,923,545원(카카오 제외) / 손익 +2,093,358원 +10.51%, 카카오 평단·손익 모두 `—`, 도넛 5색+기타, 점수 가중평균 66.0 A3·B·C2·D1, 모달 열림(486×552), 표↔카드 전환 localStorage 저장, 1600 `994px 340px` · 1280 · 375 전부 가로 오버플로 0(표는 자기 안에서 스크롤), 다크 토큰 정상
  - 남은 것: 로그인 세션이 없어 **실데이터 화면은 육안 미확인**(하네스로 계산·레이아웃만 검증). 매입 시점 환율(`fxRate`)은 컬럼만 두고 입력 UI가 없어 **환차손익 분리는 여전히 불가**
- 08-12: **랭킹 점수 중립색 + 신호등 구간 조정 + `넓게` 보기 레일 폭 420px**
  - **🔴 범위를 잘못 잡았다가 되돌린 건**: "점수 숫자가 다 초록"이라는 지적을 **화면 전체**로 읽고 5축·레이더·펀더멘털·일별 점수까지 중립으로 바꿨는데, 사용자가 말한 건 **종목 랭킹 목록의 점수**뿐이었다(스크린샷으로 확인). 나머지는 전부 롤백. 다음에 "점수 색"류 지적을 받으면 **어느 위젯인지 먼저 특정**할 것
  - **남은 변경 2가지**: ① `scoreTone()` 구간 **45 → 35**(0~35 빨강 / 35~65 주황 / 65~100 초록, 첫 요청에 명시) ② 종목 랭킹 행의 점수·등급만 `SCORE_NEUTRAL_TEXT`(`text-foreground` — 라이트 검정 / 다크 흰색)
  - 중립을 상수로 둔 이유: `text-foreground`를 그냥 적으면 다음 사람이 "색 지정 누락"으로 보고 신호등으로 되돌린다. 유일한 사용처가 랭킹 목록임을 `stock-score-api.ts` 주석에 남김
  - **`넓게` 보기 잘림 수정**: 좌측 레일 300px에 순위·아바타·이름·스파크라인·등락·점수를 다 넣으면 이름 몫이 20px대라 종목명이 **한 글자로 잘렸다**(사용자 스크린샷). 사용자 결정 — `넓게`일 때만 레일을 **420px**로 넓히고 본문에서 폭을 빌린다
  - 이를 위해 보기 밀도 상태를 패널 밖으로 꺼냈다 → **`lib/ranking-view.ts`**(localStorage + 모듈 스코프 구독자 집합 + `useSyncExternalStore`). `storage` 이벤트는 **다른 탭에서만** 발생해 같은 화면의 두 구독자(패널 ↔ 페이지)를 잇지 못한다. `PageColumns`에 `wideLead` prop 추가
  - 검증: tsc 통과, eslint 신규 파일 무경고(잔존 4건은 전부 기존 `set-state-in-effect`). **브라우저 실측** — 랭킹 90·88·A+ 본문색(라이트 `lab(2.75)` / 다크 `lab(98.26)`), 종합 73은 초록 유지(다크 `rgb(74,222,128)`), 5축 83·79·66 초록 / 52 주황, 레이더 초록으로 **롤백 확인**. 폭은 1280 `420px 766px` ↔ `300px 886px` 실시간 전환, 1600 3단 `420 734 340`, 375 단일 컬럼 — 전 구간 가로 오버플로 0이고 `넓게`에서 잘린 텍스트 **0개**(최장 "현대코퍼레이션홀딩스" 112px 완전 표시)
  - ⚠️ 마켓 레이더 지수 점수칩은 손대지 않음(신호등 유지)
- 08-12: **종목 분석 심화 — 점수 신호등 + 레일 위젯 3종 + 랭킹 보기 전환** (목업으로 선택받고 진행)
  - **진행 방식 변경(사용자 요청)**: 이제 페이지 UI 작업은 **먼저 목업/제안으로 의견을 받고** 착수한다. 이번엔 `docs/mockups/analysis-widgets-v1.html`(옵션 0~6, 라이트/다크 토글)을 만들어 선택받았다
  - **🔴 기존 버그: 점수 색이 전 화면에서 죽어 있었다** — `--success`·`--danger`가 **정의된 적이 없는데 13곳**에서 쓰였다(관리자 배치·점수 패널, 시장 브리핑, 종합 점수 전부). `var()`가 상속색으로 떨어져 성공/실패·등급이 모두 검정이었다(실측). 두 토큰을 처음 정의하고, 점수용으로는 **신호등 3색 신설**(`--score-high/mid/low` = 초록·주황·빨강, 사용자 결정)
  - **tone 맵을 `lib/stock-score-api.ts`로 단일화**(`SCORE_TONE_TEXT/BG/BORDER/BADGE`) — 같은 맵이 5개 파일에 복사돼 있었고 그중 `mid`만 파랑(`--brand`)이라 화면마다 중간 점수 색이 달랐다. ⚠️ 점수 색은 **점수를 말하는 자리에만** 쓴다(등락률 칸은 profit/loss가 주인 — 빨강이 두 뜻이 되면 안 된다)
  - **우측 레일 3종 신설**: `stock-daily-close-card`(최근 30거래일 면적차트 **+** 표 — 차트는 흐름, 표는 정확한 값. 면적 색은 마지막 하루가 아니라 **구간 전체 방향**으로 정한다), `stock-schedule-card`(실적·공시 타임라인, DART/Finnhub 배지, **예정 건을 위로**), `stock-insider-card`(최근 90일 매수/매도/순증 + 최근 5건)
  - **점수 5축을 막대 → 레이더로 교체**(사용자 결정 — 막대는 AI 분석 탭에 이미 있다). `score-radar-chart.tsx`는 recharts 없이 SVG 직접. **값 없는 축을 0으로 그리지 않는다** — 커버리지 낮은 종목에서 0은 "가치 점수 바닥"이라는 없는 사실이 된다(삼성전자 심리·수급이 실제로 `—`)
  - **랭킹 패널 개편**: `간단`(기본·종목+점수) ↔ `넓게`(심볼·가격·스파크라인·등락) **보기 전환**(localStorage 저장, `useSyncExternalStore`로 하이드레이션 불일치·이펙트 setState 둘 다 회피) + **정렬 헤더**(종목·등락·점수) + **등락 강도 배경**(±2% 미만은 거의 안 칠함)
  - **점수 랭킹 API는 국내 종목에 `changeRate`·`closePrice`를 주지 않는다**(실측) → 넓게 보기일 때만 `fetchQuotesBatchClient`로 보강. 안 그러면 등락 칸이 전부 `—`이고 등락 정렬이 무의미
  - **메달은 기본 정렬(점수 내림차순)에서만** 표시 — 등락순 목록에 금메달이 3번째 줄이 아닌 곳에 박히면 순위표처럼 잘못 읽힌다
  - 검증: tsc·`next build` 통과, eslint **신규 6파일 무경고**(잔존 9건은 전부 기존 파일). 대비 실측 — 라이트 초록 5.02·주황 5.18·빨강 6.47 / 다크 10.29·7.92·6.48 (전부 AA 통과). 브라우저 실측 — 레일 7카드 실데이터(삼성전자 73점 A·동종 상위 3%·커버리지 90%, 30거래일 -20.31%, DART 공시 2건, 내부자 매수 4·매도 26·순증 -18,093주), 넓게 보기에서 시세 채워짐(씨젠 32,350원 +5.20%)·히트셀 12·스파크라인 17, 1600/375 오버플로 0
- 08-11: **🔴 dev 서버 CSS 캐시로 UI가 깨져 보이던 문제 + 홈 대시보드 재구성**
  - **사용자 신고**: "색감·선이 너무 많이 바뀌어 가시성이 나빠졌다". **원인은 디자인이 아니라 dev 서버였다** — 사용자가 띄워둔 3000번 서버(PID 36808)가 shadcn 토큰 마이그레이션 **이전부터** 떠 있어 옛 스타일시트를 계속 내보내고 있었다(served CSS에 `--card: oklch` 없음, `--brand` 없음, 옛 4테마 팔레트 `#0b0f17`/`#000`/`#f1f5f9` 잔존)
  - 그 결과 새 컴포넌트의 shadcn 유틸이 전부 무효로 해석됨 — `bg-card` → **투명**(카드 구분 소멸), `border` → **rgb(15,23,42) 거의 검정**(선이 전부 진함), `text-muted-foreground` → **본문과 동일**(위계 소멸), `bg-accent`·`bg-muted` → 투명. 신고 내용과 정확히 일치. **디스크의 `globals.css`는 무결**이었다
  - 조치: 서버 kill → `.next/dev`·`.next/cache` 삭제 → 재기동. **이 사고는 08-05에 이어 두 번째** — 메모리(`css_ui_regression_caution`)에 "served CSS를 fetch해 새 토큰 존재 여부로 판별" 절차를 추가
  - **대비 보정(실측 기반)**: 텍스트 대비는 원래 정상이었으나(본문 19.8) 구조 분리가 약했다 — `--border` 0.922는 흰 카드 위 **1.26**이라 사실상 안 보임 → **0.87(1.48)**, 다크는 `1 0 0 / 10%` → **16%(1.63)**. `--muted-foreground` 0.556은 4.74인데 이 앱 보조 텍스트가 대부분 **10~11px**이라 부족 → **0.52(5.49)**. 다크 보조텍스트 6.94
  - **홈 대시보드 재구성(사용자 요청 "알차고 예쁘게")**:
    - **`MarketHeroStrip` 신설** — 지수 12종이 8.5rem 카드로 3그룹에 깔려 **첫 화면 600px이 전부 지수**였다. 매일 보는 5종(KOSPI·KOSDAQ·USD/KRW·나스닥·S&P500)만 스파크라인+점수칩과 함께 한 줄로. 나머지는 **`AllIndicesSection`(기본 접힘)**에 그대로 — 줄인 것이지 버린 게 아니다
    - **`DashboardOverview` 확장** — `오늘의 점수 TOP`(신규, 국내/해외 토글 · 이 앱만의 자산) + `거래대금 상위`(국내/해외 토글 신규) + `오늘의 뉴스 7` 3열, 그 아래 `최신 투자 리포트 4`
    - 시장 토글은 **블록마다 따로** 둔다 — 하나로 묶으면 "국내 점수 보며 해외 거래대금 보기"가 막히는데 그게 대시보드에서 하고 싶은 일이다
    - 토글 시 로딩 되돌리기는 `setBlock(LOADING)` 대신 **본문만 `key`로 재마운트** — 이펙트 setState는 렌더를 한 번 더 돌리고 그 프레임 동안 이전 시장 데이터가 새 탭 밑에 남는다
  - 검증: tsc·`next build` 통과, eslint **신규 파일 무경고**(대시보드 잔존 3건은 기존 `asset-*`). 브라우저 실측 — 홈 전 블록 실데이터(KOSPI 6,345.53 +0.73%·오스코텍 90 A+·삼성전자 241,000원·뉴스 7건·리포트 2건), 토글 4종 실제 전환 확인(점수 국내 오스코텍 ↔ 해외 트리플플래그 / 거래대금 삼성전자 ↔ 마이크론), 전체 지수 42px↔840px, 1600/1280/375 오버플로 0, 라이트·다크 양쪽 확인
  - **실측 함정(재확인)**: `document.querySelector('main')`이 **SSR 스트리밍용 숨은 복사본**(height 0)을 먼저 반환한다. 이걸 읽고 "데이터가 안 온다"고 한 번 오진했다 — 반드시 높이/`offsetParent`로 거를 것. 또 `key`로 재마운트되는 카드는 DOM 노드가 교체되므로 클릭 전에 잡아둔 참조는 **detached**가 된다(토글이 안 먹는 것처럼 보임)
- 08-11: **전체 폭 후속 — 화면별 다단 재구성(본문 + 우측 레일)**
  - **문제**: 앞선 세션에서 본문이 뷰포트를 다 쓰게 바뀌었는데(`page-shell-wide`) 페이지 구성은 그대로라, 예전 중앙 컬럼용 화면들이 **가로로 늘어나기만** 했다. 종목 분석은 랭킹 패널이 화면 폭을 통째로 가로지르는 배너였다(사용자 스크린샷)
  - **해결 방침(사용자 결정)**: 본문 폭을 다시 좁히지 않는다. 남는 폭을 **같은 맥락의 관련 위젯**으로 채운다
  - **공용 프리미티브 2종 신설** — `components/layout/page-columns.tsx`(`PageColumns`: 본문 + 우측 레일, 종목 분석만 좌측 레일까지 3단 / `StatRow`), `components/ui/section-card.tsx`(`SectionCard`·`SectionEmpty`·`SectionSkeleton`·`SectionKeyValue`). 홈 대시보드가 들고 있던 사설 `SectionCard`를 여기로 승격
  - **브레이크포인트 규칙**: 레일은 `xl`(1280) 이상에서만 컬럼이 되고 그 아래에서는 본문 밑으로 쌓인다. 컬럼 정의는 `minmax(0,1fr)` — `1fr`만 쓰면 안쪽 긴 표가 컬럼을 밀어 레일이 잘린다. **3단 화면의 우측 레일은 `2xl` 미만에서 두 컬럼을 가로지르며 `grid-cols-2`로 흐르고 sticky를 끈다**(본문 아래 블록을 붙여두면 스크롤이 멈춘 것처럼 보인다)
  - **재구성한 화면 8개**:
    - **종목 분석 3단** — 좌: 종목 랭킹(배너 → 레일, `max-h-420px` 고정 해제하고 레일 높이를 채움 / 좁은 화면에서만 420px), 중: 기존 탭, 우: 신규 `stock-side-rail.tsx`(시세 스냅샷 + 52주 위치 막대 · 종합 점수 5축 · 관련 뉴스 5 · 이 종목 리포트). **탭을 옮겨도 시세·점수·뉴스는 같은 자리에 남는다**
    - **뉴스 2단** — `news-side-rail.tsx`(지수 흐름 · 오늘의 키워드 · 많이 본 뉴스 · 최신 리포트). 키워드는 **화면에 이미 있는 기사에서 센다**(서버 재요청 없음 → 집계와 목록이 어긋날 수 없음), 누르면 피드가 그 키워드로 필터되고 제목 옆에 해제 칩이 뜬다
    - **리포트 2단** — 성적표 4타일을 레일로(`research-side-rail.tsx`: 내 성적표 · 많이 다룬 종목 · 많이 본 리포트 · 채점 방식 안내). 목록은 `2xl`에서만 2단. 순위표는 여전히 없음(§15-1)
    - **관심종목** — "종목 추가" 검색 카드를 레일로 옮기고(가끔 하는 일이 매번 목록을 밀어냈다) 오늘의 요약·강세/약세·지수 추가
    - **스탬프** — 요약 4타일 + 종목 칩 벽(종목 20개면 3줄로 접혀 목록을 밀어냄)을 레일 세로 목록으로. 개봉 대기·관점 분포 추가
    - **메모장** — 검색·종목 필터를 레일로, 고정한 메모·현황 추가. 정렬은 목록 헤더로
    - **목표가** — 입력 폼을 레일로. 목록에 **현재가·목표까지 남은 %**를 추가(`fetchQuotesBatchClient`). 현재가가 없으면 `—`로 두고 0%로 그리지 않는다
    - **보유종목** — `holdings-side-rail.tsx`(총 평가금액·오늘 손익·비중 상위·많이 움직인 종목). 원/달러가 섞이므로 **환율로 원화 환산**하고, 환율을 못 받았으면 일부만 더한 값을 "총 평가금액"이라 부르지 않고 합계를 보류
  - **손대지 않은 것**: 마켓 레이더(요청 제외), **설정**(데스크탑에서 896×640 모달이라 애초에 늘어나지 않음 — 실측 확인), 종목 워크스페이스·시세(이미 터미널형 다단)
  - **기존 버그 1건 수정**: 홈 "최신 투자 리포트"의 방향 배지가 자체 맵(`long`/`short`/`neutral`)을 쓰는데 API 값은 `buy`/`sell`/`hold`라 **전부 폴백인 "관망"으로 찍히고 있었다** → 리포트 화면과 같은 `DirectionBadge`로 교체
  - **실측 중 발견해 고친 것**: 관심종목 강세/약세를 정렬 후 위 3 / 아래 3으로 자르면 **종목이 5개 이하일 때 같은 종목이 양쪽에 나오고 보합이 "약세"로 불린다** → 부호로 먼저 거른 뒤 자른다(하네스 실측으로 확인)
  - **실측 함정(신규)**: 이 브라우저 창은 `document.visibilityState === "hidden"`이라 `useMarketIndices`의 `isPageVisible()` 게이트에 걸려 지수가 영영 안 온다. `visibilityState`를 덮어쓰고 `visibilitychange`를 쏘면 그때 로드된다. 이걸 모르면 "지수 위젯이 안 뜬다"고 오진한다(앱 정상 동작)
  - 검증: tsc·`next build`(37페이지) 통과, eslint **신규 파일 10개 무경고**(수정 파일에 새 문제 0 — 남은 `set-state-in-effect`는 전부 이전부터 있던 것). **브라우저 실측** — 1600px 3단(300/854/340, 오버플로 0) · 1280px 2단 + 레일 2×2 격자 · 375px 전 화면 세로 스택 오버플로 0; 종목 분석 랭킹 클릭 → 본문·우측 레일 동시 전환(RF머트리얼즈 38,400원/89점 A+), 뉴스 키워드 "유가 — 21건" 클릭 → 피드 100 → **정확히 21건**, 목표가 실측(삼성전자 +24.5% / SK하이닉스 "도달"), 보유 레일은 로그인 게이트라 **임시 하네스로 교차통화 합계 검증**(원 2,400,000 + $1,000×환율 = 3,817,710원, 일간손익 -1,266원, 비중 62.9/37.1) 후 하네스 삭제
  - 남은 것: 로그인 세션이 없어 **관심종목·보유종목·스탬프·메모장의 실데이터 화면은 육안 미확인**(레일 자체는 하네스/빈 상태로 확인). 관리자 7화면은 이번 범위 밖
- 08-11: **UI 후속 — 사이드 이펙트 수정 + 전체 폭 전환 + 홈 종합 대시보드**
  - **사이드 이펙트 3건 수정**: ① **라이트 모드에서 카드가 배경과 구분되지 않음** — shadcn 기본값은 `--background`도 `--card`도 순백이라 화면이 흰 판 하나가 됐다(실측). 페이지 배경만 `oklch(0.97)`(#F5F5F5)로 한 단 내림 ② `StatCard`가 자체 `p-3 sm:p-4`를 유지해 `Card`(p-2.5)와 안쪽 여백이 달라 보임 → 같은 값으로 통일 ③ 라벨·수치 글자 크기도 촘촘 밀도에 맞춤
  - **🔴 전체 폭 전환(사용자 결정)**: 모바일 반응형이 따로 있으므로 데스크탑에서 중앙 컬럼만 쓰던 것을 그만둔다. `PageScrollShell`/`PageHeader` 기본 아케타입 `reading` → **`wide`**. 거터는 `.page-shell-wide`가 `padding-inline: .75rem`로 가지고 **`PageHeader`의 `px-3` 특례는 제거**(거터가 두 겹이면 헤더/본문 경계가 다시 어긋난다). 직접 `page-shell`을 쓰던 admin 3개·news·settings도 교체. **실측 1280px에서 본문 714 → 1050px**
  - `reading`은 **긴 산문 화면에만 명시적으로** 남김 — 리포트 상세·리포트 작성. 이 둘은 `PageHeader`에도 같은 값을 넘겨야 컬럼이 맞는다
  - **홈 종합 대시보드 신설**(`components/dashboard/dashboard-overview.tsx`): 지수 바 아래에 거래대금 상위 8 + 오늘의 뉴스 6(2단) + 최신 투자 리포트 4(카드). **블록마다 독립적으로 실패**하도록 각자 fetch·로딩·빈 상태를 가진다 — 하나가 죽어도 화면 전체가 비지 않는다. 랭킹 응답에 종목명이 없어 `fetchQuotesBatchClient`로 이름을 채운다(코드만 보이면 훑을 수 없다)
  - 검증: tsc·`next build`(37페이지)·eslint 통과. **브라우저 실측** — 홈에서 3블록 모두 실데이터 렌더(삼성전자·SK하이닉스 등 8종목, 실제 뉴스 6건, 리포트 2건), 라이트/다크 배경-카드 단차 확인, 마켓 레이더 제목 x=232 ↔ 본문 x=232 정렬, 1280/375 오버플로 0
  - 남은 것: 컴포넌트별 자체 패딩(`p-4` 등)이 아직 여럿 남아 밀도가 완전히 수렴하지 않았다. raw `<button>` 182곳·`<input>` 48곳 이관도 그대로
- 08-11: **버그 수정 — 종합 점수 배치가 8/7 고정으로 산출되던 문제(종가 적재 스케줄러의 "놓친 세션" 영구 스킵)**
  - **증상**: 08-11 00:25 KST 정기 실행된 `worker-scores`가 `2026-08-07 — 17902 rows`로 산출(로그 확인) — 8/10(월)이 이미 지난 거래일인데도 그 이전 날짜로 고정
  - **원인**: `ScoreService.LatestTradeDate()`(`stock_score.go`)는 `stock_daily_closes`의 `MAX(trade_date)`를 그대로 씀. 그 테이블을 채우는 `DailyCloseService.RunMarketScheduler`/`nextTrigger`(`daily_close.go`)가 **항상 `now` 이후의 다음 트리거만 계산**해서, KRX 마감+20분(15:50 KST) 창이 이미 지난 뒤에 컨테이너가 재기동되면 그 세션은 **영원히 스킵**됨 — 8/10 세션이 정확히 이 경우였음(전 세션이 `docker compose up --build`로 8/10 17:50 KST에 재기동, 즉 그날 마감창을 이미 지난 뒤). `candle_bars`엔 8/10 1d 봉이 정상 존재(별도 폴러가 채움) — 프로젝션 단계만 누락. 이 프로젝트는 **BE 변경마다 Docker 재기동이 관행**(`AGENTS.md`)이라 재기동 타이밍이 마감 직후에 걸릴 확률이 낮지 않음
  - **즉시 복구**: `go run ./cmd/backfill-daily-closes`로 8/10 세션을 `candle_bars`→`stock_daily_closes` 프로젝션(외부 API 호출 없음, 355행), `go run ./cmd/compute-scores`로 8/10 기준 재산출(16,215 scored, healthy) + `batch_job_runs`에 수동 enqueue해 이미 떠 있는 `worker-scores` 컨테이너가 지수 재계산까지 수행. KR 지수(kospi/kosdaq)는 별도 폴러 경로라 이번 버그 영향 없이 이미 8/10 기준이었음(확인함); US 지수는 8/10 미국장이 아직 안 끝나 8/7 유지가 정상
  - **근본 수정**(`daily_close.go`): `RunMarketScheduler`가 루프 진입 전 `catchUpMissedSessions`를 한 번 실행 — 각 스케줄의 **가장 최근에 이미 지난 마감+지연 시각**을 역방향 탐색(`lastCompletedTrigger`, `nextTrigger`의 대칭)해 그 세션을 즉시 재적재. 멱등 UPSERT + 외부 호출 0이라 매 기동마다 무조건 실행해도 안전(이미 최신이면 그냥 조용히 끝남). 신규 유닛 3건(`daily_close_schedule_test.go` — 재기동 갭 재현, 주말 스킵, 정상 케이스)
  - 검증: go build/vet/test 전체 통과, Docker 8컨테이너 재기동(Tesseract OK) — 이번 재기동 자체가 버그를 유발했던 것과 같은 타이밍이었는데 로그에 `daily close: krx 2026-08-10 — ...`(도메스틱)·`daily close: us_equity 2026-08-07 — ...`(오버시즈, 정상 — 미국장 미마감)로 캐치업이 실제로 작동함을 실측 확인
  - `worker-scores` 자신의 00:25 KST 일일 스케줄도 같은 종류의 "놓친 창은 다음날까지 대기" 구조이나, 그쪽은 다음날 정기 실행 시 `LatestTradeDate()`를 그때그때 새로 읽어 자연 치유되므로(수동 큐도 있음) 이번엔 손대지 않음 — 영구 스킵 위험이 있는 종가 적재 스케줄러만 수정
- 08-11: **UI/UX 설계 인터뷰(D1~D18) + shadcn 2단계 구현 — 화면이 실제로 바뀜**
  - **설계 확정**: `docs/UI_SPEC.md`(최종 명세 17절, 사용자 승인) / 근거·버린 대안은 `docs/archive/ui-spec-interview.md` / 목업 `docs/mockups/shadcn-shell-v2.html`
  - **인터뷰 중 사실관계 정정 2건**: ① "사이드바에 없는 화면 6종"은 숨긴 게 아니라 **`page-stubs` 준비 중 화면 + `ACCOUNT_FEATURES_VISIBLE=false`로 꺼둔 기능**이었음 → ⌘K 팔레트 도입 근거가 무너져 **팔레트를 넣지 않기로 뒤집음** ② 사이드바 항목은 22개가 아니라 **일반 사용자 기준 10개**
  - **주요 결정**: 좌측 사이드바 + 전체 폭 헤더(56px) / 검색은 입력창만 / 헤더 우측은 계정 드롭다운 하나 / 사이드바 48px 레일 + 접힘 저장 / **활성 표시는 전 화면 중립**(파랑은 하락을 뜻하므로 색이 튀는 곳을 숫자로 한정) / **밀도 촘촘**(행 30px·본문 12px·컨트롤 32px) / 뉴스=행·리포트=카드 / **테마 2종**(midnight 제거, 다크 #000→#252525) / 짧은 건 모달·긴 건 시트 / 모바일 표는 3열+펼치기 / 관리자 데스크탑 전용 / 등락은 **색+부호**(화살표 제거)
  - **🔴 토큰 전략 — 레거시 토큰을 shadcn 토큰의 별칭으로**: `--surface: var(--card)`, `--text-primary: var(--foreground)` 등. `var(--surface)`를 쓰는 137곳을 건드리지 않고 색만 넘어간다. 별칭 제거는 호출부 교체 후
  - **`Card`/`Badge`는 레지스트리 버전으로 교체하지 않음** — shadcn `CardHeader`가 `grid`+`px-6` 기반이라 이 앱의 `flex justify-between`와 규칙이 달라 137곳 헤더 정렬이 한 번에 어긋난다. 동작 없는 표현 컴포넌트라 토큰만 갈아입히면 결과가 같다
  - **테마 마이그레이션**: `preferences.ts`가 저장된 `midnight`/`general` → `dark`로 변환(안 하면 기존 사용자가 light로 튕김). `layout.tsx` 초기화 스크립트·설정 카드·`@custom-variant dark`·news-badge 선택자도 정리
  - 사이드바 접힘 상태를 `quant-port-sidebar-open`에 저장(예전엔 `matchMedia`로 매번 초기화돼 새로고침하면 풀렸음). **좁은 화면에서는 저장값을 적용하지 않는다** — 오버레이라 앱을 가린 채 뜬다
  - 검증: tsc·`next build`(37페이지)·eslint 통과(기존 `set-state-in-effect` 3건은 이전부터 있던 패턴), **브라우저 실측** — 헤더 56px, 사이드바 220px↔48px, `--sidebar` 배경(라이트 #FAFAFA/다크 #343434), 활성 메뉴 중립(oklch 0.93/0.269 + weight 500), 로그인/회원가입 버튼 32px·12px·radius 8px, 라이트/다크 양쪽 오버플로 0
  - **실측 함정(신규)**: `document.querySelectorAll("header")`가 2개를 반환한다 — 하나는 **React SSR 스트리밍용 `<div id="S:0" hidden>` 안의 복사본**이다. `offsetParent`로 걸러야 진짜 셸을 잰다. 이걸 모르고 숨은 복사본을 재느라 "사이드바가 안 펼쳐진다"는 오진을 한 번 했다
  - **미확인**: 로그인 세션이 없어 **계정 드롭다운(설정·로그아웃)은 실제로 열어보지 못함**. 관리자 화면·종목 워크스페이스 탭 바도 동일 이유로 육안 미확인
  - 남은 것: 3단계(사이드바 shadcn 블록화·토스트 배선), 4단계(페이지별). **앱 전역의 raw `<button>` 182곳·`<input>` 48곳은 아직 자체 클래스** — 밀도가 컴포넌트로 수렴하려면 이들을 `Button`/`Input`으로 옮겨야 함
- 08-11: **shadcn/ui 전환 1단계 — 기반만 (FE만, 화면 변화 0)**
  - **목표(사용자 결정)**: FE 전체를 **shadcn 기본 스타일로 리디자인**. 4단계로 끊어 진행하며 이번은 1단계(기반)만. 계획·충돌표·단계 범위 → `docs/SHADCN_MIGRATION.md`
  - **스택은 그대로 적합**: Next 16.2.10 + React 19 + Tailwind v4 + `@/*` + `cn()`(clsx+tailwind-merge) + lucide. shadcn CLI 4.16.2가 Next 16을 문제없이 감지했고 **`globals.css`를 건드리지 않았다**(`add` 전후 diff 0). 토큰은 전부 수작업 — CLI init에 맡기면 자기 `:root`를 뒤에 붙여 기존 `[data-theme=...]` 팔레트를 특정도 동률·순서 우위로 덮는다
  - **🔴 `--accent` 이름 충돌이 이번 작업의 핵심**: 이 앱에선 **브랜드 블루**(링크·활성), shadcn에선 **은은한 hover 표면**으로 의미가 정반대다. 그대로 두면 shadcn 컴포넌트의 hover 배경이 전부 파래진다 → 앱 쪽을 **`--brand`/`--brand-muted`로 이름 변경(65파일 273곳)** 하고 `--accent`를 shadcn에 넘김. **새 코드에서 브랜드 색은 `var(--brand)`**
  - `--background`·`--border`는 이름·의미가 같고 **값만** 달라서 1단계에서 손대지 않음(덮으면 미교체 화면 전부가 그 자리에서 바뀜) — 2단계로 이월
  - **`dark:` 변형을 `data-theme`에 연결**: `.dark` 클래스가 아니라 `<html data-theme>`이고 어두운 테마가 셋(dark·midnight·general)이라 `@custom-variant dark (&:is([data-theme="dark"], [data-theme="midnight"], [data-theme="general"], ... *))`. 컴파일된 CSS와 4테마 계산값으로 실측 확인
  - **일부러 넣지 않은 것 2개(2단계)**: ① `--radius-*` 스케일 — shadcn 전용이 아니라 **Tailwind 기본 반경을 덮어써서** 실측상 로그인 버튼 `rounded-lg`가 8px→10px로 바뀌었고 앱 전체 **341곳**(md 105·lg 194·xl 39·sm 3)이 동시에 움직인다 ② shadcn base 레이어(`* { @apply border-border }`, `body { bg-background text-foreground }`) — 색 미지정 테두리 전체와 body 글자색을 즉시 바꾼다
  - 신규 의존성 `radix-ui`·`class-variance-authority`·`tw-animate-css`. **주의**: 돌고 있던 dev 서버는 새로 설치된 패키지를 못 찾아 `Can't resolve 'tw-animate-css'`로 죽는다 — 설치 후 dev 서버 재시작 필요
  - 검증: tsc·`next build`(37페이지)·eslint 통과. **브라우저 실측** — 4테마 전 토큰 값이 작업 전 기준선과 **완전 동일**, `/login` 제출 버튼(bg `rgb(37,99,235)`·h 44px·**radius 8px**)·입력창·카드 `rounded-xl` 12px 그대로, 375/1280 오버플로 0. 임시 하네스로 shadcn Button 6변형×4테마 실측(light `--primary` lab 7.78% / dark·midnight·general lab 90.95%, `dark:bg-destructive/60`·`dark:bg-input/30` 적용 확인) 후 하네스 삭제
  - **실측 함정(문서에 기록)**: 테마를 바꾸고 `getComputedStyle`을 그냥 읽으면 안 된다 — shadcn 컴포넌트의 `transition-all` 때문에 직후 읽으면 전이 시작값(이전 테마 색)이 나오고, 기다렸다 읽으면 `PreferencesProvider`가 `data-theme`를 되돌려 놓는다. 이 둘 때문에 "dark에서 토큰이 안 먹는다"는 잘못된 결론을 한 번 냈다. 전이를 죽이고 같은 태스크에서 동기적으로 읽어야 함
  - 남은 것: 2단계(프리미티브 Card 137·Button 182·Input 48·Badge 36 + 반경/base/`--background` 넘기기) → 3단계(사이드바·⌘K·토스트, `--sidebar-width` 충돌 정리) → 4단계(페이지별). **교체하지 않는 것**: lightweight-charts 터미널·recharts·자체 마크다운 파서/에디터·손익 색
  - 기존 버그 1건 발견(미수정): `--brand-foreground`(옛 `--accent-foreground`)는 **정의된 적이 없는데 3곳에서 쓰인다**(`admin-llm-panel` 2·`stock-news-panel` 1) → 글자색이 상속으로 떨어짐. 2단계에서 shadcn Button 교체로 자연 해소
- 08-10: **리포트 사용성 2차 — 목록 필터 재구성 + 2줄 초안 + 이미지 업로드 기반 구축**
  - **목록(`/reports`)**: 좌측 탭 `전체 리포트`(기본값·왼쪽) / `내 리포트`. **상태 필터(검증 중·판정 완료·철회) 제거**, 좌측에 의견(전체/매수/매도/관망)+역발상, **우측 끝에 정렬 `최신`/`조회 순`/`추천 순`**(`ml-auto`, 좁은 화면에선 아래 줄로 내려가되 계속 우측 정렬). 정렬에서 알파순·판정 임박이 빠졌고 조회/추천은 동점 시 최신순 폴백. 상태 배지는 카드에 그대로
  - **초안 2줄 + 클릭하면 사라짐**: `buildResearchDraft`가 방향별 안내 2줄만 반환(HTML 주석 → 평문). `MarkdownEditor`에 `clearOnFirstEdit` prop 신설 — 값이 그 안내 그대로일 때만 muted로 보이고 **첫 클릭/포커스에 지워진다**(글자수도 0). 손대지 않은 안내는 발행 시 빈 본문으로 치환돼 **절대 게시되지 않는다**. 기존 긴 자동 데이터는 `buildResearchDataBlock` → `preview.dataMd`로 분리해 **`발행 시점 데이터 넣기` 버튼**을 눌렀을 때만 삽입(LLM 0콜 유지)
  - **이미지 업로드 — 리포트 전용이 아니라 앱 공통 경로로 설계**(`docs/RESEARCH_CALL_DESIGN.md` §18): `be/internal/media`의 **`Store` 인터페이스 + 첫 구현 `LocalStore`**, `MediaService`(정책), `MediaHandler`(`POST /api/v1/media/images` 인증 / `GET /api/v1/media/*` 공개). **키 = 내용 sha256**(`ab/cd/<hash>.png`) → 같은 파일 중복 저장 없음 + 불변이라 `immutable` 캐시 가능. **타입은 서버 스니핑으로 결정**(클라이언트가 주장한 Content-Type·파일명은 근거 아님), **SVG 거부**(스크립트 담는 문서), 서빙에 `nosniff` + `CSP default-src 'none'; sandbox`, **키 정규식 검증이 경로 탈출 방지**. 객체 스토리지로 옮길 때 바꾸는 건 `Store` 구현 하나 + `MEDIA_PUBLIC_BASE_URL`
  - **Docker: `quant_port_media` named volume 신설** — import temp와 달리 영구 보관이라 writable layer에 두면 재기동마다 사라진다. Dockerfile에서 미디어 디렉터리를 미리 만들어 소유권(appuser)이 볼륨에 복사되게 함
  - **FE**: `lib/media-api.ts`(`uploadImageClient` — research 아닌 공통), 에디터에 사진 버튼 + **붙여넣기·끌어놓기** 업로드, 업로드 중 스피너·실패 메시지, 업로드는 prop으로 주입 가능(`uploadImage`). 마크다운 파서/렌더러에 이미지 노드 추가 — `src`는 링크와 같은 화이트리스트(http(s)/mailto/내부경로), **`javascript:`·`data:`는 이미지가 아니라 alt 텍스트로 렌더**(실측 확인). 업로드 중 타이핑해도 삽입이 기존 본문을 덮어쓰지 않게 `latest` ref 사용
  - 검증: go build/vet/test 전체(**신규 유닛 15건** — media store 5·media handler 6·draft 3 + 기존), tsc/`next build`, eslint 신규 파일 무경고, Docker 8컨테이너 재기동(Tesseract OK, `media store: /var/lib/quant-port/media (max 5 MB)` 로그 확인). **브라우저 실측**: 목록 버튼 배치·기본값(전체 리포트/전체 의견/최신)·정렬 우측 정렬(1280 flush right, 375 두 번째 줄) 및 375px 오버플로 0; 임시 하네스 페이지로 에디터 실측(안내 2줄 muted·클릭 시 즉시 삭제·타이핑 후 **이미지 붙여넣기 → `![차트캡처](/api/v1/media/...)` 삽입** → 미리보기 `<img loading=lazy>` 렌더, 위험 src 2종 평문화) 후 하네스 삭제; API 실측은 비로그인 401·경로 탈출 404·잘못된 키 404
  - **미확인**: 세션이 만료됐고 **계정 생성·비밀번호 입력은 하지 않는 정책**이라, 로그인 상태의 실제 업로드(멀티파트 → 저장 → 서빙)와 작성 화면의 `발행 시점 데이터 넣기` 버튼은 브라우저로 못 봄. 대신 **핸들러 레벨 테스트**(201 + 저장 왕복 + 헤더 + 415/413/400)로 고정
  - 남은 것: 참조 해제된 이미지 GC 없음(발행 안 한 초안 업로드는 남음), 사용자별 업로드 쿼터 없음, 리사이즈·썸네일 없음
- 08-10: **리서치 → 리포트 개편 — 리더보드 삭제(보상제도 미도입) + 마크다운 블로그형 UI**
  - **명칭(사용자 결정)**: 메뉴 `리서치` → **`리포트`**, 객체 `리서치 콜` → **`투자 리포트`**. FE 경로도 `/research/*` → **`/reports/*`**(`/reports`·`/reports/new`·`/reports/[id]`·`/reports/authors/[userId]`). **API·테이블·코드 식별자는 `research` 그대로** — `/api/v1/research/*`, `research_calls`, `components/research/*`, `lib/research-*.ts`. 화면에서 "리포트"를 찾을 땐 코드에서 `research`로 검색할 것
  - **리더보드 완전 삭제 + 보상제도 도입 안 함**: FE `/research/leaderboard`·`research-leaderboard-client.tsx`, BE `GET /research/leaderboard`·`ResearchService.Leaderboard`·`research_leaderboard.go`(연율화·페널티·기간/시장 필터)·`ResearchLeaderboardEntry`/`ResearchLeaderboard` 모델·`ResearchRepo.ResolvedForLeaderboard`·리더보드 유닛 11건 제거. **남긴 것**: 작성자 성적표(`AuthorProfile` → 신규 `service/research_authors.go`로 이동)와 개인 요약(적중률·평균 알파·철회율). 이유는 코드 주석·설계문서에 남김 — **줄 세우기는 리포트를 대회 참가작으로 만들고, 대회는 보상 없이 유지되지 않는다**(유사투자자문 규제선). 유지된 테스트 2건(`researchRatable`·`ViewerKey`)은 `research_social_test.go`로 이동
  - **마크다운 파서/렌더러 자체 구현(의존성 0)**: `lib/markdown.ts`(블록: 제목·문단·목록(중첩)·인용·코드펜스·구분선·GFM 표 / 인라인: 굵게·기울임·취소선·코드·링크) + `components/ui/markdown.tsx`. **`dangerouslySetInnerHTML`를 쓰지 않는다** — 리포트 본문은 공개된 사용자 작성물이라 HTML을 주입할 수 있는 렌더러는 그 자체로 저장형 XSS 경로. 링크는 `http(s)`/`mailto`/내부경로만 허용하고 나머지(`javascript:` 등)는 **앵커 대신 평문**으로 렌더(브라우저 실측 확인). **HTML 주석은 파싱 전에 제거** — 서버 초안의 `<!-- 왜 그렇게 보는지 -->` 안내가 발행 후 독자에게 보이지 않게 하려는 것
  - **에디터**(`components/ui/markdown-editor.tsx`): 툴바 10종(제목·굵게·기울임·목록·번호목록·인용·링크·코드·구분선·표)이 선택 영역을 감싸거나 줄 앞에 붙이고, `작성`/`미리보기` 탭의 미리보기는 **상세 화면과 같은 렌더러**를 쓴다(렌더러 2개가 갈라지지 않게). 좁은 화면에서 툴바는 줄바꿈이 아니라 가로 스크롤
  - **레이아웃을 한 컬럼 중앙 정렬로 통일**: 08-09에 compose/detail을 `wide`(2단)로 뒀던 것을 **`reading` 1단으로 전환**. 상세는 제목 → 요약 카드(기준가/목표가/손절가/기간/확신도 + 알파·수익률) → 본문 → 발행 시점 근거 → 반응 순으로 모든 폭에서 동일 순서. 목록은 블로그 인덱스형 카드(제목·발췌·읽는 시간·조회/좋아요/댓글). 상세의 글 제목은 `h2` — `PageHeader`가 이미 페이지의 `h1`
  - **서버 초안도 블로그 골격으로**(`research_draft.go`): `## 한 줄 요약` / `## 투자 논거` / `## 무엇이 확인되면 틀린 것인가` / `## 발행 시점 데이터(자동 생성)`. LLM 0콜 유지
  - 관리자 화면 라벨도 동기화: 배치 카드 `투자 리포트 판정`, DB 관리 카탈로그 `투자 리포트`/`리포트 댓글`… (테이블명은 `research_*` 유지)
  - 검증: go build/vet/test 전체 통과, tsc/`next build`(라우트가 `/reports*`만 남고 `/research*` 소멸), eslint 신규 3파일 무경고(기존 `set-state-in-effect` 4건은 이전부터 있던 패턴), Docker 8컨테이너 재기동(Tesseract OK). **브라우저 실측**: 사이드바 `리포트 > 투자 리포트` 단일 항목·리더보드 없음, `/research`·`/research/leaderboard` 404, `/reports/leaderboard`도 404(동적 세그먼트가 숫자가 아니면 notFound), 상세 렌더, 작성 화면 초안 1,157자 자동 주입, 미리보기에서 표 정렬(left/right/center)·중첩 목록·코드블록·인용·취소선 렌더 + `javascript:` 링크 평문화 확인, 375px에서 목록·상세·작성 모두 가로 오버플로 0
  - 남은 것: 종목 화면 `이 종목 리포트 N건` 섹션(API는 있음), 팔로우·`@handle`, 자동 썸네일
- 08-10: **리서치 콜 — 스탬프 결합 해제 + Phase 2~3(리더보드·소셜) 구현** ※ 리더보드는 위 08-10 개편으로 제거됨
  - **스탬프와 분리 확정(사용자 결정)**: 두 기능은 연동하지 않는다. ① 스탬프 상세의 `콜로 발행` 버튼 제거 ② **리서치가 스냅샷 타입을 직접 소유**(`ResearchContext`/`ResearchNews`/`ResearchIndex`/`ResearchChart`/`ResearchCurrent`/`ResearchReturn`, FE도 동일) — 다른 기능이 바뀌었다는 이유로 바뀔 수 있는 스냅샷은 박제된 게 아니다 ③ 공용이던 시계열 원시 연산을 중립 파일 `service/close_series.go`로 추출(`closeSeries`/`closePctChange`/`resolveHorizonClose`/`benchmarkForStock`/`marketTradeDate`) — 스탬프 파일 안에 있으면 리서치가 스탬프 위에 얹힌 것처럼 보인다. **주의**: `pctChange`는 `xbrl_select.go`에 이미 있어(반환값 1개) `closePctChange`로 명명
  - **038 소셜 테이블**: `research_reactions`(좋아요·북마크, kind enum), `research_comments`(1단계 대댓글, soft delete), `research_ratings`(3축), `research_views`(조회 중복제거). **카운트를 `research_calls`에 비정규화하지 않음** — 행과 어긋날 수 있는 카운터가 쿼리 하나보다 나쁘다. 피드는 페이지 크기와 무관하게 고정 쿼리 수(`CountsFor`/`MyReactions`)
  - **리더보드**(`research_leaderboard.go`): 순위 키 = **연율화 알파 × 페널티**. 연율화는 ±200%p 클램프(30일 +40%를 12배 하면 지수를 재는 것), 30일 미만은 연율화 안 함. 페널티 2종 — 월 8건 초과 발행, 동일 종목 반복(1건당 3%p), 하한 0.5. **판정 10건 미만은 순위에서 빼고 '관찰 중'으로 분리**. 철회 콜은 성과 평균에서 빠지되 발행 수·철회율엔 남는다
  - **평가는 판정 전에만**, 자기 콜 평가 금지(409). 결과를 보고 매기면 논거 평가가 아니라 성과 재측정이고 그건 알파가 이미 더 잘한다
  - **조회수는 상세 조회의 부수효과가 아니라 별도 엔드포인트**(`POST /research/calls/{id}/view`). 상세 fetch에 얹으면 리렌더·폴링마다 올라 렌더 카운터가 된다. 뷰어 키 = 로그인 user id 해시 / 비로그인 IP+UA SHA-256, **원문 미저장**. 30분 중복 제거
  - **FE**: `/research/leaderboard`(기간 3개월·12개월·전체 × 시장 탭, 내 순위 고정 표시), `/research/authors/[userId]`, 상세 화면 반응 바·댓글 스레드·3축 평가 패널, 피드 카드 조회/좋아요/댓글/평가 카운트. 사이드바 `리서치 > 리더보드` 추가
  - 검증: go build/vet/test 전체(신규 유닛 14건 — 리더보드 표본 게이트·연율화·클램프·페널티 2종·철회 처리·알파 정렬·캘리브레이션·뷰어 키 해시), tsc/`next build`, Docker 8컨테이너 재기동(Tesseract OK, 038 적용). **브라우저 실측**: 좋아요 토글 on/off, 북마크, 댓글+대댓글, **대댓글의 대댓글이 조부모로 평탄화**, 자기 콜 평가 409, 2번째 계정으로 3축 평가(3.7/5) 렌더, 조회 3회 호출에도 카운트 1 유지, 375px 오버플로 0. **리더보드 실측**: 픽스처 24건으로 "수익률 +60%/알파 +1%p" 작성자가 "수익률 +6%/알파 +18%p" 작성자에게 밀리는 것(0.73 vs 18.00) 확인 후 픽스처 삭제
  - 남은 것: 팔로우·`@handle`, 자동 썸네일, 종목 화면 `이 종목 콜 N건` 섹션(API는 있음), 시즌제 대회
- 08-09: **리서치 콜 Phase 1 구현** (`docs/RESEARCH_CALL_DESIGN.md` §16)
  - **범위 확정**: 설계안 §5.4 "계좌 연동 기반 실행 일치도"는 **사용자 결정으로 제외**. 콜 어디에도 보유·계좌 정보가 들어가지 않는다(차별점 8개 → 7개)
  - **BE**: `037_research_calls.sql`(`research_calls`/`research_revisions`/`research_reports`, `MigrateResearchIfNeeded`) + `ResearchRepo`/`ResearchService`/`ResearchHandler`. 공개 읽기는 `OptionalAuth`(`GET /research/calls*`), 쓰기는 `RequireUserID`. **DELETE 엔드포인트를 두지 않음** — 지울 수 있는 콜은 적중률을 반증 불가능하게 만든다. 철회는 `status='retracted'`로 남고 철회율에 반영
  - **불변/가변 분리**: 방향·기준가·목표가·손절가·기간·확신도·근거 스냅샷은 발행 후 UPDATE 금지. `UpdateBody`는 title/body/tags/visibility만 쓰고, 본문이 실제로 바뀔 때만 `body_rev`를 올리며 `research_revisions`에 append(가시성만 바꿔도 "수정 3회"가 되지 않게). **실측 검증**: PATCH에 `direction`/`targetPrice`/`basePrice`/`conviction`/`horizonDays`를 실어 보내도 전부 무시되고 rev만 2로 증가
  - **근거 수집은 전적으로 서버 책임**(스탬프 capture 패턴 재사용): 시세·5축 점수·백분위·커버리지·PER/PBR·52주·뉴스 5건·공시(캐시 전용)·지수·`stock_briefing.go` 룰 기반 브리핑. **LLM 0콜** — 자동 초안도 브리핑에서 생성. 온디맨드 LLM을 기본 경로에 넣지 않는다(08-09 앞선 세션과 동일 원칙)
  - **판정 엔진**(`research_performance.go`): `target_hit`/`stop_hit`(선도달 우선)/`expired_hit`/`expired_miss`/**`expired_unknown`**(만기는 지났으나 판정할 종가 없음 — 적중률 분모 제외). **종가만 사용** — 장중 고가 터치를 인정하면 유동성 낮은 종목의 순간 스파이크가 목표 달성이 된다. 데드라인 이후 종가는 목표 달성으로 치지 않음(늦게 들어온 백필이 만기를 늘리지 못하게)
  - **목표 도달 확률**(§5.5)은 `candle_bars` 일봉의 겹치는 전 구간으로 실측. `stock_daily_closes`는 커버리지가 얕아(005930 27행) 못 씀. 겹치는 구간 30개 미만이면 확률 대신 `insufficient` — "모른다"를 0%로 표시하는 게 가장 위험
  - **가드**: 일 5건 / 동일 종목 30일(409 + 남은 일수), 모순된 목표·손절 쌍 400 거부(기대값·손익비 부호가 뒤집힘), 공개 발행 시총 하한(KRW 300억 / USD 3천만, **시총 미상이면 차단하지 않음** — 못 잰 종목에 "너무 작다"고 하는 건 거짓), 신고 + 관리자 숨김을 소셜 기능보다 먼저
  - **배치**: `BatchResearchVerdict` + `cmd/worker-research`(신규 6번째 워커, compose·Dockerfile·`restart.ps1` 등록). **07:00 KST** — KRX 마감+20분·US 마감+20분(당일 05:20~06:20 KST) 둘 다 지난 시각. DB 전용이라 외부 호출 0
  - **FE**: 사이드바 신규 그룹 `리서치`(공개물이라 사적 기록 그룹과 분리), `/research`(내 콜·공개 피드 토글 + 성적표 4타일), `/research/new`(콜 값 ↔ 에디터 2단, 실측 지표 가격대 칩으로 목표가 스냅), `/research/[id]`. **원수익률 정렬 옵션을 두지 않음** — 알파순이 기본
  - **레이아웃 함정**: compose/detail은 2단이라 `reading`(뷰포트 68%)에서 1280px일 때 본문 입력칸이 **236px**까지 눌렸다 → 헤더·본문 모두 `wide`로 전환(640px 확보). 375px 가로 오버플로 0
  - 검증: go build/vet/test 전체(신규 유닛 24건 — 판정 9·요약 4·도달확률 4·EV 2·경계/게이트 5), tsc/`next build`, Docker 7컨테이너 재기동(Tesseract OK), **브라우저 실측**(테스트 계정으로 000660 실제 발행 → 근거 62/53점·1,938개 구간 실측 확률 28.3%·EV +6.93%·손익비 1.23:1 렌더 확인, 409 중복 가드, PATCH 불변성), **판정 배치 실측**(백데이트 픽스처로 `expired_miss` -17.65% / 알파 -3.88%p 산출 후 픽스처 삭제)
  - 남은 것: 리더보드·좋아요/댓글·평가 점수·팔로우는 Phase 2~3. **관리자 자격증명이 없어 `/admin/batch`의 "리서치 콜 판정" 카드와 관리자 숨김 API는 육안 미확인**(워커 큐 경로는 실제 enqueue→claim→done으로 확인)
- 08-09: **리서치 콜 설계안 작성** (`docs/RESEARCH_CALL_DESIGN.md`, 코드 변경 0)
  - Valley AI `투자 인사이트/예측대회`를 벤치마크로 받아 **우리 데이터로만 가능한 형태**로 재설계. 핵심은 "작성자가 쓴 숫자"가 아니라 **서버가 박제한 근거**(5축 점수·백분위·커버리지·시세·뉴스·공시)를 콜 옆에 나란히 두는 것
  - 차별점 8개: ① 근거 카드 서버 박제 ② **원수익률이 아니라 알파로 순위**(벤치마크 = `index_daily_closes`) ③ 콜 값 불변 + 본문 수정 이력 공개 + 삭제해도 통계 잔존(체리피킹 차단) ④ 계좌 연동 기반 **실행 일치도**(비중 구간만, 금액 비노출) ⑤ `stock_briefing.go` 룰 기반 **자동 초안**(LLM 0콜)으로 작성 마찰 제거 ⑥ 목표가 입력 시 `candle_bars` 실측 **도달 확률** 표시 ⑦ 확신도 캘리브레이션(Brier) 채점 ⑧ **역발상 콜 적중률 → `stock_scores` 예측력 검증 루프**로 환류
  - 재사용 전제: 스탬프의 불변/가변 분리·거래일 경계·알파 계산(`stamp_performance.go`)을 확장, `batch_job_runs` 큐 + 전용 워커(`worker-research`)를 종가 워커 뒤에 연결. **외부 API 호출 0**
  - LLM 예산: 기본 경로 전부 LLM 0콜(썸네일도 SVG 자동 생성). 온디맨드 LLM을 기본 경로에 넣지 않는다 — 08-09 앞선 세션에서 같은 이유로 온디맨드 요약을 제거했음
  - **명칭 충돌 주의**: `service/insights.go`(대시보드 인사이트)가 `insights`를 점유 → 신규는 `research_*` 접두어
  - 규제 제약을 설계에 선반영: 보상 구조 없이 시작(유사투자자문업), 서비스가 특정 콜을 추천하지 않음, 소형주 펌핑 방지용 종목 게이트·신고·관리자 숨김을 **Phase 1에 포함**
  - 로드맵 Phase 1(소셜 없이 검증 가능한 콜) → 2(피드·리더보드) → 3(커뮤니티·실행 일치도). **미결정 7건은 문서 §17** — 사용자 확인 후 구현 착수
- 08-09: **AI 요약 배치 수동 실행 — 휴장일에도 종가 기준으로 돌 수 있게 수정**
  - **문제**: `runMarketBatch`(`stock_summary_batch.go`)의 "오늘이 거래일인지" 체크가 스케줄 잡·수동 트리거(`RunBatchNow`) 양쪽에 그대로 걸려 있어, 관리자가 휴장일(주말·공휴일)에 배치 관리에서 수동 실행을 눌러도 `"OO 시장은 YYYY-MM-DD에 휴장입니다"` 에러로 항상 거부됐음. 대상 선정 자체는 마켓 레이더가 저장해둔 최신 랭킹(=휴장일엔 직전 거래일 데이터)을 읽을 뿐이라 실행을 막을 이유가 없었음
  - **수정**: `batchRunWindow`에 `asOf time.Time` 필드 추가(zero면 기존처럼 실행 시점 `time.Now()`로 거래일 판단 — 스케줄 경로는 무변경). `RunBatchNow`가 트리거 시점에 오늘이 휴장이면 `calendar.LastSessionCloseAt(schedule, now)`(기존 함수 재사용, 종가 기준 "가장 최근 완료된 세션")로 직전 거래일을 구해 `window.asOf`에 넣고, `runMarketBatch`는 `asOf`가 채워져 있으면 휴장일 가드를 건너뛰고 그 날짜를 `tradeDate`로 사용
  - 신규 유닛 1건(`TestRunMarketBatchManualUsesLastCloseOnHoliday` — 2026-08-15 광복절/토요일에 `asOf`가 2026-08-14로 정확히 해석되고 휴장 가드 대신 다음 체크(LLM 비활성화)까지 통과하는지 고정), go build/vet/test 전체 통과, Docker 재기동(Tesseract OK, 기동 로그에 llm/batch 에러 없음). **관리자 자격증명이 없어 배치 관리 화면에서 실제 휴장일 수동 클릭은 미확인** — 코드 경로는 유닛 테스트로 고정
- 08-09: **AI 요약 배치(스케줄 잡)를 배치 관리(/admin/batch)에서 수동 실행 가능하게 연동**
  - **동기**: AI 요약 배치는 `stockSummarySvc.Start(bgCtx)`로 API 프로세스 안에서만 스케줄 트리거(KR 08:00 KST / US 08:30 ET)로 돌고 있었고, `/admin/llm`의 "수동 생성"은 임의 종목 최대 10개만 즉석 재생성하는 별개 경로였음 — "오늘 스케줄이 실패했으니 지금 전체를 다시 돌리고 싶다" 같은 요청을 처리할 manual trigger가 배치 관리 쪽엔 없었음
  - **BE**: `model.BatchLLMSummaryDomestic`/`BatchLLMSummaryOverseas` 신규 잡 ID 2개(`batch_job.go`) 등록, `batchJobCatalog`(`batch_admin.go`)에 카드 추가(Cost=`rate_limited`, since/limit 미노출 — 항상 오늘 레이더 top 대상). `runMarketBatch`(`stock_summary_batch.go`)를 `(*model.LLMBatchRun, error)` 반환으로 바꾸고 신규 `RunBatchNow(ctx, market)`이 스케줄 창 대신 관대한 수동 창(2시간)으로 즉시 호출 — "오늘 이미 실행됨" 가드를 우회하는 게 의도(관리자가 명시적으로 다시 돌리는 것). `beginMarketRun`/`endMarketRun` mutex 가드 신설 — 스케줄 잡과 수동 트리거가 같은 시장에서 동시에 돌면 같은 Redis run/history 키·예산 체크가 레이스 나던 걸 방지
  - **LLM 배치엔 전용 워커 프로세스가 없음**(다른 5개 배치와 다른 점): `StockSummaryService`가 fundamentals/news/insider/dart/edgar/LLM client 등 API 프로세스의 거의 전체 의존성 그래프를 필요로 해서 분리 비용이 큼 — 대신 `RunBatchQueueWorker`(기존 `batch_job_runs` 큐 폴러 추상화, 원래 `cmd/worker-*`용)를 API 프로세스 안에서 두 잡 ID에 대해 goroutine으로 그대로 돌림(`server.go`). Trigger→enqueue→Catalog 조회 경로는 다른 5개 배치와 완전히 동일한 코드
  - 신규 유닛 5건(스케줄 조회, 동시 실행 가드, 미지 시장 거부, 카탈로그 등록, nil 서비스 커버리지) + go build/vet/test 전체 통과, Docker 재기동(Tesseract OK). `/admin/batch/jobs` 401(비로그인) 확인. **관리자 자격증명이 없어 실제 클릭 실행은 미확인** — 이 세션 로컬 컨테이너엔 `LLM_*` env가 비어 있어 `llm.Enabled()`가 false이므로, 관리자가 실제 로그인해 실행하면 즉시 quota 소모 없이 "LLM이 비활성화되어 있습니다" 오류로 안전하게 끝나는 경로부터 확인될 것
- 08-09: **AI 요약을 마켓 레이더 1일 top100(파생상품 제외) 자동 배치로 전환 + LLM 토큰 한도 이중 가드**
  - **동기**: 파일럿 2종목(005930·000660)만 LLM을 돌리고 있었음. 실측 결과 Gemini 무료 티어 일일 500콜 중 하루 11~49건만 사용 중이라 400건 가까이 놀고 있었고, 대상만 정하면 수백 종목이 들어감
  - **대상 선정**(`llm_batch_targets.go`): 레이더가 이미 15분마다 저장하는 **1일 기준 거래대금+거래량 top 100**(KR/US 4슬롯)의 합집합에서 ETF·ETN·ETP 제외. **외부 API 호출 0** — Redis/MySQL에 저장된 payload만 읽음. 실측(08-09): KR 155→**105종목**(파생 50 제외), US 171→**136종목**(파생 35 제외) = **하루 241콜**. `model.IsFundamentalsApplicable`은 ETF/ETN만 알아서 **미국 ETP 11건이 통과**하므로 재사용하지 않고 별도 판정 함수를 둠
  - **🔴 토큰 한도 가드 신설**: 무료 티어는 요청 수뿐 아니라 **분/일 토큰**도 제한하는데 기존 `apibudget`엔 TPD가 아예 없었고 TPM은 **사후 차단**(이미 쓴 양이 한도에 닿아야 거부)이라 큰 호출 하나가 그대로 넘길 수 있었음
    - `Limits.TokensPerDay` + `tokens_day` 카운터 신설, `RecordTokens`가 분·일 양쪽 적립
    - **`CheckTokens(estimate)` 사전 차단**: 프롬프트에서 토큰을 추정(`llm.EstimateTokens` — 한글 1자=1토큰, 라틴 3자=1토큰으로 **과대** 추정)해 **넘길 호출 자체를 거부**
    - 응답에 `usage.total_tokens`가 없으면 **추정치를 대신 기록** — 미계측 호출이 조용히 한도를 넘지 않도록
    - 전 한도를 env로 재정의 가능(`LLM_DAILY_QUOTA`/`LLM_PER_MINUTE`/`LLM_TOKENS_PER_MINUTE`/`LLM_TOKENS_PER_DAY`). **TPD 기본 1M은 공표값이 아니라 자체 상한** — provider 콘솔 값으로 맞출 것
  - **스케줄을 시장별 개장 전 마감으로 전환**(`stock_summary_schedule.go`): 기존 00:00 KST 일괄 → **KR은 08:00 KST**(09:00 개장 1시간 전), **US는 08:30 ET**(09:30 개장 1시간 전 = 서머타임 21:30 KST / 표준시 22:30 KST). 데드라인에서 `대상수 × 6초 + 10분`을 **역산해 시작**, 최대 2시간 전으로 클램프. **휴장일엔 실행하지 않음**(`calendar.IsTradingDay`, 내장 휴장일 JSON). 데드라인을 시장 현지시각으로 표현해 미국 서머타임이 자동으로 따라감
  - **예산 가드 + 이월**(`stock_summary_batch.go`): 시작 전 `Headroom()`에서 예비분(요청 80·토큰 12만 — 번역/브리핑/수동 생성 몫)을 뺀 만큼으로 대상을 **미리 자름**(잘린 종목은 호출 자체가 없음). 실행 중에도 매 종목 전에 데드라인·잔여 한도를 확인해 **첫 위반에서 중단하고 남은 종목을 다음 실행으로 이월**. 간격은 관측 토큰 EMA로 TPM의 60% 이내가 되도록 자동 연장(현재는 6초 하한이 지배)
  - **🔴 온디맨드 LLM 호출 제거**: 기존엔 캐시가 6시간 지난 파일럿 종목을 **사용자가 조회하기만 해도** LLM을 다시 호출했음 — 241종목으로 확대하면 조회 트래픽만으로 하루 최대 4배. LLM 소비 경로를 **배치 + 관리자 수동** 2개로 한정. 대신 규칙 기반 재빌드가 **직전 LLM 서술을 그대로 승계**(수치는 최신, 문장은 유지)하도록 해 요약이 몇 시간 뒤 규칙 문장으로 되돌아가던 문제도 같이 제거. 뉴스 감성 집계도 함께 승계(LLM만 산출하는 값이라 빠뜨리면 감성 배지·일별 점수의 뉴스 축이 사라짐). 새 `analyzedAt` 필드가 문장이 실제로 쓰인 시각을 보존
  - **설정**(`036_llm_batch_settings.sql`): `target_mode`(radar|manual)·`daily_cap`(250)·`quota_reserve`·`token_reserve` 추가, `pilot_symbols`는 **"항상 포함되는 고정 종목"**으로 의미 전환(TEXT, 상한 10→50). `/admin/llm`에 시장별 다음 실행·개장 전 마감·현재 대상 수·제외 사유·최근 실행 결과·잔여 한도 표시
  - **마이그레이션 함정**: `splitSQLStatements`가 `;`로 단순 분할해서 **SQL 주석 안의 세미콜론이 문을 쪼갬**(기동 실패로 발견). 036 주석에 경고 명시
  - 검증: go build/vet/test 전체(신규 유닛 20건 — 스케줄 6·선정 5·토큰 추정 5·예산 6), Docker 재기동(Tesseract OK, 036 적용), **실제 DB/Redis로 선정 결과 실측(KR 105 / US 136)**, 신규 admin 라우트 401, tsc/next build 통과. `/admin/llm` 육안 확인은 관리자 자격증명이 없어 미완료
- 08-09: **전 메뉴 페이지 헤더 통일** (FE만, BE 무변경)
  - **점수 관리·배치 관리 실제 버그**: `PageHeader`가 `PageScrollShell` **안쪽**에 중첩돼 있었음 — `PageScrollShell`의 `page-shell`(거터) + `py-4 sm:py-6`이 `PageHeader` 자신의 `page-shell` + `py-3 sm:py-4` 위에 또 씌워져 다른 관리자 메뉴보다 헤더가 눈에 띄게 크고 더 들여써짐. `PageHeader`를 `PageScrollShell`보다 **앞의 형제**로 이동(다른 모든 페이지와 동일 패턴)해서 해결
  - **마켓 레이더는 사용자가 재검토 후 `wide`→`reading` 통일 선택**: 앞선 세션에서 `wide` 아케타입으로 헤더·본문 폭 불일치는 고쳤지만, 화면(1706px)에서 여전히 `reading` 메뉴들과 좌측 여백·헤더 높이가 달라 보여 "안 똑같다"는 피드백. `PageHeader` `width` prop 제거(기본값 `reading`) + 본문 `page-shell-wide` → `page-shell`로 교체해 다른 모든 메뉴와 완전히 같은 컬럼 규칙 사용
  - **폭이 좁아진 만큼 랭킹 리스트 접기 기능 추가**: `reading`은 좁은 화면에서 `wide`보다 먼저 줄어들어 리스트+차트 동시 표시 공간이 부족해질 수 있음 — 리스트 컬럼과 워크스페이스 사이에 접기/펼치기 토글 버튼(`listOpen` state) 신설, 접으면 차트가 전체 폭 사용. `/quotes`·`/stocks/[symbol]`은 이번엔 미변경(마켓 레이더만 사용자가 지목) — 동일 리스트+워크스페이스 패턴이라 후속으로 동일 처리 필요할 수 있음
  - 검증: tsc 통과, 브라우저 실측(리스트 접기/펼치기 토글 정상 동작, 헤더가 `page-shell` 컬럼 사용 확인). 관리자 메뉴(점수·배치)는 로그인 자격증명 없어 육안 미확인 — 구조는 다른 정상 관리자 페이지와 동일 패턴으로 일치시킴
- 08-09: **레이아웃 아케타입 도입 — 마켓 레이더 중앙 정렬 통일** (FE만, BE 무변경)
  - **문제의 실제 원인**: "마켓 레이더만 전체 화면"이 아니라 **헤더와 본문의 컬럼 폭 불일치**였음. `PageHeader`는 `news-page-shell`(중앙 68%)을 쓰는데 `/market`의 `<main>`만 full-bleed → 1440px에서 제목 컬럼 x=415 w=830, 리스트 x=220 w=1220으로 어긋나 그 페이지만 겉돌아 보였음
  - **설계**: 폭을 하나로 맞추지 않고 **중앙 정렬 규칙·거터를 공유하고 max-width만 페이지 성격별로 선택**하는 아케타입 2종. `reading`(`.page-shell`, 기존 값 그대로) / `wide`(`.page-shell-wide`, **`--content-max-wide: 100%`**). `PageHeader`에 `width` prop 추가 — **본문과 같은 값을 넘기지 않으면 위 불일치가 그대로 재발**하므로 타입(`PageShellWidth`)과 헬퍼(`pageShellClass`)를 `page-content-shell.tsx`에 두고 양쪽이 공유
  - **최종 결론: 폭은 통일하지 않는다.** 상한을 112rem → 72rem으로 두 번 시도했으나 전부 되돌림
    - 112rem: 뷰포트 약 2012px 이상에서만 잘려 사용자 화면(1908px)에서 **변화가 0**("똑같은데?")
    - 72rem: 1908px에서 컬럼 1152px·좌우 268px로 `reading`(1148px)과 4px 차까지 일치시켰으나, 워크스페이스가 **1318 → 830**으로 줄어 **사용자가 보기 불편하다고 판단해 되돌림**. 터미널형은 밀도가 곧 기능
    - 남은 것은 "헤더 컬럼과 본문 컬럼이 같은 규칙을 쓴다"는 것까지. 다시 상한을 넣으려면 반드시 `min(<값>, 100%)` 형태 — `100%` 항이 없으면 `AppShell`의 `overflow-x-auto`가 좁은 화면에서 가로 스크롤을 만듦
  - **부수로 만들었던 버그 1건 수정**: `PageHeader`의 `wide` 분기에 `--content-gutter`(4vw)를 줬더니 1908px에서 제목이 76px 들여써져(x=296) 리스트(x=221)와 **새로 어긋남**. 터미널 패널과 같은 `px-3`으로 교체(제목 x=233 ↔ 첫 컨트롤 x=236)
  - `.page-shell-wide`에 좌우 경계선 없음 — 상한이 100%라 항상 꽉 차서 좌측선이 사이드바 테두리와 겹쳐 이중선이 되고 우측은 뷰포트 가장자리에 붙음
  - 적용: `/market`, `/quotes`(지수 바 포함), `/stocks/[symbol]`. `.news-page-shell` → `.page-shell` 리네임(뉴스 전용이 아니었음, 호출부 5개 + 헤더/스크롤셸)
  - 실측(최종, 1024/1440/1908): 헤더·본문 컬럼 x/w 완전 일치하며 **콘텐츠 폭 전체 사용**(1908에서 x=220 w=1688), 리스트/워크스페이스 320/472 · 320/888 · 368/1308, 전 구간 가로 오버플로 0
  - **되돌리기 중 발견해 함께 정리**: `market/page.tsx`가 `page-shell`(= reading 컬럼 830px)로 바뀌어 있어 `page-shell-wide`로 교정. 같이 들어와 있던 랭킹 리스트 접기 토글(`listOpen` + `ChevronLeft/Right` + 12px 세로 스트립)은 "reading으로 좁아진 폭 보완"이 목적이라 주석에 적혀 있었고 그 전제가 사라져 **사용자 요청으로 삭제**(state·import·JSX 전부, 잔여 참조 0). 워크스페이스 1308 → 1320 회복
  - **미확인**: 로그인 필요 데이터라 차트·호가 2단 분할이 실제로 그려진 상태는 못 봄(`시세 로딩 중…`에서 멈춤). `StockChartTerminal` 미변경 + 최종 폭이 원래대로라 회귀 가능성은 낮음
  - 검증: tsc, `next build` 통과. eslint는 `/quotes`의 기존 `react-hooks/set-state-in-effect` 4건만(이번 변경과 무관, className만 수정)
- 08-09: **스탬프 기능 신설 — 순간 스냅샷 + 가설 + 자동 채점** (`docs/STAMP_DESIGN.md`)
  - 사용자 요청(타임캡슐형 스탬프)을 그대로 만들지 않고 **가설(관점·확신도·검증기간)** 을 붙여 재방문 동기를 만드는 형태로 설계 후 승인받고 구현. 보존만 하면 두 번째 사용이 안 일어나고, 개봉·판정이 있어야 적중률·평균 알파가 쌓임
  - **BE**: `035_stamps.sql`(`MigrateStampsIfNeeded`, journal과 동일 idempotent 패턴) + `StampRepo`/`StampService`/`StampHandler`. `GET/POST /stamps`, `GET /stamps/{id}`, `PATCH`(가설만), `POST /stamps/{id}/open`, `DELETE`, `GET /stamps/summary`, `GET /stamps/state?symbol=` — 전부 `authctx.RequireUserID` + `WHERE id=? AND user_id=?`
  - **불변/가변 분리**: 시세·점수 5축·뉴스·공시·지수·차트 120봉은 생성 시 1회 복사 후 UPDATE 금지(수정 가능한 건 메모·태그·관점·검증기간뿐 — 스냅샷이 수정되면 적중률이 사후 조작 가능). 수익률/알파/판정은 조회 시 계산
  - **공시는 캐시 전용 복사**: 공시엔 이력 테이블이 없어(`GetTodayDisclosures` = Redis 당일 캐시) 나중에 복원 불가 → 생성 시 복사하고, 없으면 `dataGaps:["disclosures"]`로 남김(지어내지 않음). 스탬프 때문에 늘어나는 외부 API 호출 0
  - **수익률 경계 규칙**: "목표일 이하 마지막 종가"만 쓰면 **금요일 스탬프의 1D가 영원히 빈칸**(목표일=토요일→스탬프 당일 종가). 스탬프 이후 첫 종가를 쓰되 **목표일+5일 이내**로 제한(거래 끊긴 종목의 몇 달 뒤 종가가 1D를 채우는 것 방지). 미도래 구간은 `null`→`—`, 0% 대체 금지
  - **버그 2건(실측에서 발견)**: ① 장 마감 후 라이브 시세 거래량이 0이라 실제 거래된 세션이 "거래량 —"으로 박제됨 → `stock_daily_closes.volume` 폴백 ② 기존 `repository.nullFloat`이 `*v <= 0`을 NULL로 바꿔(보유수량용 헬퍼) **하락 종목 등락률이 통째로 유실**(000660 -1.94% → NULL) → `nullFloat64`로 교체, 재검증(035420 -6.64% 정상 저장)
  - **FE**: 사이드바 `기록 > 스탬프`(`/stamps`), 종목 헤더 관심 버튼 옆 도장 버튼(`StampButton`, 워크스페이스·레거시 상세 두 곳). **모달 먼저 띄우지 않음** — 클릭=즉시 저장 후 확인 패널에서 관점/기간/메모를 인라인으로 덧붙임(마찰이 사용률을 좌우). 5분 쿨다운은 서버 409(`retryAfterSeconds`)가 진실, 버튼 카운트다운은 보조. `/stamps`는 요약 4타일(총계·개봉대기·적중률·평균알파) + 종목/관점/판정 필터 + 타임라인/종목별 뷰, 행 확장 시 당시 vs 현재 2열·점수 5축 당시→현재·박제 차트(실선)+이후 종가(점선)·기간별 수익률(벤치마크/알파)·당시 뉴스/시장/AI 요약·dataGaps 명시
  - 검증: go build/vet/test(신규 유닛 15건 — 구간 경계·주말 브리지·gap 상한·알파·판정·정규화·US 거래일), tsc/next build, Docker 재기동(035 적용, Tesseract OK), 브라우저 실측(005930·000660·AAPL·035420 실제 스탬프 생성 — 점수 62/53/41, 뉴스 5건, 지수 5종, KST 시각, US는 `trade_date=NY 날짜`+`sp500` 벤치마크 확인, 409 쿨다운, PATCH 가설 반영, 375px 오버플로 0)
  - 남은 것: 개봉 알림은 사이드바/목록 배지까지만(푸시 인프라 없음), 자동 스탬프 제안·스탬프 2개 비교 뷰·메모장/목표가 연계는 미구현
- 08-09: **랭킹 크라운 → 금/은/동 메달 전환 + "AI 투자일지" → "메모장" 개편(백엔드 신규)**
  - **메달**: `score-ranking-panel.tsx` — `Crown` → `Medal`(lucide) 아이콘 교체, 1/2/3위 색상을 실제 금(#FFB300)/은(#9CA3AF)/동(#B36A3C)으로 지정. `getComputedStyle`로 렌더 색상 실측 확인
  - **메모장(기존 localStorage-only "AI 투자일지" 대체)**: 사용자에게 저장 방식(백엔드 DB vs localStorage 유지) + 핵심 기능(종목 연동/수정/검색·정렬/고정)을 먼저 확인 후 진행 — 전부 채택. 신규 `journal_notes` 테이블(`034_journal_notes.sql`, `MigrateJournalNotesIfNeeded` 패턴은 `batch_job_runs`와 동일하게 idempotent) + `JournalNoteRepo`/`JournalService`/`JournalHandler`, `GET/POST/PUT/DELETE /api/v1/journal/notes`(전부 `authctx.RequireUserID` + `WHERE id=? AND user_id=?` 소유권 검증). DB 관리 카탈로그(`db_admin_catalog.go`)에 `journal_notes` 행 추가
  - **FE**: 종목 연동은 기존 `StockCatalogSearchField` 재사용(신규 컴포넌트 안 만듦), 연동 종목 클릭 시 `analysisSymbolHref`로 AI 분석 탭 이동(랭킹 패널과 동일 패턴). 고정(pin)·수정은 인라인, 검색/필터는 로드된 전체 목록에 대해 클라이언트 사이드(개인 메모 규모상 서버 페이지네이션 불필요 판단). 사이드바 라벨 "AI 투자일지"→"메모장"(`BookOpen`→`NotebookPen`), `page-stubs.tsx`/`DESIGN.md`/`FE_ACCOUNT_FEATURES.md`의 잔여 구 명칭도 동기화
  - 검증: go build/vet, tsc/next build 통과. Docker 재기동(`034` 마이그레이션 무오류 적용 확인) 후 회원가입 테스트 계정(`journaltest0809@example.com`)으로 브라우저 실측 — 메모 생성(종목 연동 포함)·고정 토글·수정·검색 필터링·새로고침 후 데이터 유지 전부 확인. 삭제는 `window.confirm`이 자동화 도구에서 자동 취소되어 버튼 클릭 자체는 미확인이나, `DELETE` 엔드포인트를 직접 fetch로 호출해 정상 동작(200, 목록에서 제거) 확인
- 08-09: **종목 분석 탭에 국내/해외 점수 랭킹 패널 신설**
  - 사용자 요청: 종목 분석 탭이 항상 (기존 로직상 보유 비중 1위인) 임의 종목으로 시작해 둘러보기가 안 됨. 스크린샷(랭킹 리스트: 크라운·보유뱃지·추적일·시뮬수익률·점수)을 참고해 우리 서비스 스타일로 국내/해외 점수 랭킹을 붙여달라는 요청
  - **BE**: `GET /stocks/scores/top?market=domestic|overseas&limit=N` 신규(`handler/score.go`, `service/stock_score.go` `TopScores`, `repository/stock_scores.go` `TopScores`). `stock_scores`에서 `LatestScoredDate()`(= MAX(trade_date))로 오늘 배치 유무와 무관하게 "가장 최근 산출된" 날짜를 자동 선택 — 오늘 배치가 아직 안 돌았으면 자연히 어제 점수로 폴백됨(요청사항 그대로, 별도 분기 불필요). `stocks.asset_type`으로 국내(`domestic`)/해외(`us`+`etf`) 분리, `stock_daily_closes` LEFT JOIN으로 등락률·종가 동봉
  - **화면에 없는 필드는 지어내지 않음**: 스크린샷의 "거래 추적일"·"시뮬레이션 수익률"은 이 서비스에 대응 개념이 없어 제외 — 대신 실제 보유하는 데이터(등락률·종가·점수·등급)로 구성
  - **FE**: `ScoreRankingPanel`(신규, `components/analysis/score-ranking-panel.tsx`) — 국내/해외 토글 + 순위 1~3위 크라운, 보유/관심 뱃지(기존 market-radar 패턴 재사용, `use-score-rankings.ts`가 watchlist/holdings로 enrich), 행 클릭 시 `analysisSymbolHref`로 해당 종목 AI 분석 탭 이동. `AnalysisClient` 상단에 접기/펼치기 가능한 섹션으로 삽입(기본 펼침)
  - 검증: go build/vet, tsc, `next build`(prod 빌드 통과 확인 — 이 프로젝트는 `next build`에 eslint를 태우지 않음, 별도 `npm run lint`는 기존 코드에도 이미 있던 `react-hooks/set-state-in-effect` 다수 위반이 있어 이번 변경과 무관함을 `git stash`로 대조 확인). Docker 재기동 후 `/stocks/scores/top` 실측(국내/해외 각각 실제 순위·점수 확인, tradeDate=2026-08-07로 정상 폴백). 로그인 자격증명이 없어 회원가입으로 테스트 계정 생성 후 브라우저 실측: 랭킹 20건 렌더·국내→해외 토글 전환·행 클릭 시 해당 종목 AI 분석 이동·375px 모바일 가로 스크롤 없음 전부 확인
- 08-08: **전 서비스 시간 표기 KST 감사 — 백엔드 6곳·프론트 20여곳 실제 버그 수정**
  - **전제 확인**: 컨테이너 실측 `date` → UTC(`docker exec quant-port-api date` = UTC, KST와 9시간차). 이 세션 앞서 candle_bars 시간대 버그(§08-08 위 항목)의 원인이었던 바로 그 조건이 다른 곳에도 잠복해 있었음
  - **백엔드 실제 버그 6건**(전부 `time.Now()`/`now.Location()`을 KST 미변환 상태로 표시·큐키에 사용):
    1. `portfolio_history.go` `GetHistory` — `downsampleSnapshots(..., now.Location())`가 **컨테이너 로컬(UTC)** 을 라벨 타임존으로 사용 → 포트폴리오 자산 차트의 "15:04" 라벨이 실제로는 UTC 시각(KST−9h)이었음. `model.KST`로 수정 + 회귀 테스트 2건(`TestDownsampleSnapshotsLabelsAreKST`)
    2. `dart.go` `TodayDisclosures`/`FetchEarningsDisclosures` — "오늘" 공시 조회 기준일이 UTC 기준이라 KST 자정 전후 9시간 구간에서 DART에 잘못된 날짜를 요청할 수 있었음. `time.Now().In(model.KST)`로 수정
    3. `earnings.go` `FetchCalendar` — 동일 패턴(미국 실적 캘린더 조회 시작일)
    4. `market_data_store.go` — `GetTodayDisclosures`/`EnsureTodayDisclosures`/`refreshDisclosure`/`refreshInsightsData`의 Redis "오늘" 캐시키 4곳, DART 쪽 수정과 짝을 이뤄야 캐시 롤오버 시점이 실제 KST 자정과 맞음
    5. `market_index_store_batch.go` `eodKey` — 지수 EOD 1회 적재 완료 마커가 UTC 날짜로 키잉되어 있어 같은 KST 거래일에 중복 실행되거나 하루를 건너뛸 수 있었음
  - **프론트엔드**: `toLocaleString`/`toLocaleDateString`/`toLocaleTimeString` 호출 20여 곳에 전부 `timeZone: "Asia/Seoul"` 명시 추가(브라우저·SSR 로컬 zone 의존 제거). `format.ts`에 `formatDateTimeKST`/`formatTimeKST` 공용 헬퍼 신설, 관리자 패널 4곳의 중복 `formatDateTime` 로컬 구현을 이걸로 통일
  - **차트 크로스헤어 버그**: `chart-time.ts`의 `formatChartCrosshairTime`이 축 라벨(`formatAdaptiveChartAxisTime`, 이미 KST)과 달리 `date.getHours()` 등 **로컬 getter**를 써서 브라우저가 KST가 아니면 크로스헤어 시각이 축과 어긋날 수 있었음. `kstDateParts` 공용 헬퍼로 통일, `parseChartCandleDate`/`parseChartCandleEpoch`의 "오늘" 계산도 동일하게 수정
  - 검증: go build/vet/test 전체(신규 회귀 유닛 2건), tsc/next build, Docker 재기동 6개 컨테이너 정상. 의도적으로 남겨둔 것: `news_store_us.go`의 `time.Now().UTC()`는 Finnhub 조회 범위 파라미터로 저자가 명시적으로 UTC를 선택한 코드라 표시용이 아니라고 판단해 미변경
- 08-08: **배치 잡을 독립 워커 프로세스로 분리 + 국내/해외 일별 종가 완전 독립화**
  - **동기**: 관리자 수동 배치가 API 프로세스 안 고루틴으로 돌아 하나가 자원을 과점/hang 하면 API 자체가 영향받을 수 있었음. 국내/해외는 트리거 시각만 분리돼 있었고(§08-08 앞선 세션 확인) 실제 실행은 같은 프로세스·같은 루프 안에 있었음
  - **아키텍처 전환**: `batch_job_runs`를 큐로 전환(033 마이그레이션, `params JSON` + `queued` 상태). API 프로세스는 이제 **enqueue만** 하고(`BatchAdminService.Trigger`), 실제 실행은 전부 제거 — 각 잡 전용 워커 프로세스가 `ClaimNext`(`UPDATE...ORDER BY...LIMIT 1`, MySQL 원자적)로 폴링해 가져감(4초 주기)
  - **잡 5개 → 컨테이너 5개**: `worker-close-domestic`(KRX 마감+20분+평일 환율), `worker-close-overseas`(US 마감+20분+상시 BTC), `worker-scores`(00:25 KST, DB전용), `worker-index`(수동, Toss+FRED), `worker-fetch`(수동, fetch-stale/missing 2개 잡을 뮤텍스로 직렬화해 Toss 동시 호출 방지). 이미지는 `be/Dockerfile` 하나에 바이너리 6개(api+worker 5개)를 함께 빌드, `docker-compose.yml`에서 `command:`로만 분기
  - **국내/해외 완전 독립**: `IngestStockCloses`/`BackfillStocksFromCandleBars`에 `assetTypes` 필터 추가(`stocks.asset_type` 조인) — 기존엔 스케줄 트리거만 분리돼 있고 실제 SQL은 시장 구분 없이 전 종목을 대상으로 했음. 이제 domestic 워커는 `asset_type='domestic'`만, overseas 워커는 `IN ('us','etf')`만 건드려 서로 절대 겹치지 않음(테스트로 고정: `TestEveryAssetTypeAssignedToOneMarket`)
  - **배포 직후 크래시 발견+수정**: `worker-close-overseas`가 기동 즉시 패닉(`missing Location in call to Time.In`). 원인은 `calendar.ScheduleAlways`(BTC 등 24시간 자산)가 `loc: nil`로 정의돼 있었는데, 기존 코드는 이 스케줄로 `SessionCloseAt`을 호출한 적이 없어(진짜 "마감 트리거"로 쓰인 적이 없어서) 드러나지 않았던 잠재 버그. 신규 워커가 처음으로 이 경로를 태워 노출됨. `DAILY_CLOSE_DESIGN.md` §5.1("24/7 자산은 KST 자정 기준")에 맞춰 `loc=서울, closeHour=23:59`로 수정 + 회귀 테스트 추가
  - 실측: 큐에 직접 INSERT → 4초 내 워커가 claim → 실행 → done 기록까지 전 과정 확인(국내/해외/점수 3개 잡 개별 검증). 6개 컨테이너 전부 안정 기동
  - 검증: go build/vet/test 전체(신규 유닛 6건 — assetType 필터 분리·파라미터 검증·캘린더 회귀), Docker Compose 6개 서비스 기동, DB 직접 enqueue로 워커 클레임 실측
- 08-08: **배치 관리 메뉴 + 지수 카드 점수 + 전 종목 백필 완주**
  - **배치 관리**(`032_batch_job_runs.sql`, `batch_admin.go`, `/admin/batch`): 5개 잡(일별 종가 투영 / 종목 점수 / 지수 종가·점수 / 누락 세션 채우기 / 미보유 종목 채우기)을 목록·수동 실행·커버리지·최근 실행 기록으로 관리. **DB 전용 vs 외부 API 호출**을 배지로 구분해 한도 소모 여부를 실행 전에 알 수 있게 함
    - **동시 1건 제한(409)**: 조회 잡들은 앱과 Toss 한도를 공유하므로 병렬 실행은 빨라지는 게 아니라 앱 폴링을 굶김
    - 실행 기록을 DB에 영속화(`batch_job_runs`) — 기존엔 로그 한 줄뿐이라 "돌았는지"를 사후에 알 수 없었음. 재시작으로 `running`에 남은 행은 기동 시 정리(`MarkStaleRunsFailed`)하지 않으면 이후 실행이 영구 차단됨
    - FE는 실행 중일 때만 4초 폴링(끝나면 중단)
  - **지수 카드 점수**: 신규 `GET /indices/scores`로 12개 지수 점수·요약을 **1회 조회**(카드당 1회면 개요 화면이 이유 없이 느려짐). 대시보드 카드에 점수·등급 칩 + 요약 툴팁. 실측: 다우 61(B+), 코스닥 58(B), 코스피 50(B)
  - **전 종목 백필 완주**: 커버리지 **1,469 → 16,225/17,901**(436,199행). 점수도 재계산해 **16,215종목 산출**(평균 50.0, 표준편차 12.6, 분포 정상). 잔여 1,676종목은 Toss가 `400`을 주는 미국 우선주 티커라 재실행으로 해결 불가
  - `batch_job_runs`도 DB 관리 카탈로그에 추가(read-only — 감사 기록이라 수정은 이력 위조)
- 08-08: **룰 기반 시장 분석 + 탭별 점수 + 지수 점수 + 통화 표기 + DB 카탈로그**
  - **룰 기반 시장 분석**(`stock_briefing.go`, LLM 미사용): AI 요약 앞에 붙는 prefix. ① 투자 포인트 최대 4개(종합점수 → 수익성 → 밸류에이션 → 실적 → 주가 → 내부자 순 우선순위, 톤 색상) ② **상승/하락 시나리오 + 주요 가격대**. 가격대는 전부 **실제 지표에서 유래**(SMA20/50/200·최근 스윙 고저·52주 범위) — 목표가를 지어내지 않음. 현재가 ±0.2% 이내는 실행 불가라 제외, 가까운 순 최대 3개. 실측(005930): 상승 50일선 258,260(+9.9%)·200일선 261,002·스윙고점 267,000 / 하락 20일선 234,200(-0.34%)·스윙저점 189,200·52주최저 165,700. **LLM 미설정·파일럿 외 종목에서도 항상 제공**
  - **탭별 점수**: AI 분석 탭에 종합 점수 상세(축별 막대·동종 백분위·커버리지, 커버리지 60% 미만이면 수축 사실 명시), 나머지 탭은 해당 축만 노출(차트=추세, 밸류에이션=가치, 재무제표=수익성, 실적=성장, 뉴스·내부자=심리). 점수는 워크스페이스 상위에서 1회 fetch해 공유
  - **지수 점수·요약**(`031_index_scores.sql`, `index_backfill.go`, `index_score.go`): 7/1부터 **9개 지수 241행** 백필 + 전 지수 채점. 소스별 **날짜 있는** 이력만 사용 — Toss 지표캔들(kospi/kosdaq), FRED `SeriesHistory`(신규 추가, nasdaq/nasdaq100/sp500/dow/vix), `stock_daily_closes`(nq100=QQQ, sox=SOXX는 실제 ETF라 API 호출 0). **스파크라인은 날짜가 없어 사용 불가** — 날짜를 추정하면 전 행이 어긋남. 지수는 재무제표가 없으므로 기술적 단독 점수이고, 성격이 다른 12개(주가·FX·암호화폐·금)는 서로 피어가 아니라 **횡단면 순위를 쓰지 않음**. btc·gold는 날짜 이력 소스가 없어 이후 일별 배치로만 누적. 실측: KOSPI 50점 + "최근 5거래일 -5.1%, 20일선 아래, RSI 42.2" 자동 요약. `ScoreService.RunScheduler`에 편승해 1일 1회
  - **통화 표기**(`formatFinancialAmount`): 재무 차트 Tooltip·축에 포매터가 없어 `240000000`이 그대로 노출되던 것 수정(국내 조/억/만원, 해외 B/M/K). 기존 `formatTradingValue`는 `<= 0`을 "—"로 렌더해 **영업적자가 숨겨지던 버그**가 있어 부호를 다루는 신규 함수로 분리, `earnings-tab`의 로컬 우회 래퍼 제거
  - **DB 관리 카탈로그**: 누락 8개 추가 — `stock_daily_closes`/`index_daily_closes`/`stock_scores`(단방향 투영·배치 산출물이라 read-only), `llm_settings`/`score_settings`/`score_symbol_weights`(편집 가능), 레거시 백업 2개(read-only, 수동 DROP 대기)
  - 검증: go build/vet/test(신규 유닛 9건 — 가격대가 실제 지표에서만 유래하는지 고정), tsc/next build, 지수 백필·점수 실행, API 실측
- 08-08: **candle_bars 시간대 버그 수정 + LLM 입력 76% 압축(점수 연동)**
  - **🔴 시간대 버그 수정 완료**: `UpsertBatch`가 KST `time.Time`을 넘기는데 DSN이 `loc=Local` — UTC로 도는 컨테이너에서 `D 00:00 KST`가 `D-1 15:00`으로 저장됐고, 호스트 도구(KST)는 `D 00:00`으로 저장해 **두 표기가 공존**. `dailyCloseSessionBars`의 "날짜당 마지막 봉" 규칙이 UTC 행을 골라 **`trade_date`가 하루 밀림**. 일요일(2026-08-02) 15:00 봉 존재가 결정적 증거. **`DAILY_CLOSE_DESIGN.md` §12.1의 "00:00=전일 종가 복제" 진단은 오진**이었고 정정 주석 추가
    - 수정: 일봉(`IsDailyOrCoarser`)은 KST 벽시계 **문자열**로 저장 → 프로세스 시간대와 무관하게 결정적. 1분봉은 Go 읽기 경로(`bt.In(KST)`)가 대칭이라 미변경
    - 보정: `cmd/repair-candle-tz`(드라이런 기본). 149,803행 처리, **무손실 병합** — 세션별로 거래량이 큰 쪽(완성된 봉)을 남김. 결과 일봉 673,842행 전부 00:00
    - **드라이런이 사고 2건을 막음**: ① `bar_interval IN ('1d','1w','1M')`이 MySQL 대소문자 무시 콜레이션 때문에 **`'1m'` 분봉 280만 행까지 매칭** ② 제자리 `UPDATE bar_time`은 살아있는 유니크 인덱스에 부딪혀 실패(`Duplicate entry 'GEV-1d-2026-08-05'`) → `INSERT…ON DUPLICATE KEY`로 충돌마다 명시 해소
    - 검증: 005930 `08-07 235,000(+2.17%) / 08-06 230,000(-4.96%) / 08-04 242,000(+3.64%)` 정상 복구
  - **LLM 입력 재설계**(`stock_summary_facts.go` 신설): 호출당 **10,728자 → 2,621자(76%↓)**. 원지표 구조체·분기 재무제표 4개·내부자 거래 10건·기사 요약문을 전부 제거하고 **서버가 분류한 결과만** 전달(`trend:"bearish"`, `macd:"recovering"`, `rsiState`, `earnings.trend`, `insider.direction`). 뉴스는 기존 `dedupeNewsArticles` 재사용 후 **헤드라인만**
    - **차별화 핵심 — 종합 점수를 프롬프트에 투입**: `score.axes`/`strengths`/`weaknesses`/`peerTopPercent` + 지표별 피어 백분위(`pbrPeerPct` 등). 프롬프트를 "수치 나열 금지, 축 간 **긴장 관계의 인과**를 설명하라"로 재작성. 이전엔 점수가 프롬프트에 아예 없어 **요약과 옆에 표시된 점수가 서로 다른 근거로 말하고 있었음**
    - 실측 개선: "펀더멘털의 우수함과 주가 추세 간의 괴리가 현재 변동성을 키우는 핵심 요인", "PBR 상위 2%… 시장 기대치가 이미 주가에 반영" — 인과·우선순위·동종 대비가 실제로 등장
    - 부수: 소수점 반올림(`per: 24.934903853743542` → `24.9`) — 토큰 낭비이자 허위 정밀도, `dataGaps`로 결측 영역 명시해 전반적 헤지 방지
  - 검증: go build/vet/test(신규 유닛 8건 — 분류 경계·중복 제거·페이로드 크기 상한), Docker 재기동, 000660 실제 LLM 호출 재생성
- 08-08: **종합 점수 구현(전 종목) + 관리자 점수 관리 + AI 요약 카드 재설계**
  - **종합 점수**(`docs/STOCK_SCORE_DESIGN.md` 설계 → 구현): 마이그레이션 `030_stock_scores.sql`(`stock_scores` + `score_settings` + `score_symbol_weights`). 5축(추세30/수익성25/성장20/가치15/심리10). 배치 `cmd/compute-scores` — **외부 API 호출 0회**, 17,901종목 **5초**(일봉 스트리밍 1패스, 심볼 경계에서 지표 계산해 메모리 상수)
  - **핵심 설계 3가지**: ① 모든 지표를 **피어그룹 순위 백분위**로 정규화(측정된 ROE 227,162% 같은 꼬리값 때문에 절대 임계값 불가) ② **도메인 가드** — 윈저라이즈만으로는 순위에서 이상치를 못 막음(값을 잘라도 여전히 1위). 범위 밖은 결측 처리. 음수 부채비율(자본잠식)은 반전 시 최우량으로 뒤집히므로 하한 0 ③ **커버리지 수축** `50+(raw-50)×min(1,cov/0.6)` — 재정규화만으로는 "RSI 하나로 90점"을 못 막음
  - **지표별 최소 표본 폴백**(`minMetricSamples=20`): 그룹 크기만 보면 안 됨 — 피어에 수백 종목이 있어도 ROE 보유가 2~3개면 백분위가 0/50/100으로 튐. 실제로 삼성전자 성장 축이 0점으로 나왔고, `peer → assetType:market → assetType → *` 폴백으로 해결(005930 성장 0→66)
  - **추세 축도 순위 정규화**: 절대 곡선은 구조적으로 중심이 위(RSI 50 → 68점)라 전체 평균이 53.1로 밀려 자체 분포 게이트에 걸림. 2단계(절대 곡선으로 "좋은 셋업" 판정 → 피어 내 순위)로 전환 후 **평균 49.9 / 표준편차 12.6 / 0·100점 뭉침 0 / 12,564종목 산출**
  - 관리자 `/admin/score`: 전역 가중치 + **종목별 오버라이드**(추가/수정/삭제) + 커버리지 하한·수축 지점 + 즉시 재계산(분포 검증 결과 표시). 배치가 분포 이상 시 경고
  - `stock_daily_closes.score`에 미러링(8,550행) → 기존 일별 시세 화면에 즉시 노출
  - **8월 7일 종가 누락 원인**: `-fetch-missing`이 "종가가 **하나도 없는**" 종목만 대상이라 08-06까지 있는 종목은 영구 제외됐음(삼성전자 포함 819종목). `-fetch-stale=<날짜>` 모드 신설. 커버리지 1,469 → **9,147종목**
  - AI 요약: 개요 탭 카드 재설계(제목/본문 4줄/키워드/전체 분석 보기), `keywords` 필드 BE 신설(LLM 프롬프트 + 규칙 기반 폴백), 개요 외 탭에서 좌측 레일 숨김
  - 검증: go build/vet/test(신규 유닛 15건 — 실측 꼬리값·수축 시나리오·오버라이드), tsc/eslint/next build, Docker 재기동(Tesseract OK), 관리자 엔드포인트 401 확인. **관리자 화면 육안 확인은 자격증명 부재로 미완**
- 08-08: **탭 잘림 버그 수정 + 전망 탭 제거 + AI 분석 탭 신설 + 좌측 레일 축소**
  - **탭 콘텐츠 잘림(치명)**: `scrollMain = compact || effectiveTab === "overview"` 이라 데스크톱에서 개요 외 모든 탭이 `overflow-hidden` → 넘치는 내용에 **스크롤 수단 자체가 없었음**. 실측 잘림량: 내부자 거래 **910px/1521px**, 뉴스 484px, 재무제표 165px, 밸류에이션 80px. `effectiveTab !== "chart"`(차트 터미널만 자체 스크롤 소유)로 변경해 전부 도달 가능
  - **전망 탭 삭제**: 워크스페이스/레거시 `/analysis` 양쪽 탭 목록·렌더 제거, 참조 0이 된 `outlook-tab.tsx` 파일 삭제
  - **AI 분석 탭 신설(개요 바로 우측) + 헤더 요약 배치**: 헤더 중앙 빈 공간에 한 줄 요약(`StockAISummaryLine`, truncate 1줄 고정이라 요약 길이가 헤더 높이를 흔들지 않음) + "전체 분석 →". 전체 분석은 신규 `AIAnalysisTab`(4개 섹션 + 일별 종가/점수 리스트)에 배치 — **기존 `일별 시세` 탭을 흡수·대체**. 헤더는 넓은 화면 중앙 슬롯 / 좁은 화면 자체 행 두 곳에 같은 노드를 렌더하므로 fetch는 상위(`useStockSummary`)에서 1회만 수행하고 프레젠테이션 컴포넌트로 분리(중복 요청 방지, StrictMode 이중 마운트와 동일 수준까지 실측 확인)
  - **탭별 `StockSummaryBanner` 전면 제거**: 요약이 헤더+AI 탭으로 일원화돼 중복이고, 탭마다 세로 공간을 먹어 위 잘림 문제를 키우고 있었음. 참조 0이 된 `stock-summary-banner.tsx` 삭제
  - **좌측 레일 축소**: 시세 + 52주 범위만 유지, 거래(거래량/거래대금/시가총액)·재무(PER/PBR/ROE)·PER/PBR 설명 블록 제거(기업 정보·밸류에이션 탭과 중복). 모바일 `StockMetricsGrid`도 동일 범위로 축소하고 1열 카드 + 내부 2열로 재배치. 미사용이 된 `exchangeRate` prop 및 import 정리
  - 검증: 1440/1024/375px 실측 — 탭 9개(전망 없음, AI 분석이 개요 우측), 레일 `시세`/`52주 범위`만, 헤더 AI 라인 데스크톱 중앙(x=966 w=366 h=32)·모바일 자체 행(w=315 h=31), 전 탭 `overflow-y:auto` + 가로 오버플로 0, tsc/eslint/next build 통과
- 08-08: **마켓 레이더 반응형 정비 + 일별 종가/AI점수 적재 구현** (v0.8.0 이후, 릴리스 미생성)
  - **마켓 레이더 치명적 레이아웃 버그**: `StockChartTerminal`이 1024–1279px(비compact·비xl)에서 `flex-col + overflow-hidden` 인데 호가 aside가 `shrink-0`(525px) → **차트+체결 컬럼이 0px로 압살**되어 노트북 폭에서 차트 탭이 통째로 안 보였음(실측 확인). `xl` 미만은 스크롤 가능한 단일 컬럼, `xl` 이상만 좌우 분할로 CSS만으로 분기하도록 재작성
  - **메트릭 레일 브레이크포인트 도입**(`railFrom: "lg"|"xl"`): 마켓 레이더는 랭킹 리스트가 이미 ~320px를 쓰는데 레일이 `lg`부터 떠서 1024px에서 탭 본문이 **208px**만 남았음 → 마켓 레이더만 `xl`로 미룸(실측 208→448px). 레일이 숨는 구간에는 `StockMetricsGrid`(신규)로 시세/거래/재무/52주범위를 카드 그리드로 제공 — 기존엔 모바일에서 이 수치들에 **접근 경로 자체가 없었음**
  - 그 외: 로딩 중 레일이 `hidden` 누락으로 모바일에 220px 죽은 컬럼 렌더되던 것 수정, `?symbol=` 딥링크가 모바일에서 리스트로 튕기던 것 수정(paramsKey 리셋 effect가 마운트 시에도 실행), 모바일 랭킹 행에 **랭킹 기준값(거래대금/거래량) 노출**(기존 desktop 전용) + 탭 타겟/폰트 확대, 탭바 활성 탭 자동 스크롤(reduced-motion 존중), TOP5 차트 모바일 개방, 리스트 하단 `pb-9` 죽은 여백 제거
  - **일별 종가 적재**(`docs/DAILY_CLOSE_DESIGN.md` §12): 마이그레이션 `029_daily_closes.sql`(`stock_daily_closes` + `index_daily_closes`), `score TINYINT NULL`(없으면 FE `-`) + `ai_summary TEXT NOT NULL`(없으면 `''`). 시장별 정규장 마감 +20분에 적재하는 `DailyCloseService.RunScheduler`(고정 tick 아님 — 다음 마감까지 대기)
  - **설계안 대비 변경**: 원안은 "주식 종가는 candle_bars 직독"이었으나 실측 결과 `candle_bars` 1d가 **날짜당 2행(00:00 행이 전일 종가 복제) + 주말 날짜 행**을 갖고 있어 그대로 읽으면 등락률이 `0%↔2배`로 튐(005930에서 -9.85%/0%/+3.64% 재현). `ROW_NUMBER`로 날짜당 마지막 봉만 + 주말 제외 + 중복 제거 후 `LAG`로 정규화한 **투영 테이블**로 전환(단방향·멱등, score/ai_summary는 UPDATE 제외). 수정 후 005930 `+2.05%/+0.69%/-0.34%` 정상. 기존 주말 행 378건 `PruneWeekendRows`로 삭제. **candle_bars에 중복 행을 쓰는 상위 경로는 미특정 — 차트 등 다른 소비자는 여전히 영향받을 수 있음(후속 과제)**
  - **점수는 서버 계산**(`daily_close_score.go`): RSI30%/추세25%/MACD20%/뉴스감성15%/52주위치10%, 결측 구성요소는 비중 제외 후 재정규화(0점 처리 금지), RSI는 70 부근 정점 후 하락. 유닛테스트 5건. LLM은 문장만. 실측: 000070 `score=75`
  - **적재 커버리지 미완**: 활성 종목 17,901개 중 일봉 보유 종목만 적재 가능 → 세션 종료 시점 **1,469종목/6,131행**(2026-08-03~08-07). 일봉 없는 종목은 Toss 종목당 1회 조회 필요한데 한도가 초당 5회라 `-fetch-missing` 반복 실행 필요(재개 가능, 429는 Retry-After 준수 재시도). 백그라운드 실행 중이었음
  - 검증: go build/vet/test(신규 유닛테스트 포함), Docker 재기동, `/daily-closes` API 실측(005930·AAPL·000070), 브라우저 375px/1024px/1440px 실측(오버플로 0, 차트 컬럼 높이 복구, 레일 전환, 일별 시세 탭 렌더 + 요약 펼침), tsc/eslint/next build
- 08-07: **종목 뉴스 탭 페이지네이션** (v0.8.0 이후, 릴리스 미생성)
  - `StockNewsPanel`(default variant, 워크스페이스/`/analysis` "뉴스" 탭)이 `limit={5}`로 호출되며 그 5건이 그대로 최종 표시 개수였음 — 더 볼 방법이 없어 스샷처럼 "잘린" 인상. 백엔드 offset 페이지네이션은 만들지 않음: `NewsStore.List`가 SQL LIMIT 이후에도 관련성/중복 필터를 추가로 거는 구조라 서버 페이지네이션을 넣어도 페이지당 정확한 건수를 보장 못함 — 대신 **fetch 상한을 60건으로 올리고 클라이언트에서 8건씩 페이지네이션**(prev/next + `1 … 4 5 6 … 12`식 번호, `pageWindow` 유닛 로직은 node로 별도 검증). 심볼 전환 시 페이지 1로 리셋. 워크스페이스/analysis 두 호출부의 `limit={5}` 제거
  - 관리자 로그인 페이지들과 달리 `/stocks/[symbol]`도 일반 로그인 세션이 필요해 이 환경에서 브라우저 실사용 확인은 못 함(테스트 계정 자격증명 없음) — tsc/eslint/next build 통과 + pageWindow 로직 node 스크립트 검증까지만 완료, 사용자 로그인 후 확인 필요
- 08-07: **LLM 재생성 00:00 KST 고정 + 관리자 수동 즉시 생성 + 사이드바 버그 진짜 원인 수정 + 종가 적재 설계안** (v0.8.0 이후, 릴리스 미생성)
  - **사이드바 버그 실제 원인 발견**: 이전 세션이 `item.href === "/admin"` 특수 케이스를 제거했지만, 남은 일반 규칙 `pathname.startsWith(`${item.href}/`)` 이 `/admin`에 대해 `/admin/`으로 평가돼 **모든 admin 하위 페이지에서 여전히 뉴스 관리가 활성**이었음(이전 세션의 "브라우저 캐시 문제" 결론은 오진). 각 항목 독립 판정을 버리고 **전체 nav href 중 최장 일치 1개만 활성**(`activeNavHref`)으로 변경. 실행 중인 dev 서버가 서빙하는 번들에서 함수와 `allNavHrefs`를 직접 추출·평가해 `/admin/llm → /admin/llm`, `/admin → /admin` 검증
  - **백그라운드 재생성 00:00:00 KST 고정**: `refresh_interval_hours` 노브 제거(모델/repo/handler/FE에서 제거, DB 컬럼은 NOT NULL DEFAULT라 마이그레이션 없이 미사용 방치). `nextScheduledRefresh`(KST 자정, 유닛테스트 5케이스) 기반 루프로 교체. **기동 시 즉시 스윕도 제거** — 재기동이 곧 추가 과금이 되지 않도록. 파일럿 외 종목은 종전대로 Get() 온디맨드
  - **수동 즉시 생성**(신규): `POST/GET /api/v1/admin/llm/refresh`. 파일럿 게이트를 **의도적으로 우회**(임의 종목 지정이 목적, apibudget 한도는 그대로 적용), 종목당 6초 간격 순차 실행, 종목별 pending/running/done/error + source(llm|rule) 추적. 동시 1건 제한(409). `StockSummaryService.Start(ctx)` 신설로 bg context 보관(수동 잡도 종료 시 취소됨). 요청 로그에 `manual` 플래그 추가
  - **종가 적재 설계안**(`docs/DAILY_CLOSE_DESIGN.md`, 미구현): 종목 종가 = `candle_bars` 1d **그대로**(종가 스냅샷 테이블 신설은 candle_bars와 완전 중복이라 비권장), 지수 종가 = 신규 `index_daily_closes`(`market_indices`는 PK=index_id 최신 1행이라 이력 불가), LLM 일자별 = 신규 `stock_daily_analyses`. 점수는 **서버 계산**(뉴스 감성 집계와 동일 원칙) + LLM은 문장만. 00:00 KST 기준 `trade_date`가 시장별로 다른 함정(미국은 그 시각이 장중) 명시. 부가 제안: 점수 적중률(score(D) vs return(D+1) 정렬 필수), 지수 대비 알파, 이벤트 오버레이
  - 검증: go build/vet/test(신규 유닛테스트 포함), Docker 재기동(Tesseract OK), 신규 admin 라우트 401 확인, tsc/eslint/next build, dev 서버 번들에서 사이드바 로직 실측. **관리자 로그인 필요 화면(`/admin/llm`) 육안 확인은 이전 세션들과 동일 사유로 미완료** — 수동 즉시 생성 UI와 실제 LLM 호출은 사용자 확인 필요
- 08-07: **Gemini 실측 한도 반영(RPM 15/RPD 500/TPM 250K) + 요청 기록 종목명 표시** (v0.8.0 이후, 릴리스 미생성)
  - 사용자가 Gemini 3.1 Flash-Lite 공식 무료 티어 한도를 직접 제공(RPM 15, TPM 250K, RPD 500) — 기존 추정치(DailyQuota 800/PerMinute 10, 서드파티 집계 사이트 기반)를 정확한 값으로 교체(`DailyQuota:500, PerMinute:15, TokensPerMinute:250000`)
  - **TPM(분당 토큰) 실측 추적 신규 구현**: `apibudget.Manager`에 분당 토큰 카운터 추가(`RecordTokens`, Redis IncrBy + 2분 TTL, 메모리 폴백), `Check()`가 이번 분에 이미 누적된 토큰이 한도 이상이면 추가 호출을 차단(사전 추정이 아니라 사후 누적 기준 — 단일 요청이 한도를 처음 넘기는 건 못 막지만 같은 분 내 반복 호출로 인한 초과는 막음). `llm/client.go`가 OpenAI 호환 응답의 `usage.total_tokens`를 실제로 파싱해 기록(실측: 삼성전자+SK하이닉스 2회 호출로 14,771 토큰 기록 확인)
  - `/admin/llm`·`/admin/api` 양쪽에 분당 토큰(TPM) 사용량 바 추가
  - **요청 INPUT 기록을 종목명 기준으로 확인 가능하도록 수정**: `LLMRequestLogEntry`에 `Name` 필드 추가(로깅 시점에 `stock.Name` 그대로 사용, 별도 조회 불필요), `/admin/llm`에 종목명 드롭다운 필터 추가 — 로그 항목도 종목코드 대신 종목명을 우선 표시(코드는 보조로 작게 병기)
  - **사이드바 버그 재확인**: 사용자가 "여전히 뉴스 관리 탭이 항상 활성으로 보인다"고 재보고 — 실행 중인 dev 서버가 실제로 서빙하는 컴파일된 JS 번들을 직접 열어 검사해 수정된 코드(주석 포함)가 정상 반영되어 있고 기존 버그 패턴은 존재하지 않음을 확인. 코드는 정상이며 브라우저 캐시 문제로 판단, 사용자에게 강력 새로고침 요청
  - 검증: go build/vet/test, Docker 재기동 후 005930·000660 재검증(TPM 14,771 기록, 종목명 정상 표시, 한도값 500/15/250000 정상 노출), tsc/eslint/next build
  - 버전/릴리스 문서화는 v0.9.0에서 누적분과 함께 완료(`docs/releases/v0/v0.9/v0.9.0.md`)
- 08-07: **v0.8.0 — 사이드바 활성 탭 버그 수정 + LLM 모델 관리 상세화(호출 주기·API 한도·요청 INPUT 기록)** (`docs/releases/v0/v0.8/v0.8.0.md`)
  - **사이드바 버그**: `item.href === "/admin"` 특수 케이스가 `pathname.startsWith("/admin/")`이면 무조건 활성 처리 — `/admin`이 다른 모든 관리자 페이지의 접두사라 항상 "뉴스 관리"도 같이 활성으로 보이던 원인. 특수 케이스 제거, 다른 메뉴와 동일한 정확일치/자식경로 규칙으로 통일(`/analysis`,`/market`용 중복 특수케이스도 함께 정리)
  - **호출 주기 관리자 편집**: `llm_settings`에 `refresh_interval_hours` 컬럼 추가(1~24h 클램프, `columnExists` 기반 idempotent 마이그레이션). `time.Ticker`(런타임 변경 불가)를 버리고 매 주기 최신 설정을 재조회하는 sleep 루프로 전환
  - **API 한도 인라인 표시**: `/admin/llm`에서 openai provider의 분당/일간/월간 호출량 그래프를 바로 확인 가능(기존 `/admin/api` 이동 불필요)
  - **요청 INPUT 기록**(신규): `GenerateStockSummary` 호출마다 factsJSON+성공/실패를 Redis 캡드 리스트(`llm:requests:v1:log`, 최근 30건·7일 TTL)에 기록, `GET /admin/llm-requests`(읽기전용) + `/admin/llm`에서 펼쳐보기. 실측: 000660 호출의 실제 factsJSON(재무·내부자·뉴스 데이터 전체) Redis LRANGE로 직접 확인
  - **버그 발견+수정**: v0.7.0 스케줄러가 파일럿 종목을 간격 없이 연속 갱신해 2번째 종목부터 apibudget MinInterval(4s)에 걸려 조용히 규칙 기반으로만 캐시되던 문제를 재검증 중 다시 확인(005930 llm / 000660 rule) — 6초 간격 추가로 재수정, 재검증 후 둘 다 llm 확인
  - 검증: go build/vet/test, Docker 재기동, Redis 요청 로그 실측, 관리자 엔드포인트 401 정상 차단, tsc/eslint/next build, `/admin/llm` 미인증 리다이렉트 확인. 관리자 로그인 화면 육안 확인은 이전 세션들과 동일 사유로 미완료
- 08-07: **관리자 메뉴 개편 — LLM 모델 관리 신설 + 뉴스/데이터베이스 관리 명칭 정리** (v0.8.0에 포함되어 출시됨)
  - 사용자 피드백: API 관리 탭엔 (기존처럼) 호출량·한도만 있으면 되고, 파일럿 종목 관리는 별도 "LLM 모델 관리" 메뉴로 분리 요청 + 종목을 관리자가 직접 편집 가능해야 함(기존엔 env 전용)
  - **파일럿 종목을 DB 설정으로 전환**: 신규 `llm_settings` 테이블(단일 행, `market_data_settings`와 동일 패턴) + `LLMSettingsRepo`/`LLMSettingsStore`(요청마다 즉시 재조회 — 재기동 없이 관리자 편집 즉시 반영). 기존 `STOCK_SUMMARY_LLM_SYMBOLS` env 플러밍은 완전히 제거(단일 소스 유지)
  - 신규 `/admin/llm`(LLM 모델 관리): provider 상태 + 파일럿 종목 편집 폼(쉼표 구분 입력, 최대 10개·자동 정규화) + 종목별 캐시 상태. `/admin/api`에서는 파일럿 섹션 제거(원래 목적대로 provider별 호출량/한도 그리드만 유지 — LLM도 다른 provider와 동일하게 그 그리드에 카드로 표시됨)
  - 사이드바 라벨 정리: "관리자"→"뉴스 관리"(실제로 뉴스 키워드/통계/폴링 설정 페이지였음), "데이터베이스"→"데이터베이스 관리"
  - **버그 발견+수정**: 스케줄러가 파일럿 종목을 간격 없이 연속 갱신해 2번째 종목부터 apibudget MinInterval(4s)에 매번 걸려 조용히 규칙 기반으로만 캐시되던 것을 실측(005930은 llm, 000660은 rule로 확인) 후 발견 — 심볼 간 6초 간격 추가로 수정, 재검증 후 둘 다 llm 확인
  - 검증: go build/vet/test, Docker 재기동 후 005930·000660 모두 `source:"llm"` 확인(간격 버그 수정 전/후 대조), `/admin/llm-settings`가 401로 정상 차단, `/admin/llm` 미인증 접근 시 정상 리다이렉트 확인, tsc/eslint/next build. 관리자 로그인 화면 육안 확인은 이전 세션들과 동일 사유로 미완료
  - 버전/릴리스 문서화는 v0.8.0에서 완료(`docs/releases/v0/v0.8/v0.8.0.md`)
- 08-07: **v0.7.0 — Gemini 연동 + 2종목(삼성전자·SK하이닉스) 파일럿 제한 + 관리자 모니터링** (`docs/releases/v0/v0.7/v0.7.0.md`)
  - `config.buildLLMConfig()`: `OPENAI_API_KEY` 우선, 없으면 `GEMINI_API_KEY`로 폴백해 Google OpenAI 호환 엔드포인트+`gemini-3.1-flash-lite` 기본값 사용(코드 변경 없이 provider 전환 가능, `OPENAI_BASE_URL`/`OPENAI_MODEL`로 오버라이드 가능). 실측: 005930·000660 모두 실제 Gemini 응답으로 `source:"llm"` 확인
  - **비용 통제**: `STOCK_SUMMARY_LLM_SYMBOLS` env(기본 `005930,000660`)로 LLM 호출 자체를 파일럿 심볼로 제한(규칙 기반 요약은 모든 종목에서 여전히 제공). 스케줄러도 파일럿만 예열(기존 `AllTrackedSymbols` 전체 예열에서 축소). apibudget 한도를 Gemini 무료 티어 공개치보다 보수적으로 조정(DailyQuota 800/PerMinute 10/MinInterval 4s). 실측: AAPL 등 비파일럿 종목은 `source:"rule"` 고정, LLM 호출 자체가 안 일어나는 것 확인
  - **관리자 모니터링**: 신규 `GET /admin/stock-summary`(읽기 전용, 생성 트리거 안 함) — provider + 파일럿 종목별 캐시 상태. `/admin/api`에 "AI 요약 파일럿" 섹션 추가
  - 검증: go build/vet/test, 005930·000660·AAPL 실측(LLM/규칙 분기 정상), apibudget 사용량 증가 확인, tsc/eslint/next build, 브라우저 데스크톱+모바일 확인(관리자 화면 로그인 확인은 이전 세션과 동일 사유로 미완료)
- 08-07: **뉴스 탭 + AI 3줄 요약(밸류에이션/기술적분석/내부자/뉴스) 신규 구현** (v0.7.0에 포함되어 출시됨)
  - **뉴스 탭**: 백엔드 변경 없이 기존 파이프라인 재사용 — `fetchStockNews`(이미 심볼 필터 지원) + 그동안 미사용이던 `StockNewsPanel` 컴포넌트를 워크스페이스/`/analysis` 양쪽 "뉴스" 탭에 연결. `stock-workspace-tabs.ts`/`analysis-tabs.ts`에 `news` 탭 id 추가
  - **AI 3줄 요약**: 예시 문구(밸류에이션/기술적분석/뉴스, 감성 점수 포함)와 동일한 기능이 이미 있는 줄 알았으나 조사 결과 전무 — 종목별 LLM 파이프라인, 뉴스 감성 분석, RSI/MACD/ADX 계산 모두 신규 구현. 사용자 확인: 지표는 실제 계산(허구 금지), LLM은 비용 통제 설계 필요, 노출 위치는 탭별 상단 → Plan Mode로 설계 승인 후 구현
    - **기술적 지표**(`technical_indicators.go`, 신규): SMA20/50/200·RSI(14, Wilder)·MACD(12,26,9)·ADX(14, Wilder)·52주 고저·최근 스윙 고저 — `candle_bars` 최근 200일봉에서 순수 함수로 계산, 데이터 부족 시 해당 필드만 nil(수치 조작 없음). 유닛 테스트 3건(부족한 히스토리/단조 상승/빈 입력) 통과
    - **비용 통제**: 탭 4개(밸류에이션·기술적분석·내부자·뉴스)를 **LLM 호출 1회**로 한 번에 생성(`llm.GenerateStockSummary`, 신규). `insights.go`와 동일한 stale-while-revalidate 패턴을 심볼 기준으로 이식(`StockSummaryService`, Redis `stock:summary:v1:<symbol>`, TTL 24h/stale 6h). 백그라운드 예열은 `MarketDataStore.AllTrackedSymbols`(관심+보유 종목, 기존 헬퍼 재사용)만 6시간 주기로 순회 — 무제한 전체 예열이 아님. 뉴스 감성은 LLM이 기사별 라벨만 반환하고 긍정/부정/중립 **집계는 서버에서 직접 계산**(실제 기사 수와 항상 일치, LLM이 숫자를 지어낼 수 없는 구조)
    - `llm/client.go` 리팩터: `GenerateBriefing`/`TranslateENToKO`가 중복 구현하던 HTTP 스캐폴딩을 `chatCompletion` 공용 헬퍼로 추출(동작 변경 없음) 후 `GenerateStockSummary` 추가. `apibudget`에 `ProviderOpenAI`/`CategoryLLM` 신설, `llm.Client`에 budget Check/Record/429 BlockFor 연결(기존엔 LLM 호출이 budget 추적 대상이 아니었음 — 부수 개선)
    - **버그 발견+수정**: `apibudget.Manager.AllUsage()`의 provider 하드코딩 목록에 신규 `ProviderOpenAI`를 빠뜨려 `/admin/api`·`/health/api-quotas`에 노출되지 않던 것을 실측(`curl .../health/api-quotas`에서 openai 항목 없음) 후 발견·수정
    - **LLM 미설정(OPENAI_API_KEY 없음) 환경에서 항상 폴백**: 규칙 기반 문장(계산된 실수치만 나열, 예: "PER 35.2배, PBR 3.1배, 영업이익률 13.1%...")으로 정상 동작 확인 — 이 개발 환경엔 키가 없어 LLM 경로 자체는 라이브 검증 못 함(코드 리뷰로만 확인), 실제 LLM 응답 파싱은 사용자 환경에서 키 설정 후 재확인 권장
  - FE `StockSummaryBanner`(`section` prop으로 4종 분기) 신규, 재무제표/실적/전망/내부자거래(→valuation·insider)·차트(→technical)·뉴스(→news) 6개 탭 상단에 배선(워크스페이스+`/analysis` 양쪽)
  - 검증: go build/vet/test(신규 유닛테스트 포함) 전체 통과, Docker 재기동 후 005930·AAPL `/summary` 실측(실제 계산된 PER/PBR/RSI/ADX/내부자 집계/뉴스 건수 확인, 가짜 수치 없음), Redis 캐시 키·TTL(24h) 확인, 스케줄러가 관심종목 자동 예열하는 것 실측 확인(재기동 직후 여러 심볼 캐시 생성됨), FE tsc/eslint/next build, 브라우저로 6개 탭 전부 데스크톱+모바일 렌더링 확인
  - 버전/릴리스 문서화는 v0.7.0에서 완료(`docs/releases/v0/v0.7/v0.7.0.md`)
- 08-07: **재무제표 상세 모달 "1년" 트레일링 윈도우 버그 수정** (v0.7.0에 포함되어 출시됨)
  - `DartClient.FetchQuarterlyHistory`(`dart_financials.go`)가 `연도` 단위로 DART를 조회해(`currentYear-offset`), "1년" 선택 시 올해 들어 이미 제출된 분기만 반환 — 연초에는 1~2분기만 표시되던 버그. `years`만큼의 여유 연도를 추가로 더 가져온 뒤(`years+1`개 연도), 최근 `years*4`개 분기만 남기는 방식으로 변경해 진짜 트레일링 12개월이 되도록 수정. 미국(EDGAR) 쪽은 애초에 `FetchQuarterlySnapshots`가 "가장 최근 N개 신고기간"을 반환하는 구조라 이 버그가 없었음(대조군)
  - 검증: go build/vet, Redis 캐시(`financials:history:kr:v1:*`) 삭제 후 005930 `years=1` 재조회로 25년 6월~26년 3월(트레일링 4분기) 확인, `years=3`도 12분기 정상 확인, 브라우저로 재무제표 탭→수익성 카드→모달→"1년" 클릭해 동일 결과 렌더링 확인
- 08-07: **v0.6.0 — 내부자 거래 탭 구현 + 전망 탭·재무제표 상세 모달·DART/EDGAR 최신성 비교·마켓 레이더 탭 유지 버그 묶어 출시** (`docs/releases/v0/v0.6/v0.6.0.md`)
  - 신규 `GET /stocks/{symbol}/insider`: 국내는 DART `elestock.json`(임원ㆍ주요주주 소유보고, 날짜 필터 없음 — 전체 이력 중 최근 30건) 직접 매핑. 미국은 `submissions.json`으로 최근 Form 4/4-A 신고 목록을 얻은 뒤 각 신고의 원본 소유보고 XML을 개별 fetch해 보고자ㆍ관계(이사/임원/10%주주)ㆍ거래코드(P/S/G/M/F 등 SEC 고정 19종)ㆍ수량ㆍ단가ㆍ거래후보유를 파싱(최근 3건 신고로 제한). Redis 24h 캐시
  - **버그 발견+수정**: EDGAR `apibudget`은 분당 8회를 앱 전체가 공유하는데, US 심볼 1건 조회가 최대 7회(submissions 1 + Form4 XML 최대 5 + 티커캐시미스시 +1)를 소비 — 여러 US 심볼을 연속 조회하면 예산 소진으로 개별 fetch가 전부 실패하는데, 기존 방식은 이를 "신고 없음"으로 오인해 빈 배열을 24시간 캐시해버림(MSFT로 실측 재현). "시도한 신고가 있는데 전부 실패"면 에러를 반환해 캐시하지 않도록 수정 + 신고 상한 5→3으로 하향. AAPL/MSFT/TSLA/NVDA/AMZN 실측으로 수정 전후 재현·검증
  - FE `InsiderTab`(국내/미국 통합 테이블, 구분별 색상, 지분율(KR)/단가(US) 동적 컬럼) 신규, 워크스페이스+레거시 `/analysis` 양쪽의 "내부자 거래" 플레이스홀더 교체(`AnalysisPlaceholderTab` import 완전 제거 — 8개 탭 전부 실구현 완료)
  - v0.6.0 릴리스 문서에 이번 세션의 내부자 거래 탭과 함께, 이전 세션에서 미출시 상태로 남아있던 전망 탭ㆍ재무제표 상세 모달ㆍDART/EDGAR 최신성 비교ㆍ마켓 레이더 탭 유지 버그 수정을 모두 묶어 문서화(`docs/releases/README.md` 인덱스 갱신, `fe/package.json` 0.5.0→0.6.0)
  - `.claude/launch.json`에 FE dev 서버 `autoPort: true` 추가 — 동시 세션 포트 충돌 시 자동 대체 포트(브라우저 검증 중 다른 세션의 3000번 서버와 충돌 확인 후 조치, 없어도 기존 단일 세션 워크플로엔 영향 없음)
  - 검증: go build/vet, 005930·AAPL·MSFT·TSLA·NVDA·AMZN 실측(EDGAR 예산 소진 재현+수정 재검증 포함), tsc/eslint/next build, 데스크톱(1280×720)·모바일(375×812) 브라우저 렌더링 확인(다른 세션의 실행 중인 dev 서버에 직접 접속해 검증 — 이 세션 자체 dev 서버는 포트 충돌로 기동 불가했음)
- 08-07: **전망 탭 구현 + DART/EDGAR 최신성 비교 + 마켓 레이더 탭 유지 버그 수정** (v0.6.0에 포함되어 출시됨)
  - `전망` 탭: 실적추세(financials/history 재사용)·밸류에이션 맥락(PER/PBR + PSR은 FE에서 `price*shares/revenue`로 신규 계산, "FY{연도} 실적 기준·DART/EDGAR" 라벨)·공시일정(`earnings` 엔드포인트의 `calendar` 재사용, `CalendarSection`을 `earnings-tab.tsx`에서 export해 공유)·컨센서스 안내블록. BE 변경 없이 전부 기존 엔드포인트 재사용. 스펙 4번(DART 잠정실적 공시 "본문" 파싱)은 자유 형식 문서 파싱이라 범위 밖으로 명시, 후속 과제
  - DART/EDGAR 최신성 비교(`FundamentalsService.RefreshSymbol`): SEC `company_tickers.json` 실측으로 실제 20-F 파일러 7종목만 매핑(`krxToEdgarTicker`: SK텔레콤·KB금융·신한지주·우리금융지주·POSCO홀딩스·한국전력·LG디스플레이) — 삼성전자·SK하이닉스는 무보증 ADR이라 매핑 없음(정상, 사용자 예시가 애초에 해당 없는 케이스였음). `PeriodEnd` 비교로 EDGAR가 "진짜 더 최신"일 때만 교체, 동일/구버전이면 DART 유지. 부수 수정: `FetchAnnualFinancialSnapshots`가 지금까지 `PeriodEnd`를 아예 채우지 않던 걸 발견해 `calendarQuarterEnd(year,4)`로 채움(이전 세션에 이미 만든 헬퍼 재사용) — 이 필드가 없으면 비교 자체가 불가능했음. `cmd/seed-fundamentals -symbols=017670` 로컬 실행으로 실측(DART가 정상적으로 계속 승리 — 20-F는 연간뿐이라 DART 분기 주기가 구조적으로 항상 더 최신, 예상대로)
  - 마켓 레이더 탭 리셋 버그: `market/page.tsx`의 `selectSymbol`/URL-sync 두 군데에 있던 강제 `setWorkspaceTab("overview")` 제거 — 각 탭이 이미 `symbol` prop 변경 시 자체 재조회하므로 탭 유지만으로 안전. 헤더 검색으로 SK하이닉스 선택 브라우저 테스트로 재무제표 탭 유지 확인(마켓 레이더 랭킹 리스트는 이 세션 브라우저에서 로딩이 안 걸려 리스트-클릭 대신 헤더 검색으로 동일 콜백 경로 검증)
  - 검증: go build/vet, tsc/eslint/next build, 017670·005930 fundamentals 실측 비교, 전망 탭 브라우저 렌더 확인(005930), 탭 유지 브라우저 확인(005930→000660)
- 08-07: **재무제표 탭 — 클릭 시 상세 모달(토스 스타일) + 실데이터 다분기 히스토리** (v0.6.0에 포함되어 출시됨)
  - 신규 `GET /api/v1/stocks/{symbol}/financials/history?years=N`(기본 3, 최대 5): 국내(DART) 최근 N년 × 4개 reprt_code 순회, 손익 항목(매출/영업이익/순이익/EPS)은 1~3분기 `thstrm_amount`(이미 단독분기, 라이브 확인) 직접 사용 + 4분기는 `연간 thstrm_amount − 3분기 thstrm_add_amount(9개월 누적)`으로 역산; 재무상태표 항목은 각 reprt_code의 시점값 그대로. 미국(EDGAR)은 기존 `FetchQuarterlySnapshots`에 재무상태표 concept 추출 추가 + limit만 늘려 재사용(추가 API 호출 없음, companyfacts 응답 1건에 이미 포함)
  - **버그 발견+수정 3건** (모두 라이브 API로 직접 검증):
    1. DART 반기(11012) 보고서의 당기순이익 계정명이 "반기순이익"으로 달라 순이익만 누락 — 계정명 매칭을 정확매치에서 "(손실)" 접미사 허용 매치로 일반화(`dartAccountNameMatchesAny`), `net_income` 폴백에 "반기순이익" 추가
    2. SEC XBRL이 동일 물리적 분기를 필터마다 다른 `fy` 태그로 중복 보고(원 신고 1회 + 다음 해 비교컬럼 1회) — AAPL 실측에서 2024-09-28이 "FY2024 Q4"·"FY2025 Q4" 두 번 중복 확인. `recentReportPeriods`/`pickFactForPeriod`의 매칭 키를 (FY,FP,End)에서 (FP,End)로 변경, FY는 표시전용으로 분리
    3. (이전 세션 실적 탭에서 발견) EDGAR 표준분기 길이 검증은 이미 있었음 — 이번엔 재무상태표(BS) 항목에 동일 검증을 잘못 적용하지 않도록 `pickInstantFactForPeriod`(End만 매칭, duration 무관) 별도 추가해 회피
  - FE `FinancialDetailModal`(3-preset: 수익성/성장성/안정성, ComposedChart bar+line + 표 + 1· 3 · 5년 토글, 헤드라인 문장 "직전 분기 대비 ±%/%p") 신규, `FinancialsChartsTab`의 가짜 배수 데이터(`0.72+i*0.07`)를 실제 히스토리로 교체하고 3개 카드에 클릭 핸들러 연결
  - 검증: BE go build/vet, 005930·AAPL 실측(005930 9분기 전 필드 정상, AAPL 13분기 중복 없음 확인), FE tsc/eslint/next build, 브라우저로 두 프리셋(수익성/안정성) 클릭→모달→연도토글→닫기 전체 플로우 데스크톱 확인. **모바일 뷰포트(375px)에서 카드 자체는 정상 렌더 확인, 단 모달 오픈 클릭이 이 환경의 `computer` 도구에서 타임아웃되어(브라우저 pane 미표시) 모바일 모달 자체는 직접 클릭 검증 못함** — CSS는 기존에 이미 동작 중인 `chart-fullscreen-modal.tsx`와 동일한 반응형 패턴 재사용, 사용자 확인 권장
  - 버전/릴리스 문서화는 v0.6.0에서 완료 (`docs/releases/v0/v0.6/v0.6.0.md`)
- 08-07: **v0.5.0 — 실적 탭 구현** (`docs/releases/v0/v0.5/v0.5.0.md`)
  - 신규 `GET /api/v1/stocks/{symbol}/earnings`: 국내(DART 잠정실적/사업·분기·반기보고서 키워드 필터 공시 + reprt_code 4종 분기 상세) / 미국(Finnhub 발표일정 + EDGAR 최근 5개 신고기간) 분기 처리, Redis 24h 캐시, DB 스키마 변경 없음(라이브 조회)
  - **버그 발견+수정 1**: DART curl TLS 폴백이 `curl.exe`+`GOOS=="windows"` 게이트라 Docker(Linux) 컨테이너에서 항상 스킵 — 국내 실적 신규 코드가 전부 빈 응답이던 원인. `dart_http.go` OS 무관하게 동작하도록 수정(기존 DART 실시간 경로 전반에 영향, 잠재 버그였음)
  - **버그 발견+수정 2**: EDGAR 분기 추출이 3개월 단일분기와 6/9개월 누적(YTD) XBRL 사실을 구분 못해 애플 Q2/Q3 매출이 \$254B/\$364B로 부풀려짐 — `xbrl_select.go`에 60~100일 표준분기 길이 검증(`isStandaloneQuarterDuration`) 추가, 수정 후 \$109~144B 정상 범위로 확인
  - FE `EarningsTab`(`components/analysis/earnings-tab.tsx`) 신규, `stock-workspace-client.tsx`/`analysis-client.tsx` 양쪽 실적 탭 플레이스홀더 교체. 실적 요약은 기존 `/fundamentals` 데이터 재사용(중복 호출 없음)
  - 검증: BE go build/vet 통과, 005930·AAPL·MSFT 실측 확인, FE tsc/eslint/next build 통과, 데스크톱·모바일 브라우저 렌더링 확인(워크스페이스 + 레거시 `/analysis` 양쪽)
- 08-06: **마켓 레이더 "1분 주기" 표시 수정 + 관리자 `/admin/api` 신설**
  - 원인: 실제 폴링은 `market_data_settings.rankingsPollRealtimeMinutes`(=5)를 정상 사용 중이었으나, 툴바 라벨(`rankingRefreshHint`, `market-radar-config.ts`)이 `"1분 주기"`를 하드코딩 — `useMarketDataSettings()` 실측값을 인자로 전달하도록 수정, 훅의 DEFAULTS 폴백(1→5)도 정정
  - `apibudget` 패키지에 provider별 용도/카테고리 카탈로그(`catalog.go`) 추가; `Limits`에 커스텀 `MarshalJSON`(MinInterval을 ms로) 추가 — 기존엔 JSON 태그가 없어 FE가 못 읽는 상태였음
  - `AdminHandler.APIQuotas`가 카탈로그(용도·카테고리) + 실시간 사용량(분/일/월 카운트, 마지막 호출, 차단 여부)을 병합 응답. 라우트(`/api/v1/admin/api-quotas`)는 기존 것 재사용
  - FE `/admin/api` 신규 페이지: 카테고리(시장데이터/공시·재무/뉴스/번역)별 provider 카드 대시보드 + 사용량 progress bar. 사이드바 "관리" 그룹에 "API 관리" 추가. 관리자 홈의 인라인 "API 할당량" 표는 제거하고 링크로 대체
  - 검증: `go build`/`tsc`/`eslint`/`next build` 통과, Docker 재기동 후 `/health/api-quotas`(동일 타입 사용)로 JSON 직렬화 실측 확인. **관리자 세션 화면 육안 확인은 미완료** — 테스트 계정에 admin 권한 부여(DB UPDATE)가 auto-classifier에 차단되어 실제 로그인 후 `/admin/api` 렌더링은 사용자 확인 필요
- 08-06: **랭킹 5분 정렬 폴링 + UI 회귀 조치 + 설계안 2건** (v0.4.0 이후, 릴리스 미생성)
  - 마켓 레이더 랭킹 `realtime` duration: 1분(상대시간) → **5분, 벽시계 정렬**(00:00/00:05/…). 최초 1회는 서비스 시작 시각 그대로, 이후는 `alignedRankingBucket`(`t.UTC().Truncate(interval)`)로 버킷 경계에서만 갱신 — `market_rankings_poller.go`. DB 기존 행 기본값도 마이그레이션으로 1→5 일괄 조정(`MigrateRankingsRealtimeDefaultIfNeeded`, 관리자가 이미 바꿔둔 값은 보존)
  - 기업정보 탭 매출·산업 구성 파이차트 애니메이션 제거(`isAnimationActive={false}`) — `company-info-tab.tsx`
  - UI 회귀(모바일 뷰가 풀 화면 기준으로 보임) 조사: `data-shell` 메커니즘 자체는 정상(코드 리뷰+모바일/데스크톱 뷰포트 실측 모두 정상). 오래 켜져있던 dev 서버가 08-05와 동일 패턴(dev 캐시)일 가능성 높아 `.next` 캐시 삭제 후 재기동 — **육안 재현은 못 함(스크린샷 도구 제약), 사용자 확인 필요**
  - 설계안 2건 제시(미구현, 결정 대기): 마켓 레이더 랭킹 실시간 가격 오버레이(기존 배치가격 파이프라인에 편승), 실적/전망/내부자거래 탭 무료 API 매핑(DART `elestock.json`으로 내부자거래는 국내·미국 완전 커버 가능, 전망 탭은 목표가/컨센서스가 전부 유료라 플레이스홀더 문구 재정의 필요)
- 08-06: **v0.4.0 — 헤더/차트 시세 정합 + Toss 429 구조 개선** (`docs/releases/v0/v0.4/v0.4.0.md`)
  - 헤더·차트 가격 불일치: `refreshTerminalSnapshot`이 계산한 가격을 `refreshFormingBars`에 공유(같은 틱 내 동일값 보장); `splitFormingCandle`/`lastCompletedTime` 배열 순서 의존 제거(Toss 일봉은 내림차순 응답); FE `use-quote-candles.ts`에 2초 REST 폴백 추가(dev 프록시가 SSE 스트림을 버퍼링해 응답이 전혀 안 오던 것 확인)
  - Toss 429 폭주 원인: `/api/v1/candles`가 94% 비중 — `refreshRawCandles`가 30초마다 심볼당 1분봉 7,200개(36페이지) 전량 재조회하던 것을 DB 마지막 봉 이후 델타만 요청하도록 변경(`candles_intraday_delta.go`); `fetchIntradayStats`(시세 통계)도 동일 패턴으로 DB-first 전환
  - `/api/v1/trades`,`/api/v1/orderbook`: 관심·보유종목(pinned)이 미조회 상태여도 5초마다 갱신되던 것 제거 — 실제 조회 중(active)인 심볼만 전체 갱신, 가격은 배치 호출로 계속 신선(사용자 수 확장 시 관심종목 수가 아니라 동시 조회 수에만 비례)
  - `X-RateLimit-*` 헤더 로깅(`toss/rate_limit.go`) + `Retry-After` 실값 사용(고정 2초/60초 제거)
  - 실측: trades 75→0, orderbook 59→0 (60초 표본); candles 94%→소수 비중
  - **BE 전용 — CSS/스타일 파일 미변경** (2026-08-05 dev 서버 CSS 캐시 레이아웃 회귀와 무관)
- 08-05: **DB 최적화 + 관리자 DB 콘솔**
  - `candle_bars` 재구축: 행 단위 UPDATE 루프(사실상 미완료) → `INSERT IGNORE ... SELECT` 단일 패스 (2.53M행 27초)
  - `bar_time` VARCHAR(32) → DATETIME, PK와 완전 중복이던 보조 인덱스 제거 → **516.9MB → 195.8MB (62%↓)**
  - 이전 테이블은 `candle_bars_legacy_v1` 로 보존 → 확인 후 `migrations/026_storage_cleanup.sql` 참고해 수동 DROP
  - `ranking_snapshot_entries` 90일 보관 (`PruneOlderThan`, 12시간 주기, 5000행 배치)
  - **`/admin/database`** — 테이블 용도 카탈로그 + 행 CRUD. 접근 가능 테이블은
    `service/db_admin_catalog.go` 화이트리스트에만 한정, 식별자는 항상 live schema와 대조 후 사용,
    비밀번호 해시·암호화 자격증명 컬럼은 마스킹, 머신 관리 테이블은 읽기 전용
- 08-05: **성능 리팩토링 (BE+FE)** — 상세: `docs/archive/sessions.md`
  - Poller: fast tick 겹침 방지(CAS) + 동시성 4 제한 + active(1s)/pinned(5s) 분리 + 심볼 상한 40
  - `refreshTerminalSnapshot` Toss 콜 5→3 (no-op KRW 정규화용 `fxForSymbol` GetStock 제거)
  - `candle_bars` UpsertBatch 행별 Exec → 500행 multi-row upsert
  - `HasTable` / `barTimeIsDatetime` information_schema 결과 캐싱 (positive latch + 30s 음성 재확인)
  - watchlist/holdings 조회를 fan-out 스코프로 메모이즈 (`withOverlayCache`) — 배치당 N→1
  - MySQL pool 10 → 50/25 (`MYSQL_MAX_OPEN_CONNS`/`MYSQL_MAX_IDLE_CONNS`)
  - Graceful shutdown: http.Shutdown → bgCancel → pool close 순서 + SIGTERM 핸들링
  - `QuotesService.StartPolling(ctx)` 분리 — 생성자에서 poller 기동 시 Set* 와이어링과 경합하던 문제
- 08-04: **quotes:detail Redis 제거** — detail read = `quotes:terminal`(1s) → MySQL; startup purge `quotes:detail:*`; ensureQuoteDetail → refreshTerminalSnapshot
- 08-13: **뉴스 RSS·지수 태그 확장** — 국내 RSS(한경 finance·MK 증권·연합 경제·토큰포스트·코인리더스·블록미디어, 더팩트 제거), 크립토 관련성·Finnhub crypto, 국내 헤드라인 중복 skip, ingest `index:{id}` 태그 + 폴링 키워드(`040_news_index_keywords.sql`)
- 08-04: **Redis cache unify** — single envelope key per entity; startup purge of `:last` duplicates; freshness guard on read
- 08-03: **Admin 캐시 관리** — `/admin/cache` UI + `/api/v1/admin/cache/*` (Redis SCAN/GET/SET/DELETE; oauth 차단, apibudget 읽기전용)
- 08-03: **Continuous Toss polling** — `shouldLivePollSymbol` = Toss enabled (세션 무관); 장 마감 종가 캐시 reuse 폐기
- 08-03: **Single price resolver** — `ResolveLiveQuotePrice` (rankings·quotes·batch·terminal 공통)
- 08-03: 마켓 레이더 — 시세 시각(updatedAt) 실시간 갱신, 랭킹→배치시세 덮어쓰기(프리장) 수정, PER/PBR/PSR 한글 설명, `market_data_settings` 관리자 폴링 설정
- 07-30: FE `StockChartTerminal` — chart/trades/orderbook extracted from quotes-detail-panel → `fe/src/components/stock-workspace/stock-chart-terminal.tsx`
- 07-26: Market indices batch refresh (FRED 3분, warm-up, sp500/dow/vix fix, API `groups`/`dataKind`) → `docs/MARKET_INDICES_BATCH.md`
- 07-26: FE 계좌 연동 메뉴 비노출 (`ACCOUNT_FEATURES_VISIBLE=false`, `docs/FE_ACCOUNT_FEATURES.md`); BE/storage 유지
- 07-25: Naver/Kakao news apibudget → 일일 cap만 (MinInterval/PerMinute 제거, tick당 키워드 일괄 검색)
- 07-25: Naver 뉴스 → NAVER API HUB (`naverapihub.apigw.ntruss.com`, NCP 인증 헤더)
- 07-25: 뉴스 `stripHTML` → `html.UnescapeString` (&#34;/&#39; 등 숫자 엔티티 디코딩)
- 07-25: MySQL terminal persist (011) + write-through/read chain; AGENT_STATE durable policy
