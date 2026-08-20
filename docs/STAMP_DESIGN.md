# 스탬프(Stamp) 설계안

> 상태: **1차 구현 완료** (2026-08-09) — 마이그레이션 `035_stamps.sql`, `StampService`,
> `/api/v1/stamps*`, `/stamps` 화면, 종목 헤더 도장 버튼.
> 관련: `docs/STOCK_SCORE_DESIGN.md`, `docs/DAILY_CLOSE_DESIGN.md`, `docs/AGENT_STATE.md`

## 1. 한 줄 정의

**스탬프 = 내가 시장을 바라본 순간을 도장으로 찍어두고, 시간이 지난 뒤 "그때의 판단이 맞았는지"를 우리 데이터로 자동 채점받는 기록.**

단순 타임캡슐(과거 보존)로 끝내면 두 번째 사용이 일어나지 않는다. 보존은 수단이고,
재방문 동기는 **판정(개봉)** 에서 나온다. 그래서 스탬프는 저장할 때 아주 가벼운
**가설**(강세/약세/관망 + 검증 기간)을 함께 남기고, 기간이 지나면 "개봉 대기"로 떠오른다.

## 2. 왜 우리가 만들면 다른가 — 보유 데이터 매핑

| 스탬프 구성요소 | 우리 데이터 (이미 있음) | 외부 API 호출 |
|---|---|---|
| 당시 주가·등락률·거래량·시총·PER/PBR·52주 | `quote_snapshots` / `symbol_last_prices` / terminal 캐시 | 0 |
| 당시 **종합점수 5축 + 등급 + 피어 백분위** | `stock_scores` (breakdown JSON) | 0 |
| 당시 **AI 요약 + 키워드 + 룰기반 투자포인트/시나리오/주요 가격대** | `stock_daily_closes.ai_summary`, `stock_summary_facts.go`, `stock_briefing.go` | 0 |
| 당시 차트 | `candle_bars` (일봉 673k행 + 분봉) | 0 |
| 당시 뉴스 | `news_articles` (`symbols_json`, `published_at`) | 0 |
| 당시 공시 | DART/EDGAR — **이력 미저장**(Redis 당일 캐시뿐) → §5.2 | 0 (스냅샷 복사) |
| 당시 시장 상황(코스피/나스닥/환율/VIX) | `market_indices`, `index_daily_closes` | 0 |
| 이후 1일·1주·1개월 수익률 | `stock_daily_closes` | 0 |
| **지수 대비 알파** | `index_daily_closes` | 0 |
| 당시 내 보유/관심 상태·평단 | `user_holdings`, `watchlist_items(added_price)` | 0 |

**스탬프 때문에 늘어나는 외부 호출은 없다.** 시세는 종목 화면이 이미 쓰는 표준 read chain
(terminal 캐시 → MySQL → 콜드미스 1× backfill)을 그대로 타고, 점수·뉴스·지수·AI 요약은 전부
DB에서 읽는다. 공시만은 **캐시 전용**(`GetTodayDisclosures`)으로 읽어 DART를 새로 호출하지
않는다 — 사용자가 아무 때나 누르는 버튼이 한도를 태우면 안 되기 때문이다.

경쟁 서비스의 "관심종목 저장"과의 차이는 결국 **점수 5축과 알파**다. 가격만 저장하면
"올랐다/내렸다"밖에 못 말하지만, 우리는 *"주가는 -8%인데 추세 축만 -14 빠졌고 수익성·가치 축은
+6 올랐다. 시장(KOSPI)은 -11%였으니 알파는 +3%p"* 까지 말할 수 있다.

## 3. 사용자 시나리오

1. 마켓 레이더에서 급등 종목을 보고 도장 버튼을 누른다 → **즉시 저장**(1클릭). 토스트가 뜨고
   그 자리에서 관점(강세/약세/관망)·한 줄 메모를 덧붙일 수 있다(선택).
2. 한 달 뒤 사이드바 `기록 > 스탬프` 에 **"개봉 대기 3"** 배지가 뜬다.
3. 열면 좌우 대조: **당시 vs 현재** — 가격, 점수 5축, AI 요약, 그때의 뉴스 헤드라인.
4. 상단에 판정 한 줄: `강세 관점 · 1개월 · +6.2% (KOSPI 대비 +3.1%p) · 적중`
5. 누적되면 요약 카드: **내 적중률 62% (18/29) · 평균 알파 +1.8%p · 강세 관점이 약세보다 정확**

## 4. 화면·조작 (사용자 결정 사항 반영)

### 4.1 스탬프 버튼

`관심 추가/해제`(★) 버튼 바로 옆에 도장 버튼(lucide `Stamp`). 두 곳에 동일 컴포넌트 배치:
- `fe/src/components/stock-workspace/stock-workspace-header.tsx:105` 부근
- `fe/src/components/stocks/stock-detail-client.tsx:175` 부근

상태 3가지:
| 상태 | 표시 |
|---|---|
| 기본 | 외곽선 도장 + `스탬프` |
| 이 종목 스탬프 보유 | 채워진 도장 + 개수 배지(`3`) — 클릭 시 새 스탬프, 배지 클릭 시 목록 |
| 쿨다운(5분 내 재스탬프) | 비활성 + `4:12 후 가능` 툴팁 |

**마찰 최소화가 사용률을 결정한다.** 모달을 먼저 띄우지 않는다 — 클릭 = 낙관적 즉시 저장,
메모·관점은 토스트에서 인라인으로 붙인다(안 붙여도 유효한 스탬프).

### 4.2 `기록 > 스탬프` 탭 (`/stamps`)

사이드바 `기록` 그룹에 `{ href: "/stamps", label: "스탬프", icon: Stamp }` 추가
(`fe/src/components/layout/sidebar.tsx:65`).

- **상단 요약 바**: 총 스탬프 · 개봉 대기 · 적중률 · 평균 알파
- **뷰 토글**: 타임라인(날짜 역순) / 종목별 묶음
- **필터**: 종목(기존 `StockCatalogSearchField` 재사용) · 기간 · 관점 · 적중여부 · 보유/관심
- **카드 1행**: 종목명 · 스탬프 시각(KST) · 당시가 → 현재가 · `1D/1W/1M` 수익률 칩 ·
  점수 변화(`68 → 61 ▼7`) · 메모 발췌
- **상세**: 좌 "당시" / 우 "현재" 2열 대조 + 차트(스탬프 시점 마커, 이후 경로 강조) +
  당시 뉴스·공시 리스트 + 점수 5축 막대 2개 겹쳐보기. 375px에서는 세로 스택.
- **비교 모드**: 같은 종목 스탬프 2개 선택 → 두 시점 간 축별 변화. *"지난번 봤을 때와 무엇이
  달라졌나"* 는 재방문 사용자의 실제 질문이다.
- 행 클릭 → `analysisSymbolHref`로 AI 분석 탭 이동(랭킹 패널·메모장과 동일 패턴).

## 5. 데이터 설계

### 5.1 불변 / 가변 분리 (핵심 원칙)

- **불변(저장 시 1회 복사, 이후 절대 UPDATE 금지)**: 가격·등락률·거래량·PER/PBR·52주,
  점수 5축·등급·피어 백분위, AI 요약 텍스트·키워드, 뉴스 N건, 공시 N건, 지수 스냅샷,
  보유/관심 상태·평단, 차트 봉 배열.
- **가변(조회 시 계산)**: 현재가, 1D/1W/1M/3M 수익률, 알파, 현재 점수·축 변화, 적중 판정.

사용자가 수정할 수 있는 건 **메모·태그·관점·검증기간뿐**. 스냅샷을 수정 가능하게 하면
기록이 아니라 창작이 되고 적중률 통계가 무의미해진다.

### 5.2 공시는 왜 스냅샷을 복사해야 하나

DART/EDGAR 공시는 현재 **MySQL에 이력이 없다**(`GetTodayDisclosures`가 Redis 당일 캐시만 유지).
나중에 다시 조회하려면 DART API를 날짜 범위로 때려야 하고, 이건 스탬프 조회마다 외부 한도를
태우는 설계다. 따라서 **스탬프 생성 시점의 캐시 내용을 그대로 복사**한다.
캐시가 비어 있으면 빈 배열 + `dataGaps: ["disclosures"]` 로 남긴다 —
**없는 데이터를 나중에 지어내지 않는다**(LLM facts의 `dataGaps` 와 동일 철학).

뉴스는 `news_articles` 에 이력이 있으므로 id 참조만 저장해도 되지만, 기사 삭제/재수집으로
링크가 끊길 수 있어 **헤드라인·URL·게시시각까지 복사**한다(id는 함께 보관해 살아 있으면 리치 렌더).

### 5.3 차트 재현

일봉은 `candle_bars` 로 언제든 재현 가능하다. 다만 **분봉은 종목·기간 커버리지가 보장되지
않아** 그대로 두면 "그때 본 차트"가 사라질 수 있다. 그래서 저장 시 화면에 보이던 구간의
봉을 **최대 200개** JSON으로 함께 박제한다(스탬프당 약 8~12KB). 상세 화면은
`chart.bars` 를 우선 렌더하고, 스탬프 이후 구간만 `candle_bars` 에서 이어 붙여
"찍은 순간 → 그 이후" 를 한 차트에 그린다.

### 5.4 마이그레이션 `035_stamps.sql`

```sql
USE quantport;

CREATE TABLE IF NOT EXISTS stamps (
  id              BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
  user_id         BIGINT UNSIGNED NOT NULL,
  symbol          VARCHAR(32)  NOT NULL,
  symbol_name     VARCHAR(100) NOT NULL DEFAULT '',
  asset_type      VARCHAR(16)  NOT NULL DEFAULT '',
  currency        CHAR(3)      NOT NULL DEFAULT 'KRW',
  -- 저장은 UTC, 표시는 항상 Asia/Seoul (2026-08-08 KST 감사 결론 준수)
  stamped_at      TIMESTAMP(3) NOT NULL,
  -- 수익률 기준일. 시장별 거래일이 다르므로 calendar 패키지로 판정
  trade_date      DATE         NOT NULL,
  market_session  VARCHAR(16)  NOT NULL DEFAULT '',
  benchmark_id    VARCHAR(32)  NOT NULL DEFAULT '',   -- kospi | kosdaq | sp500 | nasdaq100
  price           DOUBLE       NOT NULL,
  prev_close      DOUBLE       NULL,
  change_rate     DOUBLE       NULL,
  volume          BIGINT       NOT NULL DEFAULT 0,
  score           TINYINT      NULL,
  grade           VARCHAR(2)   NULL,
  stance          VARCHAR(8)   NOT NULL DEFAULT 'neutral',  -- bullish | bearish | neutral
  conviction      TINYINT      NOT NULL DEFAULT 2,          -- 1~3
  horizon_days    SMALLINT     NOT NULL DEFAULT 30,         -- 7 | 30 | 90
  note            VARCHAR(1000) NOT NULL DEFAULT '',
  tags_json       JSON         NULL,
  -- 당시 시세·밸류에이션·점수 breakdown·보유/관심 상태
  snapshot        JSON         NOT NULL,
  -- 당시 뉴스·공시·지수·AI 요약·dataGaps
  context         JSON         NOT NULL,
  -- { interval, bars: [...최대 200] }
  chart           JSON         NOT NULL,
  opened_at       TIMESTAMP(3) NULL,
  created_at      TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at      TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  KEY idx_stamps_user_time (user_id, stamped_at DESC),
  KEY idx_stamps_user_symbol (user_id, symbol, stamped_at DESC),
  KEY idx_stamps_user_open (user_id, opened_at, stamped_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
```

정렬·필터에 쓰는 값만 컬럼, 나머지는 JSON — `stock_scores.breakdown/inputs` 와 같은 패턴.
`MigrateStampsIfNeeded` 는 `journal_notes`/`batch_job_runs` 와 동일한 idempotent 방식.

### 5.5 5분 중복 방지

서버가 판정한다(클라이언트 비활성화는 UX용 보조):

```sql
SELECT stamped_at FROM stamps
 WHERE user_id = ? AND symbol = ? AND stamped_at > NOW(3) - INTERVAL 5 MINUTE
 ORDER BY stamped_at DESC LIMIT 1
```

존재하면 **409** + `retryAfterSeconds`. FE는 이 값으로 카운트다운을 그린다.

## 6. API

전부 `authctx.RequireUserID` + `WHERE id = ? AND user_id = ?` 소유권 검증(메모장과 동일).

| 메서드 | 경로 | 설명 |
|---|---|---|
| POST | `/api/v1/stamps` | 생성. 본문은 `symbol` + 선택값(stance/conviction/horizonDays/note/interval). **스냅샷 수집은 전적으로 서버 책임** — 클라이언트가 보낸 가격을 믿지 않는다 |
| GET | `/api/v1/stamps` | 목록(symbol·기간·stance·적중여부 필터, cursor 페이지네이션) |
| GET | `/api/v1/stamps/{id}` | 상세 = 불변 스냅샷 + 현재 비교(계산) |
| PATCH | `/api/v1/stamps/{id}` | 메모·태그·관점·검증기간만 수정 |
| POST | `/api/v1/stamps/{id}/open` | 개봉 확인(개봉 대기에서 제외) |
| DELETE | `/api/v1/stamps/{id}` | 삭제 |
| GET | `/api/v1/stamps/summary` | 총계·개봉 대기·적중률·평균 알파 |
| GET | `/api/v1/stamps/state?symbol=` | 버튼 상태(개수 + 쿨다운 잔여초) |

## 7. 성과·판정 계산

- **기준가** = 스탬프 `price`. 비교값은 `stock_daily_closes` 종가, 당일치는 현재가.
- **구간**: 1D / 1W / 1M / 3M. 캘린더 기준이되 해당일에 거래일이 없으면 직전 거래일 종가.
  단 직전 종가가 **스탬프 당일 이하**면 그 구간은 아직 안 지난 것이다 —
  이때만 스탬프 이후 첫 종가를 쓰되 **목표일 +5일 이내**인 경우로 제한한다
  (금요일 스탬프의 1D 목표일은 토요일이라 이 보정이 없으면 1D가 영원히 비고,
  제한이 없으면 거래가 끊긴 종목의 몇 달 뒤 종가가 1D 칸을 채운다).
- **알파** = 종목 수익률 − `benchmark_id` 수익률(`index_daily_closes`). 국내는 kospi/kosdaq,
  해외는 sp500(개별주)·nasdaq100(기술주 ETF) 를 `asset_type`으로 자동 배정하되 저장 후 고정.
- **미도래 구간은 `null`** → FE `—`. 있는 척하지 않는다.
- **적중 판정**(horizon 종료 후): `bullish` = 수익률 > 0, `bearish` = 수익률 < 0,
  `neutral` = |수익률| < 3%. 알파는 병기하되 판정 기준은 단순 수익률(설명 가능성 우선).
- **배치 불필요** — 사용자당 스탬프 수가 수백 규모라 조회 시 계산으로 충분. 목록은 최신 구간만,
  상세에서 전 구간 계산.

> 이건 사용자 **본인의 판단에 대한 자기 채점**이지 서비스의 매매 추천이 아니다.
> 화면 문구도 "적중/빗나감"까지만 쓰고 "매수/매도하세요" 로 넘어가지 않는다.

## 8. 이후 확장 (1차 범위 밖)

- **개봉 알림**: 현재 푸시 인프라가 없으므로 1차는 사이드바 배지 + 목록 상단 섹션.
- **자동 스탬프 제안**: 급등락·점수 급변·실적 발표일에 "지금 찍어둘까요?" 힌트(배지 수준, 자동 저장 금지).
- **메모장·목표가 연계**: 스탬프에서 `journal_notes` 메모 생성, `/targets` 목표가 등록.
- **집계 인사이트**: "점수 70+ 에서 찍은 스탬프의 1개월 평균 알파" — `stock_scores` 예측력
  검증(AGENT_STATE TODO)과 같은 축으로 붙일 수 있다.

## 9. 가드레일

| 항목 | 값 |
|---|---|
| 중복 스탬프 | 동일 종목 5분 (서버 409) |
| 사용자당 상한 | 5,000개 (초과 시 안내) |
| 차트 봉 상한 | 200봉 / 스탬프 (약 10KB) |
| 뉴스·공시 복사 | 각 최대 5건 |
| 외부 API 호출 | 생성·조회 모두 **0회** |
| 시간 표기 | 저장 UTC / 표시 `Asia/Seoul` 명시(`formatDateTimeKST`) |

## 10. 구현 중 확정/수정된 사항 (2026-08-09)

- **거래량 폴백**: 장 마감 후 라이브 시세는 거래량 0을 반환한다. 그대로 박제하면 실제로
  거래된 세션이 "거래량 —"으로 남으므로, 0이면 `stock_daily_closes.volume`으로 대체한다.
- **음수 등락률 유실 버그**: 기존 `repository.nullFloat`은 `*v <= 0`을 NULL로 바꾼다(보유수량용).
  이걸 그대로 쓰면 **하락 스탬프의 등락률이 전부 사라진다** — `nullFloat64`로 교체.
- **현재 점수 동봉**: 목록/상세 응답의 `current`에 오늘 점수·등급·5축을 채워 넣는다
  (당시 breakdown을 저장하는 이유가 이 대조이므로).
- 스탬프 목록의 현재가는 `BatchQuoteSummaries`(관심종목 목록과 동일 경로)로 종목당 1회.
  현재 점수는 종목당 1쿼리 — 스탬프 수백 건 규모에서는 충분하나, 커지면 배치 조회로 교체.

## 11. 구현 순서

1. `035_stamps.sql` + `MigrateStampsIfNeeded` + `StampRepo`
2. `StampService` — 스냅샷 수집기(quote/score/news/index/holding 각각 nil-safe, 하나가 없어도 저장 성공) + 쿨다운
3. `StampHandler` + 라우터 + DB 관리 카탈로그(`db_admin_catalog.go`)에 `stamps` 행 추가
4. 성과 계산기 + 유닛테스트(거래일 경계·미도래 구간 null·알파·판정)
5. FE: 도장 버튼 컴포넌트(2개 호출부) + `use-stamps` 훅
6. FE: `/stamps` 목록·상세·비교
7. 검증: go build/vet/test, tsc/next build, Docker 재기동, 375/1024/1440px 실측
