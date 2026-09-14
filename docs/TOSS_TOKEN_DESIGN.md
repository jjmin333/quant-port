# Toss REST + WebSocket 자격증명 공존 설계

> 실측일 2026-08-25 02:28~04:13 KST (미국 정규장 중). 프로브 `be/cmd/probe-toss-coexist`.
> 측정 중 `quant-port-api` · `quant-port-worker-fetch`를 정지시켜 토큰 간섭을 제거했다.

## 요약

**분리의 근거였던 전제 ①이 틀렸다.** 웹소켓을 닫아도 토큰은 죽지 않는다.
토큰을 죽인 것은 종료가 아니라 **재연결이 부른 재발급**(`IssueFresh`)이었다.

WS가 스스로 발급하지 않고 공유 토큰을 그대로 쓰면, 두 자격증명 모두에서
REST와 WS가 같은 토큰으로 공존한다 — 실측으로 REST 실패 0건.
얻는 것은 **WS 슬롯 200 → 400**과 **REST failover 복구**다.

## 1. 실측 결과

기존 코드(`be/internal/toss/stream.go` 헤더)가 근거로 삼은 두 명제:

	① 웹소켓을 닫으면 그 액세스 토큰이 영구 무효화된다.
	② client_id당 활성 토큰이 하나다 — 새로 발급하면 이전 것이 즉시 죽는다.

| # | 질문 | 결과 |
|---|------|------|
| A | client_id당 활성 토큰이 하나인가 | **예** — 새 발급 후 이전 토큰 REST가 401. `expires_in=86399s` |
| B | 열린 WS가 자기 토큰의 무효화를 견디는가 | **예** — 새 발급 후에도 12초간 88프레임, PING 성공 |
| C | 죽은 토큰의 WS를 닫아도 현재 토큰이 사는가 | **예** — REST 200 |
| D | WS를 열어 둔 채 같은 토큰으로 REST | **12/12 200** (같은 구간 WS 프레임 204) |
| E | **살아 있는 토큰의 WS를 정상 종료한 뒤 REST** | **200 — ① 반증** |
| F | 강제 TCP 절단(CloseNow) 후 REST | 200 (+3초), 200 (+13초) |
| J | 종료 후 +2s / +20s / +60s REST | 전부 200 — **지연 무효화 없음** |
| I | 종료 후 **같은 토큰**으로 재연결 | **3/3 성공** (PRIMARY·SECONDARY 각각) |
| K | 강제 절단 후 같은 토큰으로 재연결 | 성공, 재연결 후 프레임 126 |
| L | 2연결 중 1개만 종료 | 나머지 연결·토큰 모두 무영향 |
| M | 앱당 동시 연결 상한 | **2** — 3·4번째를 열면 오래된 2개가 끊긴다 |
| N | 연결당 구독 슬롯 상한 | **100** — `too-many-topics: too many topics requested (max 100)` |
| G | 두 앱 × (REST + WS 2연결) 45초 동시 | REST **30/30 200**, 프레임 1,552, 연결 **4/4 생존** |
| H | make-before-break 회전 중 REST | **57/57 200** — 무중단 |
| D | WS 트래픽이 MARKET_DATA 쿼터를 먹는가 | **아니오** — 프레임 204에도 remaining 13/15 → 14/15 |

②는 그대로 사실이고, **①만 거짓**이다.

## 2. 진짜 인과

기존 관측 「WS 종료 후 그 토큰의 REST가 401」은 혼동(confound)이었다.
종료가 아니라, **종료 다음에 일어난 재연결이 새 토큰을 발급했고**(②) 그것이
이전 토큰을 죽인 것이다.

현재 코드에서 그 발급이 일어나는 곳:

- `be/internal/service/quotes_stream.go:327` `streamToken()` — 프로세스 메모리의
  `s.token`이 비어 있으면 **무조건** `IssueFresh`를 부른다.
- `be/internal/toss/auth.go:340` `IssueFresh()` — 캐시를 무시하고 새로 발급한다.

즉 **프로세스가 뜰 때마다 반드시 한 번은 그 자격증명의 토큰을 갈아 치운다.**
REST와 앱을 공유했다면 그때마다 REST가 통째로 401을 맞았을 것이고, 그것이
분리 결정의 실제 원인이다. 종료와는 무관했다.

## 3. 설계

핵심 규칙은 하나다.

> **토큰 발급은 자격증명당 단 하나의 주체(공유 `TokenManager`)만 한다.
> WS는 절대 스스로 발급하지 않는다.**

### 3.1 변경점

| # | 파일 | 변경 |
|---|------|------|
| 1 | `be/internal/service/quotes_stream.go` | `streamToken`/`tokenAuth` 제거. REST와 **같은** `*toss.TokenManager`를 받아 `AccessToken(ctx)`로 캐시된 토큰을 쓴다 |
| 2 | `be/internal/toss/auth.go` | `IssueFresh` **삭제**. 남겨 두면 언젠가 다시 불린다 |
| 3 | `be/internal/toss/stream.go` | 401 핸드셰이크에서만 `Invalidate` → `AccessToken`. 유일한 발급 경로 |
| 4 | `be/internal/config/config.go` | `RESTConfig()` 제거 → REST가 SECONDARY failover를 되찾는다 |
| 5 | `be/internal/config/config.go` | `StreamReady()`에서 `SecondaryEnabled()` 요구 제거 → 단일 앱에서도 WS 가능 |
| 6 | `be/internal/service/quotes_stream.go` | **두 자격증명 모두**로 연결을 연다: 앱당 2연결 × 100슬롯 = **400슬롯** |

### 3.2 반드시 함께 넣어야 하는 안전장치 — CAS invalidate

`be/internal/toss/client.go`의 401 처리는 지금 `auth.Invalidate(ctx)` 후 재시도다.
`Invalidate`는 Redis 키를 **무조건 지운다**. 같은 자격증명을 쓰는 프로세스가
여럿(`api`, `worker-fetch`)이므로 다음 왕복이 가능하다:

```
A 발급 T2 → B가 들고 있던 T1로 401 → B가 Invalidate(T2 삭제) → B 발급 T3
→ A가 T2로 401 → A가 Invalidate(T3 삭제) → A 발급 T4 → …
```

지금은 WS가 앱을 나눠 써서 드러나지 않았을 뿐, 합치면 노출된다.

→ `Invalidate`를 **compare-and-swap**으로 바꾼다. 401을 맞은 토큰 문자열을 넘겨,
저장소의 현재 값이 **그것과 같을 때만** 지운다. 다르면 형제가 이미 새 토큰을
넣어 둔 것이므로 그것을 읽어 재시도한다. 발급이 자격증명당 1회로 수렴한다.

### 3.3 토큰 회전 (24시간 만료)

`refreshLeadTime`(5분) 전에 배경 갱신이 새 토큰을 발급한다. 그 순간:

- 열려 있는 WS 연결은 **살아남는다** (B — 핸드셰이크 시점에만 인증)
- REST는 즉시 새 토큰으로 간다 (Redis 공유)
- 다음 재연결이 새 토큰을 쓴다

H에서 이 순서를 그대로 재현해 REST 57/57 200을 확인했다.
따로 「make-before-break」 코드를 넣을 필요는 없다 — 재연결에 재발급이 없으므로
회전은 하루 한 번, 배경 갱신 하나로 끝난다.

## 4. 예산 (변경 전 → 후)

| 항목 | 지금 | 제안 |
|------|------|------|
| WS 슬롯 | 200 (SECONDARY 2연결) | **400** (PRIMARY 2 + SECONDARY 2) |
| trade+orderbook 동시 종목 | 100 | **200** |
| trade만 | 200 | **400** |
| REST `MARKET_DATA` | 15 req/s (PRIMARY만) | **30** (failover 복구) |
| REST 기타 그룹 | 그룹당 1배 | 2배 |
| 재연결이 REST에 주는 영향 | — | **없음** (재발급 없음) |

### failover 복구의 실측 근거

측정 직후 재기동에서 (미국장 중, 캐시 비어 있음):

```
rate-limit exceeded 31건 / 5분   ← /api/v1/stocks 20, /trades 6, /orderbook 3, /prices 1, /market-calendar 1
failover            0건          ← SECONDARY가 WS 전용이라 REST가 못 쓴다
```

기동 버스트는 failover가 정확히 흡수해야 할 모양인데 지금은 쓸 수 없다.

## 5. 남은 위험

1. **동시 연결 상한 2 (M)** — 배포 중 옛/새 컨테이너가 겹치면 앱당 2연결을 서로
   뺏는다. 앱이 2개가 되면 4연결이 걸린다. 이미 있는 재연결 백오프+jitter가
   완충하고, `streamConnections`는 앱당 2로 고정 유지한다.
2. **자격증명 1개가 죽으면 REST·WS가 함께 영향** — 분리 구성의 유일한 실질
   장점이었다. 다만 그런 사건은 관측된 적이 없고 대응(키 교체)은 어차피 수동이다.
3. **미측정: 24시간 연속 연결 유지** — 프로브는 최대 수 분이다. 만료 경계는 배경
   갱신이 덮지만, 실제 무재연결 24h 유지는 운영 중 관측이 필요하다.
4. **`marketFastSymbolCap = 40`** (`be/internal/service/market_poller.go:49`) —
   슬롯을 400으로 늘려도 이 상한이 그대로면 실사용은 안 늘어난다. 3단계에서 별도 검토.

## 6. 적용 순서

단계마다 되돌릴 수 있게 나눈다.

1. **자격증명 공유를 안전하게 만든다** (필수·저위험)
   WS가 `IssueFresh`를 안 쓰게 + `IssueFresh` 삭제 + CAS invalidate.
   구성은 그대로 두고 **회귀만 확인**한다.
2. **REST failover 복구** — `RESTConfig()` 제거.
   관측: `rate-limit exceeded` / `failover` 로그 카운트.
3. **WS를 두 앱으로 확장** — 400슬롯. `marketFastSymbolCap` 상향은 여기서 함께 검토.

## 재현

```bash
cd be && go run ./cmd/probe-toss-coexist          # A~H
cd be && go run ./cmd/probe-toss-coexist -round2  # I~N
```

토큰을 여러 번 재발급하므로 **`quant-port-api`·`quant-port-worker-fetch`를 멈춘 뒤**
돌린다. 같은 자격증명을 쓰는 프로세스가 살아 있으면 서로 토큰을 죽여 판정이 오염된다.

---

# 2부: 구현과 실측 결과 (2026-08-25)

1부의 설계를 구현하고, 거기에 **차트·호가 화면의 요청 경로**까지 구독으로 옮겼다.

## 7. 구현 전에 발견한 것 — 구독이 그 화면을 전혀 대체하지 않고 있었다

`/metrics`에 `quantport_toss_rest_path_total{path}`를 넣어 재 보니, **구독이 붙어
있는 종목인데도** 화면이 부르는 REST가 그대로였다.

실측 (2026-08-25 미국 정규장, 시청자 1명, 30초, 터미널 스냅샷 15건):

| 엔드포인트 | 유휴 | 시청 중 | 증가 | rate-limit 그룹 |
|---|---|---|---|---|
| `/api/v1/orderbook` | 0 | 19 | **+19** | MARKET_DATA (15/s) |
| `/api/v1/trades` | 0 | 19 | **+19** | MARKET_DATA |
| `/api/v1/stocks` | 0 | 19 | **+19** | STOCK (**5/s**) |
| `/api/v1/prices` | 60 | 70 | +10 | MARKET_DATA |

같은 30초에 웹소켓은 그 종목의 프레임을 1,257건 받고 있었다.

원인은 두 갈래였다.

1. **구독은 폴러(`tickFast`)만 대체했다.** 화면이 2초마다 부르는
   `/quotes/{symbol}/terminal-snapshot`은 캐시 TTL이 1초라 매번 만료돼
   `refreshTerminalSnapshot`으로 들어갔고, 그 함수는 구독을 보지 않고 REST를 냈다.
2. **구독이 `/api/v1/stocks` 낭비를 오히려 키웠다.** 폴러의 `prefetchStockInfo`가
   배치로 받아 두는데, 구독 중인 종목은 `filterStreamedSymbols`가 폴링에서 빼므로
   배치 대상이 아니었다 — 그래서 요청 경로가 종목마다 개별 `GetStock`을 불렀다.

`STOCK` 버킷이 5 req/s이므로 이 비율이면 **동시 시청자 8명에서 소진**된다.
기동 버스트에서 `/api/v1/stocks` rate-limit이 20건 찍힌 것이 같은 원인이었다.

## 8. 무엇을 바꿨나

| 변경 | 파일 |
|---|---|
| WS가 재발급 대신 **공유 토큰**을 쓴다 (`IssueFresh` 삭제 → `TokenAfter`) | `toss/auth.go`, `toss/stream.go` |
| 401 처리를 **CAS**로 (`InvalidateIfCurrent`) — 다중 프로세스 재발급 왕복 차단 | `toss/auth.go`, `toss/client.go` |
| REST가 SECONDARY failover 회복 · 구독은 두 앱 사용(슬롯 200→400) | `config/config.go`, `server/server.go` |
| 체결 프레임을 **링버퍼(50건)** 로 쌓아 화면에 공급 | `service/quotes_stream_terminal.go` |
| 호가 프레임을 메모리에 유지, **Redis 0.5초 · MySQL 3초** 스로틀로 반영 | 〃 |
| 스냅샷 조립이 구독을 먼저 본다 (`planTerminalSources`) | `service/quotes_marketdata.go` |
| `/api/v1/stocks`를 Redis 30초 캐시로 (`stockInfo` 3단 계층) | `service/quotes_stock_batch.go` |
| 「조용한 종목」과 「끊긴 구독」 분리 (`IsFed` — 구독 확인 + 5분 유예) | `service/quotes_stream_terminal.go` |
| Toss REST 호출 계측 (`{group}` · `{path}`) | `metrics/` |

### 왜 `IsLive`와 `IsFed`를 갈랐나

질문이 다르다.

	IsLive — 「폴링을 꺼도 되나」(tickFast). 틀리면 화면이 멈추므로 보수적으로.
	IsFed  — 「REST를 안 불러도 되나」. 조용한 것은 바뀐 게 없다는 뜻이다.

`IsFed`는 프레임 도착(20초)에 더해 **서버가 확인한 구독**(`subscriptions` 프레임)이
서 있으면 5분까지 버퍼를 믿는다. 5분은 이 코드베이스가 같은 데이터에 이미 쓰던
기준(`liveMicrostructureMaxAge`)을 그대로 가져왔다. 연결이 끊기거나 구독이
거부되면 확인이 **즉시** 취소되므로 진짜 장애를 덮지 않는다.

## 9. 실측 결과

### REST 호출 (시청자 1명, 터미널 스냅샷 기준)

| 엔드포인트 | 변경 전 (15건/30초) | 변경 후 (20건/40초) |
|---|---|---|
| `/api/v1/orderbook` | 19 | **0** |
| `/api/v1/trades` | 19 | **0** |
| `/api/v1/stocks` | 19 | **4** |
| 합계 | **57** | **4** |

같은 구간에서 구독이 응답한 비율 **20/20**. 15건 기준으로 환산하면 57 → 3,
**약 95% 감소**다.

`/api/v1/prices`(+88)·`/rankings`·`/market-calendar`는 배경 폴러의 몫으로,
시청 여부와 무관하며 이번 변경 대상이 아니다.

### 데이터 정확성 — 구독 vs REST 기준값

`go run ./cmd/probe-toss-coexist -compare AAPL` (운영 공유 토큰 사용, 발급 0회):

	source=toss-ws
	최우선 매도: 구독 310.6600 vs REST 310.6600 (괴리 0.0000%) — 완전 일치
	호가 정렬 asks 오름차순=true · bids 내림차순=true · 스프레드 정상
	체결 13건 · 최신순=true · side="unknown" (REST `mapTrades`와 동일)

체결 순서도 REST와 일치했다:

| REST | 구독 |
|---|---|
| 310.6107 | 310.61 |
| 310.66 | 310.66 |
| 310.6538 | 310.65 |
| 310.6361 | 310.63 |

**웹소켓은 미국 종목 가격을 소수 2자리로 준다** (REST는 4자리). 체결 순서와
식별은 같고, 화면은 USD를 이미 2자리로 렌더하므로(`PriceDisplay` 기본
`decimals=2`) **표시 결과가 동일**하다. KRX는 정수 호가라 무관하다.

호가 단 수도 두 경로가 같았다 — AAPL은 양쪽 다 1단(미국 종목의 상류 특성이지
구독의 한계가 아니다). 국내는 REST 기준 10단을 확인했다.

### DB 쓰기

호가 프레임이 종목당 초당 17건 관측된다. 전에는 그때마다 `UpsertOrderBook`이
나갔다. 3초 스로틀로 **종목당 초당 17회 → 0.33회**가 된다(이 DB는 1 ECPU).

### 자격증명 공존

	toss open api global client enabled (primary + secondary failover)
	toss stream: websocket quotes enabled (2 credentials, 4 connections, 400 slots)

기동 이후 `invalid-token`/401 **0건**, 토큰 발급 **0회**(캐시된 공유 토큰 재사용).

## 10. QA

| 항목 | 결과 |
|---|---|
| 전체 Go 테스트 | 통과 |
| `-race` (빌더 이미지, `-count=2`) | `internal/service`·`internal/toss` 통과 |
| 동시성 스트레스(수신 4 + 플러시 + 조회 + 정리) | 통과 |
| 롤백 (`TOSS_STREAM_ENABLED=false`) | `stream_subscriptions=0`, `source=toss`, 시세 정상 |
| 구독 끊김 → 폴링 복귀 | 단위 테스트 (`IsFed`, `OnDisconnect`, `planTerminalSources`) |
| 구독 거부 → 폴링 유지 | 단위 테스트 |
| 데이터 정확성 | 위 비교 프로브 |

새 테스트 22건 (`quotes_stream_terminal_test.go`, `auth_cas_test.go`,
`groups_test.go`, `config_test.go` 갱신).

## 11. 아직 검증 못 한 것

- **KRX 개장 중 실측.** 위 수치는 전부 미국 세션에서 잰 것이다. 국내 종목의
  호가 10단이 구독으로도 그대로 오는지, 개장 직후 버스트에서 슬롯 400이
  충분한지는 09:00 이후 재측정이 필요하다.
- **24시간 연속 연결.** 토큰 만료(24h) 경계에서 배경 갱신 → 재연결이 무중단인지는
  프로브(H)에서 절차만 확인했고, 실제 만료 경계를 지나 본 적은 없다.
- **동시 시청자 다수.** 슬롯 400·연결 4개는 상한을 실측했을 뿐, 그만큼 채운 적은 없다.

---

# 3부: KRX 개장 중 실측 · 토큰 만료 방어 (2026-08-25 09:44~10:05 KST)

## 12. KRX 개장 중 실측 — 2부에서 남겨 둔 항목

2부의 수치는 전부 미국 세션에서 잰 것이었다. 국내 정규장(`marketSession=regular`)에서
같은 측정을 다시 했다.

### REST 호출 (005930, 터미널 스냅샷 20건 / 40초)

| 엔드포인트 | 폴링 모드 (`TOSS_STREAM_ENABLED=false`) | 구독 모드 |
|---|---|---|
| `/api/v1/orderbook` | **33** | **0** |
| `/api/v1/trades` | **33** | **0** |
| `/api/v1/stocks` | **24** | **1** |
| 합계 | **90** | **1** |

구독이 응답한 비율 **20/20**.

미국 세션(57 → 4)보다 격차가 큰 이유: 국내 정규장에서는 `filterLiveSessionSymbols`가
그 종목을 살려 두므로 **폴러의 `tickFast`까지** 같은 호출을 얹는다. 즉 폴링 모드의
90은 요청 경로와 폴러가 겹친 실제 부하이고, 구독 모드에서는 양쪽이 함께 사라진다.

### 데이터 정확성 (장중)

	source=toss-ws  호가 asks=10 bids=10  체결=50건
	최우선 매도: 구독 249500.0000 vs REST 249500.0000 (괴리 0.0000%)
	정렬 asks 오름차순=true · bids 내림차순=true · 스프레드 정상
	체결 50건 · 최신순=true · side="unknown"

**국내 호가 10단이 구독으로도 그대로 온다** — 2부에서 미검증으로 남겨 둔 항목이다.
REST 기준값도 10단이라 두 경로의 깊이가 같다.

체결 최신가는 구독 249,250 / REST 249,500으로 갈렸는데, 이는 두 요청 사이에 실제로
체결이 더 일어난 것이다(호가는 같은 순간의 스냅샷이라 완전 일치했다).

## 13. 토큰이 죽는 순간을 견디게 만들었다

### 무엇이 문제였나

24시간 만료 경계에서 REST가 통째로 멈출 수 있는 경로가 넷 있었다.

1. **갱신 실패 = 전면 실패.** 갱신은 만료 5분 전에 시작하는데, 그 사이 발급이
   실패하면(상류 5xx·네트워크·락 경합) 에러를 그대로 올렸다. **아직 5분 남은 멀쩡한
   토큰을 들고도** 모든 호출이 죽는다.
2. **발급에 재시도가 없었다.** 순간적인 5xx 하나가 그대로 장애가 된다.
3. **401 재시도가 1회뿐.** 자격증명을 REST·웹소켓·워커가 공유하므로 회전 순간
   **연속으로** 밀릴 수 있고, 그러면 사용자 요청이 실패한다.
4. **일시적 전송 실패에 재처리가 없었다.** 연결 리셋·5xx 하나에 시세 한 칸이 빈다.

그리고 측정 중 하나 더 나왔다:

5. **경합이 없어도 발급 전에 2초를 기다렸다.** `ensureFresh`가 락을 잡기 **전에**
   `waitForPeerRefresh`(50ms × 40회)를 돌렸다. 아무도 갱신 중이 아니면 그 2초를
   그냥 태운 뒤 발급했다 — 기동 직후와 24시간 회전, 즉 평상시 경로에 매번 붙던
   지연이다. 무리를 막는 것은 Redis 락이므로 **못 잡았을 때만** 기다리면 된다.

### 무엇을 넣었나

| 방어 | 내용 |
|---|---|
| 만료 전 토큰으로 물러서기 | 갱신 실패 시 **아직 만료 전인** 캐시 토큰을 쓴다. 만료된 것은 거른다(어차피 401) |
| 발급 재시도 | 3회 · 200ms/600ms 백오프. **`invalid_client` 같은 4xx는 재시도하지 않는다** — 같은 답이고 쿼터만 태운다 |
| 401 재시도 3회 | 연속 두 번의 회전을 견딘다. 매 시도가 토큰을 다시 읽는다 |
| 일시적 전송 재처리 | 전송 실패·5xx에서 **조회만** 3회 재시도(150ms/450ms). POST/DELETE는 두 번 적용될 수 있어 제외. 429는 제외 — 그건 예산 소진이라 failover가 맡는다 |
| 락 대기 순서 교정 | 락을 먼저 잡고, 못 잡았을 때만 기다린다 → 평상시 발급 지연 **2.0초 → 0.03초** |
| 만료 시각 의미 정리 | 저장값을 **실제 만료 시각**으로. 전에는 리드타임을 미리 빼 둬 「갱신할 때인가」와 「아직 쓸 수 있나」가 구분되지 않았다 |
| 계측 | `quantport_toss_token_events_total{kind}` — issued / issue_retry / issue_failed / fallback_unexpired / retry_401 |

### 실측 — 토큰을 실제로 죽여 봤다

24시간을 기다릴 수는 없으므로 같은 사건을 즉시 재현했다. 밖에서 토큰을 발급하면
client_id당 활성 토큰이 하나이므로 API가 쓰던 토큰이 그 자리에서 죽는다 —
만료와 구별되지 않는 상태다.

	go run ./cmd/probe-toss-coexist -killtoken 005930

결과 (국내 정규장 중, 0.2초 간격으로 화면 요청을 계속 두드리며 8초 간격 3회 사망):

	토큰 사망 #1 (10:03:58) #2 (10:04:06) #3 (10:04:14)
	스냅샷 요청: 성공 173 · 실패 0
	retry_401  14 → 17  (사망 1건당 정확히 1회)
	issued     15 → 18  (사망 1건당 정확히 1회 — 재발급 왕복 없음)
	웹소켓     3회 사망 후에도 source=toss-ws 유지

`retry_401`이 정확히 3 늘었다는 것이 **방어가 실제로 발동했다**는 증거다.
그러면서 사용자 요청은 173건 전부 200이었다.

> **주의 — 처음에 셸로 같은 실험을 했다가 오독할 뻔했다.** `cut -d= -f2`가 base64
> 시크릿의 `=` 패딩에서 잘려 `invalid_client`로 발급이 실패했고, 아무것도 안 죽었으니
> 당연히 안 끊겼다. 그래서 프로브가 **새 토큰이 실제로 200을 받는지** 확인한 뒤에만
> 「죽였다」고 기록한다.

### failover 복구도 실동작 확인

기동 이후 `rate-limit exceeded` 24건에 대해 **failover 16건**이 실제로 발생했다.
2부 이전에는 SECONDARY가 웹소켓 전용이라 이 수치가 **0**이었다 — 초과분을 흡수할
수단이 아예 없었다는 뜻이다.

## 14. QA (3부 추가분)

| 항목 | 결과 |
|---|---|
| 전체 Go 테스트 | 통과 |
| `-race` (빌더 이미지) | `internal/service`·`internal/toss` 통과 |
| 토큰 강제 사망 3회 · 실트래픽 | 173/173 성공, retry_401 +3, issued +3 |
| 웹소켓 생존 | 사망 3회 후에도 구독 유지 |
| 국내 호가 10단 | 구독·REST 양쪽 동일 |
| 롤백 | `TOSS_STREAM_ENABLED=false` 재확인(장중) |

신규 테스트: 만료 전 토큰 폴백 · 만료 토큰 거부 · 영구 실패 무재시도 · 일시적 실패
복구 · 락 대기 회귀 · 5xx 재시도와 failover 구분 · 연속 401 생존 · 재시도 포기 ·
배치 일시 실패 복구.

## 15. 남은 것

- **실제 24시간 만료 경계.** 강제 사망은 만료와 같은 상태를 만들지만, 배경 갱신이
  **먼저** 도는 정상 경로(만료 5분 전 선제 갱신)는 아직 실제로 지나 본 적이 없다.
  `quantport_toss_token_events_total{kind="issued"}`가 자격증명당 하루 1이면 정상이고,
  `fallback_unexpired`가 0이 아니면 갱신 경로가 흔들린다는 뜻이다.
- **다수 동시 시청자.** 슬롯 400·연결 4개는 상한만 실측했고 채워 본 적은 없다.
- **`marketFastSymbolCap = 40`** — 슬롯을 400으로 늘려도 이 상한이 그대로면 실사용은
  안 늘어난다.
