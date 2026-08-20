# 종가 시세 적재 · 일자별 LLM 분석 제공 설계

> 작성일: 2026-08-07 · **구현 완료: 2026-08-08** (§12 참조)
> 관련 코드: `be/internal/service/daily_close.go`, `daily_close_fetch.go`, `daily_close_score.go`,
> `be/internal/repository/daily_closes.go`, `be/migrations/029_daily_closes.sql`,
> `fe/src/components/analysis/daily-close-tab.tsx`

> ⚠️ **구현은 아래 설계안과 한 군데 다릅니다.** 원안은 "주식 종가는 `candle_bars`를 그대로 읽는다"였으나,
> 실제로는 **`stock_daily_closes` 투영 테이블을 신설**했습니다. 이유와 절충은 §12.1에 정리했습니다.

---

## 1. 요구사항

첨부된 화면(일자별 리스트)은 한 종목에 대해 다음을 보여준다.

| 컬럼 | 예시 | 필요한 데이터 |
|------|------|---------------|
| 날짜 | `'26년 8월 6일 (목)` | 거래일 |
| 실제주가 | `-0.1%` / `+3.43%` (방향 아이콘) | 그날 **종가** + **전 거래일 종가** |
| 점수 | `+64` | 그 날짜의 **LLM/분석 점수** |

즉 필요한 것은 두 가지다.
1. **일자별 종가 시계열** — 지수와 종목 모두
2. **일자별 분석 결과(점수 + 근거)** 의 영속화 — 현재는 Redis 24h 캐시라 **하루 지나면 사라진다**

---

## 2. 결론 (권고안 요약)

**"기존 확장 vs 종가 스냅샷 테이블 신설" 중 하나를 고르는 문제가 아니라, 대상별로 답이 다르다.**

| 대상 | 권고 | 이유 |
|------|------|------|
| **종목 종가** | **기존 `candle_bars`(`bar_interval='1d'`) 그대로 사용. 신규 테이블 만들지 않음** | 이미 같은 데이터가 완성 일봉으로 들어와 있다. 별도 종가 테이블은 같은 숫자의 두 번째 source of truth를 만드는 것 |
| **지수 종가** | **신규 `index_daily_closes` 테이블** | `market_indices`는 PK가 `index_id`인 **최신 1행** 스냅샷이라 구조적으로 이력이 남지 않는다. sparkline(JSON 20포인트)도 이력 저장소가 아니다 |
| **일자별 LLM 분석/점수** | **신규 `stock_daily_analyses` 테이블** | 현재 어디에도 영속되지 않는다. AGENT_STATE 원칙 1(사용자에게 보이는 상태는 Redis만 두지 않는다)에도 어긋난 상태 |

### 왜 "종가 스냅샷 테이블"을 새로 만들면 안 되는가

`quote_snapshots` / `orderbook_snapshots` 는 **PK = symbol, 최신 1건 last-good** 패턴이다. 장중에 계속 덮어써지는 휘발성 상태를 유실 없이 보관하는 게 목적이라 이력이 남지 않는 게 정상이다.

반면 종가는 **확정되면 다시 바뀌지 않는 저카디널리티 시계열**이다. 성격이 정반대다.

- 같은 스냅샷 패턴(PK=symbol)으로 만들면 → 최근 1일치만 남아 요구사항을 못 채운다
- 시계열 패턴(PK=symbol+date)으로 만들면 → `candle_bars`의 `1d` 행과 **완전히 중복**된다. `candle_bars`는 이미 195.8MB / 2.53M행 규모이고, 종가 사본을 하나 더 두면 용량뿐 아니라 "어느 쪽이 맞는 종가냐" 정합성 문제가 생긴다

지수만 예외인 이유는 단순하다. **지수는 `candle_bars`에 들어있지 않고, `market_indices`에는 이력 자리가 없다.** 즉 지수는 "기존 테이블 확장"이 애초에 불가능하다.

> 지수를 `candle_bars`에 `symbol='^KOSPI'` 형태로 밀어넣는 방안도 검토했으나 비권장한다. `candle_bars.symbol`은 `stocks` 마스터와 조인되는 네임스페이스이고, 지수는 `volume`이 무의미하며 하루 12행 × 250일 ≈ 3천 행/년이라 전용 테이블 비용이 사실상 0이다.

---

## 3. 현재 저장 구조 실사

| 테이블 | 키 | 이력 | 종가 관련 보유 데이터 |
|--------|-----|------|----------------------|
| `candle_bars` | (symbol, bar_interval, bar_time) | ✅ 시계열 | `1d` 행에 OHLCV 완비 — **종가 이미 있음** |
| `market_indices` | index_id | ❌ 최신 1행 | value/change/changeRate + sparkline 20포인트(JSON) |
| `symbol_last_prices` | symbol | ❌ 최신 1행 | 목록용 last price |
| `quote_snapshots` | symbol | ❌ 최신 1행 | QuoteDetail JSON |
| `ranking_snapshot_entries` | id (+symbol 인덱스) | ✅ 90일 | 랭킹 진입 종목의 순위·등락률만 (전 종목 아님) |
| LLM 요약 | — | ❌ | Redis `stock:summary:v1:<symbol>` TTL 24h **뿐** |

핵심: **종목 종가는 이미 있고, 지수 종가와 LLM 분석 이력은 없다.**

---

## 4. 권고 스키마

### 4.1 `index_daily_closes` (신규)

```sql
CREATE TABLE IF NOT EXISTS index_daily_closes (
  index_id      VARCHAR(32)    NOT NULL,
  trade_date    DATE           NOT NULL,
  close_value   DECIMAL(20,4)  NOT NULL,
  prev_close    DECIMAL(20,4)  NULL,      -- 조회 시 계산 대신 확정 시점 값 고정
  change_amount DECIMAL(20,4)  NULL,
  change_rate   DECIMAL(12,6)  NULL,
  currency      CHAR(3)        NOT NULL,
  source        VARCHAR(16)    NOT NULL,  -- toss | fred | finnhub | upbit | goldprice
  created_at    TIMESTAMP(3)   NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at    TIMESTAMP(3)   NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (index_id, trade_date)
) ENGINE=InnoDB;
```

- PK가 (index_id, trade_date) → **재실행해도 UPSERT로 멱등**
- 대상: `index_batches.go`의 12개 (`kospi`, `kosdaq`, `nasdaq`, `nasdaq100`, `sp500`, `dow`, `vix`, `nq100`, `sox`, `usd-krw`, `btc`, `gold`)
- 용량: 12 × 250거래일 ≈ **3천 행/년** (btc/gold는 365일이라 소폭 증가). 사실상 무시 가능

### 4.2 `stock_daily_analyses` (신규)

```sql
CREATE TABLE IF NOT EXISTS stock_daily_analyses (
  symbol         VARCHAR(32)  NOT NULL,
  trade_date     DATE         NOT NULL,   -- 분석의 기준이 된 "마지막 확정 종가일"
  score          TINYINT      NOT NULL,   -- 0~100 종합 점수 (서버 계산)
  score_breakdown JSON        NOT NULL,   -- {technical, valuation, momentum, insider, news} 각 0~100
  summary_json   JSON         NOT NULL,   -- StockSummaryView (valuation/technical/insider/news 문장 + newsSentiment)
  source         VARCHAR(8)   NOT NULL,   -- llm | rule
  model          VARCHAR(64)  NULL,       -- 예: gemini-3.1-flash-lite (재현성/모델 교체 추적)
  generated_at   TIMESTAMP(3) NOT NULL,
  PRIMARY KEY (symbol, trade_date),
  KEY idx_daily_analyses_date (trade_date)
) ENGINE=InnoDB;
```

- 파일럿 10종목 기준 **2.5천 행/년**. 전 종목으로 확대해도 종목수 × 250 수준
- `summary_json`을 통째로 보관해 나중에 화면 요구가 늘어도 재생성 불필요
- `model` 컬럼: 모델을 바꾸면 점수 체계가 미묘하게 달라진다. 어떤 모델이 만든 점수인지 남겨야 과거 데이터와 비교할 때 사고가 안 난다

### 4.3 `candle_bars` — 변경 없음

종목 종가는 스키마 변경 없이 그대로 쓴다. 등락률은 **조회 시 Go에서 계산**한다(최대 120행 응답 → 단순 루프, MySQL 윈도우 함수 의존 없음).

---

## 5. 적재 파이프라인

### 5.1 지수 — 기존 EOD 훅에 편승

`market_index_store_batch.go`에 이미 **마감 후 1회 실행** 로직(`eodAfter` + Redis 마커 `indices:eod:v1:{batch}:{YYYYMMDD}`)이 있다. 그 지점에서 `index_daily_closes`에 UPSERT 한 줄만 추가하면 된다. **신규 폴러도, 신규 API 쿼터 소비도 없다.**

- KR: `calendar.KRIntegratedAfterEnded` 이후
- US: `calendar.USAfterMarketEnded` 이후
- 24/7 자산(btc, gold): KST 자정 기준 일봉으로 확정

### 5.2 종목 — "일봉 확정 보장" 잡 신설 (실질 작업량은 여기)

`candle_bars`의 1d 행은 **폴러 tail + 사용자가 스크롤할 때의 백필**로 채워진다. 즉 **아무도 안 본 종목은 일봉이 비어 있을 수 있다.** 화면에 최근 N일을 항상 보여주려면 대상 종목에 대해 일봉 완결성을 보장하는 잡이 필요하다.

- 대상: `MarketDataStore.AllTrackedSymbols`(관심 + 보유) ∪ 파일럿 종목 — 전 종목이 아님
- 시점: 시장별 마감 후 (KR 통합장 종료 후 / US 애프터마켓 종료 후)
- 동작: 대상 종목별로 "마지막 확정 거래일 일봉이 있는지" 확인 → 없으면 Toss 1× 백필 (기존 백필 경로 재사용)
- 멱등: PK 충돌 UPSERT

### 5.3 LLM 일자별 분석 — 00:00 KST 스케줄러에 편승

이번 세션에서 백그라운드 재생성이 **매일 00:00:00 KST 고정**으로 바뀌었다(`StockSummaryService.RunScheduler`). 그 실행이 끝날 때 결과를 `stock_daily_analyses`에 UPSERT 하면 된다.

> ⚠️ **시장별 `trade_date` 주의.** 00:00 KST 시점의 "마지막 확정 종가일"은 시장마다 다르다.
> - 국내: 전일(D-1) 종가가 최신 확정치 → `trade_date = D-1`
> - 미국: 00:00 KST는 **미국 장중**(ET 오전)이다. 따라서 최신 확정 종가는 D-2 영업일일 수 있다 → `trade_date`를 달력으로 계산해야 하며, "실행일 − 1일"로 하드코딩하면 **미국 종목의 점수가 하루 밀린 종가에 붙는다.**
>
> `calendar.LastSessionCloseAt(schedule, now)` 를 그대로 써서 시장별로 계산할 것.

---

## 6. 점수(score) 설계 — 서버 계산 권고

**점수는 LLM이 만든 숫자가 아니라 서버가 계산한 숫자여야 한다.** 이는 이 프로젝트가 이미 뉴스 감성 집계에 적용한 규칙(LLM은 기사별 라벨만 반환, 긍/부/중 집계는 서버가 계산 → LLM이 숫자를 지어낼 수 없는 구조)의 연장이다.

LLM이 직접 0~100을 뱉게 하면:
- 같은 입력에 매번 다른 점수가 나온다 (일자별 비교가 무의미해진다)
- 근거 없이 튀는 값을 사후 검증할 수 없다
- 모델을 바꾸면 과거 점수와 스케일이 끊긴다

권고 구조: **서브스코어는 전부 이미 계산 중인 실수치에서 뽑고, LLM은 근거 문장만 쓴다.**

| 서브스코어 | 입력 (모두 기존 코드에 있음) | 비중(초안) |
|-----------|------------------------------|-----------|
| 기술 | `ComputeTechnicalIndicators` — RSI(14)·MACD·ADX·SMA 배열·52주 위치 | 30% |
| 실적 모멘텀 | `FinancialHistoryService` — 매출/영업이익 YoY·QoQ | 25% |
| 밸류에이션 | `FundamentalsService` — PER/PBR/PSR (자기 이력 대비 백분위) | 20% |
| 뉴스 감성 | `NewsSentimentSummary.AverageScore` (−100~100) | 15% |
| 내부자 | `InsiderTabService` — 최근 순매수/순매도 | 10% |

- 각 서브스코어는 0~100으로 정규화, 데이터 없으면 **해당 항목을 비중에서 제외하고 나머지로 재정규화**(0점 처리 금지 — 없는 데이터를 나쁜 신호로 둔갑시키면 안 된다)
- `score_breakdown`에 서브스코어를 **원본 그대로 저장** → 나중에 비중을 바꿔도 과거 데이터를 재계산할 수 있다
- LLM은 `summary_json`의 4개 문장만 생성. LLM 미설정/실패 시 `source='rule'`로 저장되고 점수는 그대로 유효

---

## 7. API 설계

```
GET /api/v1/stocks/{symbol}/daily-closes?limit=60
GET /api/v1/indices/{indexId}/daily-closes?limit=60
```

응답 (화면 1행 = 배열 1요소):

```json
{
  "data": {
    "symbol": "005930",
    "currency": "KRW",
    "rows": [
      {
        "tradeDate": "2026-08-06",
        "weekday": "목",
        "close": 71200,
        "prevClose": 71271,
        "changeAmount": -71,
        "changeRate": -0.001,
        "score": 64,
        "scoreBreakdown": { "technical": 71, "momentum": 60, "valuation": 55, "news": 68, "insider": 50 },
        "analysisSource": "llm"
      }
    ]
  }
}
```

- `limit` 기본 60 / 최대 120 (화면이 무한 스크롤이 아니라 리스트이므로 충분)
- 종가 행이 있고 분석 행이 없으면 `score: null` → 화면은 점수 칸을 비운다. **분석이 없다고 종가 행을 감추지 않는다**
- 캐시: Redis 엔벨로프 (키 `daily:closes:v1:<symbol>:<limit>`), 확정 데이터라 TTL을 길게(마감 후 갱신) 가져가도 안전

---

## 8. 화면 매핑

| 화면 컬럼 | 응답 필드 | 비고 |
|-----------|-----------|------|
| `'26년 8월 6일 (목)` | `tradeDate` + `weekday` | 요일은 서버에서 KST 기준으로 계산해 내려주는 게 안전(클라 로케일 흔들림 방지) |
| `↘ -0.1%` | `changeRate` | 부호로 색/아이콘 결정. **0.0%와 데이터 없음(null)을 구분**할 것 |
| `+64` | `score` | `null`이면 `-` |

---

## 9. 이미 가진 데이터로 더 줄 수 있는 것

여기부터가 "우리 데이터를 활용해 더 좋은 걸 준다"에 해당하는 부분이다. 추가 API 비용 **0원**인 것부터 나열한다.

### 9.1 점수 적중률 (가장 가치 큼)

점수와 실제 종가를 같은 테이블 셋에 쌓는 순간, **"이 점수가 실제로 맞았는가"를 우리 스스로 계산할 수 있다.** 외부에서 절대 살 수 없는 데이터다.

- 방향 적중률: `score(D) ≥ 60` 인 날들의 **D+1 등락률이 양수인 비율**
- 점수 구간별 평균 D+1 수익률 (구간 단조성이 보이면 점수가 실제로 작동한다는 증거)
- 정보계수(IC): score와 D+1 수익률의 순위상관

> ⚠️ **정렬 규칙이 핵심이다.** 00:00 KST 배치가 만드는 점수는 그날 종가를 **이미 보고** 만든 값이다. 화면처럼 `score(D)` 옆에 `return(D)`를 나란히 두면 그건 예측이 아니라 사후 설명이다.
> 화면 표기는 그대로 두되(그날의 분석 결과라는 의미로 정확하다), **성능 평가는 반드시 `score(D)` vs `return(D+1)`로 해야 한다.** 이걸 섞으면 적중률이 비현실적으로 높게 나오고, 그 수치를 사용자에게 노출하면 심각한 오해를 만든다.

### 9.2 지수 대비 상대수익률 (알파)

지수 종가를 같이 쌓으므로 공짜로 나온다. `종목 등락률 − 소속 지수 등락률` 컬럼 하나면 "시장이 빠져서 빠진 건지, 종목이 문제인지"가 바로 보인다. 국내는 KOSPI/KOSDAQ, 미국은 S&P500/나스닥으로 매핑.

### 9.3 점수 변화 마커

`score(D) − score(D−1)`이 큰 날에 배지를 달고, 그날의 `summary_json` 근거 문장을 펼쳐 보여준다. 이미 저장하는 데이터의 재활용일 뿐이다.

### 9.4 이벤트 오버레이

같은 날짜 축에 이미 있는 것들을 얹는다 — 실적 발표일(`EarningsTabService`), 내부자 거래일(`InsiderTabService`), 뉴스 급증일(`news_articles` 일별 건수). "그날 왜 움직였는가"를 사용자가 스스로 연결할 수 있다.

### 9.5 뉴스 감성 시계열

`NewsSentimentSummary.AverageScore`를 일자별로 남기면 **감성 곡선 vs 주가 곡선** 비교가 가능해진다. 이미 매번 계산하면서 버리고 있는 값이다.

### 9.6 52주 위치 / 거래대금 급증 플래그

`candle_bars` 1d만으로 계산된다. 52주 백분위, 거래량 20일 평균 대비 배수 — 별도 저장 없이 조회 시 계산 가능.

---

## 10. 리스크 · 확인 필요 사항

| 항목 | 내용 |
|------|------|
| **수정주가** | Toss 일봉이 액면분할/배당 반영된 수정주가인지 확인 필요. 원주가라면 분할일 전후 등락률이 왜곡된다. 확인 후 필요하면 재적재 정책 또는 `adjusted` 플래그 |
| **`trade_date` 시장별 계산** | 6.5절 경고 참조. 미국 종목에서 하루 밀리는 사고가 가장 나기 쉬운 지점 |
| **점수 스케일 연속성** | LLM 모델 교체 시 문장은 바뀌어도 점수는 서버 계산이라 유지된다 — 서버 계산을 권고하는 실질적 이유 중 하나 |
| **대상 종목 범위** | 파일럿 10종목으로 시작 권장. 전 종목 확대는 LLM 비용이 아니라 **일봉 완결성 보장 잡의 Toss 호출량**이 병목 |
| **면책 문구** | 점수를 화면에 노출하는 이상 "투자 자문이 아님" 고지가 필요 |
| **다중 인스턴스** | EOD 잡·00:00 배치 모두 리더 선출이 없으면 인스턴스 수만큼 중복 실행된다. UPSERT라 데이터는 안전하지만 LLM 호출은 중복 과금 — AGENT_STATE의 폴러 리더 선출 TODO와 같이 처리 |

---

## 11. 구현 순서 제안

1. `index_daily_closes` 마이그레이션 + 기존 EOD 훅에서 UPSERT (가장 싸고, 없으면 아무것도 못 하는 기반)
2. `GET /indices/{id}/daily-closes` + 종목용 `candle_bars` 조회 API (등락률은 Go 계산)
3. 일봉 완결성 보장 잡 (대상 = AllTrackedSymbols ∪ 파일럿)
4. `stock_daily_analyses` 마이그레이션 + 서버 계산 점수 + 00:00 KST 배치에서 UPSERT
5. FE 리스트 화면 (날짜 / 등락률 / 점수)
6. 지수 대비 알파 컬럼 (9.2)
7. 점수 적중률 모듈 (9.1) — **데이터가 최소 40~60거래일 쌓인 뒤에** 노출. 표본이 적을 때 적중률을 보여주면 노이즈를 실력으로 오독하게 된다

---

## 12. 구현 결과 (2026-08-08)

> 🔴 **2026-08-08 정정**: 아래 §12.1의 "`00:00` 행이 전일 종가를 복제한다"는 진단은 **오진이었습니다.**
> 실제 원인은 `candle_bars.bar_time`의 **시간대 불일치**입니다. `UpsertBatch`가 KST `time.Time`을
> 넘기는데 DSN이 `loc=Local`이라, UTC로 도는 Docker 컨테이너에서는 `2026-08-07 00:00 KST`가
> `2026-08-06 15:00`으로 저장됐습니다(같은 봉, 거래량 완전 일치, 정확히 9시간 차). 호스트에서 돌린
> 도구(Windows=KST)는 같은 봉을 `00:00`에 써서 두 표기가 공존했고, "날짜당 마지막 봉" 규칙이
> **UTC 행을 골라 `trade_date`가 하루 밀렸습니다.** 즉 올바른 쪽은 `00:00`이었습니다.
> 2026-08-02(일요일) 15:00 봉의 존재가 결정적 증거였습니다.
>
> 조치: 일봉은 KST 벽시계 문자열로 저장하도록 쓰기 경로 수정(`IsDailyOrCoarser`),
> 기존 149,803행은 `cmd/repair-candle-tz`로 무손실 보정(세션별로 더 완성된 봉을 남김).
> 1분봉은 Go 읽기 경로가 대칭이라 건드리지 않았습니다.

### 12.1 원안과 달라진 점 — 왜 `stock_daily_closes`를 만들었나

원안은 "종목 종가는 `candle_bars`(1d)를 그대로 읽고 테이블을 새로 만들지 말자"였습니다. 구현 중 **`candle_bars`가 그대로 읽을 수 있는 상태가 아니라는 것**을 실측으로 확인해 방침을 바꿨습니다.

실측한 문제 두 가지:

1. **하루에 1d 행이 두 개** — 같은 날짜에 `00:00`과 `15:00` 행이 함께 있고, `00:00` 행은 **전일 세션의 종가를 그대로 복사**한 값입니다.
   ```
   005930  07-30 15:00  close=259000
   005930  07-31 00:00  close=259000   ← 전일 종가 복제
   005930  08-02 15:00  close=233500
   005930  08-03 00:00  close=233500   ← 전일 종가 복제
   ```
   그대로 읽으면 등락률이 `0% → 두 배` 로 번갈아 나옵니다. 실제로 첫 적재에서 005930이 `-9.85% / +3.64% / 0% / -4.96%` 라는 말이 안 되는 계열을 만들었습니다.
2. **주말 날짜 행** — 2026-08-02(일)에 378개 종목의 1d 행이 존재합니다. 어느 주식시장도 일요일에 거래하지 않으므로 상위 데이터 잡음입니다.

즉 "읽기만 하면 된다"는 전제가 성립하지 않았고, 매 조회마다 이 정규화를 반복하는 것보다 **한 번 정규화해 투영 테이블에 적재**하는 편이 정확하고 싸다고 판단했습니다.

원안이 경계한 "이중 source of truth" 문제는 다음으로 막았습니다.
- 적재는 **항상 단방향** (`candle_bars`/지수 폴러 → `stock_daily_closes`), 역방향 없음
- PK `(symbol, trade_date)` 기준 **멱등 upsert** — 재실행은 같은 값을 다시 쓸 뿐
- 가격 컬럼만 갱신, `score`/`ai_summary`는 UPDATE 목록에서 제외 → 재적재가 분석 결과를 지우지 않음

### 12.2 정규화 규칙 (`dailyCloseSessionBars`)

```sql
ROW_NUMBER() OVER (PARTITION BY symbol, DATE(bar_time) ORDER BY bar_time DESC) = 1  -- 날짜당 마지막 봉만
AND DAYOFWEEK(bar_time) NOT IN (1, 7)                                               -- 주말 날짜 제외
LAG(close_price) OVER (PARTITION BY symbol ORDER BY trade_date)                     -- 중복 제거 후의 전일 종가
```

수정 후 005930: `+2.05% / +0.69% / -0.34% / +0.87% / +0.17%` — 정상 범위. 이미 적재된 주말 행은 `PruneWeekendRows`로 378건 삭제했습니다(투영이 이제 주말을 건너뛰므로 재실행만으로는 고쳐지지 않기 때문).

> **후속 과제**: `candle_bars`에 `00:00` 중복 행을 쓰는 상위 경로가 어디인지는 아직 특정하지 않았습니다. 종가 파이프라인은 방어했지만, 차트/지표 등 `candle_bars`를 직접 읽는 다른 소비자는 여전히 중복 봉을 볼 수 있습니다.

### 12.3 적재 스케줄

`DailyCloseService.RunScheduler` — 고정 tick이 아니라 **다음 마감 시각까지 대기**하므로 장중 재기동해도 정확한 다음 마감에 깨어납니다.

| 시장 | 정규장 마감 | 적재 시각 |
|------|-------------|-----------|
| KRX | 15:30 KST | 마감 +20분 |
| US | 16:00 ET | 마감 +20분 |

+20분은 종가 확정과 폴러의 일봉 기록을 기다리는 여유입니다. 휴장일은 `calendar` 패키지의 휴일 목록으로 건너뜁니다.

### 12.4 점수 — 서버 계산 (§6 원안 유지)

`daily_close_score.go`. LLM은 문장만 쓰고 숫자는 서버가 계산합니다.

| 구성 | 비중 | 출처 |
|------|------|------|
| RSI(14) | 30% | `ComputeTechnicalIndicators` |
| 추세 (종가 vs SMA20/50) | 25% | 〃 |
| MACD vs 시그널 | 20% | 〃 |
| 뉴스 감성 | 15% | `NewsSentiment.AverageScore` |
| 52주 밴드 내 위치 | 10% | 〃 |

- 데이터 없는 구성요소는 **비중에서 제외 후 재정규화** — 없는 데이터가 나쁜 신호로 둔갑하지 않습니다(유닛 테스트로 고정).
- RSI는 단조 증가가 아니라 70 부근에서 정점 후 하락 — 과매수를 최고점으로 읽지 않기 위함(테스트로 고정).
- 점수가 전혀 계산 불가하면 `NULL` → FE는 `-` 로 표기.

실측: 000070(삼양홀딩스) 2026-08-07 → `score=75` (RSI 61.9, 20일선 위, 52주 고점 대비 -26.2%).

### 12.5 API · 화면

```
GET /api/v1/stocks/{symbol}/daily-closes?limit=60   (최대 120)
GET /api/v1/indices/{indexId}/daily-closes?limit=60
```

FE는 `일별 시세` 탭(워크스페이스 + 레거시 `/analysis`). 점수 없는 날은 `-`, 요약 있는 행만 펼쳐집니다.

### 12.6 적재 커버리지 — 미완

**전 종목 적재는 완료되지 않았습니다.** `stocks` 활성 종목은 17,901개인데 `candle_bars`에 일봉이 있는 종목은 소수였습니다. 일봉이 없는 종목은 SQL로 만들어낼 수 없고 Toss에서 종목당 1회씩 받아와야 하는데, Toss 캔들 한도가 **초당 5회**이고 구동 중인 앱과 공유합니다.

```bash
go run ./cmd/backfill-daily-closes                                   # candle_bars → 투영 (API 호출 0)
go run ./cmd/backfill-daily-closes -fetch-missing -fetch-limit=2500  # 없는 종목 Toss로 채우기 (재실행 가능)
```

`-fetch-missing`은 매 실행마다 "아직 없는 종목"을 다시 조회하므로 중단해도 진행분이 보존됩니다. 429는 서버가 준 `Retry-After`만큼 기다렸다가 최대 3회 재시도합니다. 전체를 채우려면 반복 실행이 필요하며, 남은 종목 수는 실행 로그의 `coverage:` 줄에 나옵니다.
