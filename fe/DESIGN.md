# Quant Port — Design System

## 컨셉

**Professional Dark Terminal** — 장시간 모니터링에 적합한 다크 테마 기반의 금융 대시보드.
Bloomberg / 토스증권 / 키움 HTS의 장점을 결합하되, **통합 자산**에 초점.

### 차별화 UI 원칙

1. **한눈에 총자산** — 로그인 후 첫 화면에서 핵심 KPI 3초 내 파악
2. **증권사 통합** — 계좌별 + 종목별 합산을 동일한 카드/테이블 패턴으로 표시
3. **정보 밀도 조절** — 대시보드는 요약, 상세는 종목 페이지로 분리
4. **숫자 가독성** — 금액/수익률은 `font-mono`, 색상으로 상승/하락 즉시 인지

---

## 컬러 팔레트

| 토큰 | 값 | 용도 |
|------|-----|------|
| `--background` | `#0B0F17` | 앱 배경 |
| `--surface` | `#111827` | 카드, 사이드바 |
| `--surface-elevated` | `#1A2030` | 호버, 강조 영역 |
| `--border` | `#1F2937` | 구분선 |
| `--text-primary` | `#F1F5F9` | 제목, 금액 |
| `--text-secondary` | `#94A3B8` | 본문 |
| `--text-muted` | `#64748B` | 라벨, 보조 |
| `--brand` | `#3B82F6` | 브랜드, 링크, 활성 |
| `--profit` | `#EF4444` | 수익, 상승 (한국식 — 상승이 빨강) |
| `--loss` | `#3B82F6` | 손실, 하락 (한국식 — 하락이 파랑) |

---

## 타이포그래피 (v1.0.0)

**폰트는 Pretendard Variable 한 벌이다.** 숫자도 같은 글꼴을 쓰고 `tabular-nums`로 자릿수를 맞춘다.

| 클래스 | 뜻 |
|---|---|
| `font-num` | 숫자 칸 — Pretendard + `tabular-nums`. 예전 `font-mono`(Geist Mono)를 대체 |
| `font-code` | 코드·식별자 — 시스템 고정폭. 관리자 화면의 Redis 키·SQL·컬럼명, 마크다운 `<code>` |

크기는 이름 있는 계단 일곱 개다. 정의는 `src/app/globals.css`의 `@theme inline` **한 곳**:

| 토큰 | 크기 | 용도 |
|---|---|---|
| `text-micro` | 11px | 배지·칩·각주·메타 — **하한** |
| `text-cell` | 12px | 표 셀·목록 본문·기본 UI |
| `text-note` | 13px | 강조된 보조 · 위젯 제목 |
| `text-read` | 14px / 1.7 | **읽는 글** |
| `text-article` | 15px / 1.85 | 문서 본문(리포트·기사) |
| `text-section` | 15px | 섹션 제목 |
| `text-page` | 19px | 페이지 제목 |

- 총자산·KPI는 Tailwind 기본 계단(`text-2xl` 이상)을 그대로 쓴다
- **긴 문단에 `text-cell`을 쓰지 않는다** — 두세 문장이 넘으면 `text-read`
- 새 계단을 추가하면 `src/lib/utils.ts`의 `TEXT_SCALE`에도 넣는다 (안 넣으면 `cn()`이 조용히 지운다)

자세한 규칙과 실측 근거 → **[docs/UI_SPEC.md §14](../docs/UI_SPEC.md)** · 실물 견본 → `/dev/typography`

---

## 컴포넌트

| 컴포넌트 | 파일 | 용도 |
|----------|------|------|
| `Card` | `components/ui/card.tsx` | 기본 컨테이너 |
| `StatCard` | `components/ui/stat-card.tsx` | KPI 숫자 |
| `Badge` | `components/ui/badge.tsx` | 상태, 태그 |
| `Sidebar` | `components/layout/sidebar.tsx` | 전역 네비게이션 |
| `Header` | `components/layout/header.tsx` | 페이지 헤더 |
| `Button` | `components/ui/button.tsx` | **shadcn** — 아직 호출부 없음(2단계에서 교체) |

---

## shadcn/ui 전환 (진행 중)

전체 UI를 shadcn 기본 스타일로 옮기는 중이다. 계획·토큰 충돌 처리·단계별 범위는 **[docs/SHADCN_MIGRATION.md](../docs/SHADCN_MIGRATION.md)**.

지금 알아야 할 것 두 가지:

1. **브랜드 색 토큰은 `--brand`다.** 예전 이름 `--accent`는 shadcn이 "은은한 hover 표면"이라는 다른 뜻으로 쓰기 때문에 넘겨줬다. 새 코드에서 브랜드 블루가 필요하면 `var(--brand)`.
2. 위 컬러 팔레트 표의 값은 **아직 유효하다**. shadcn 토큰(`--primary`·`--card`·`--muted` …)은 정의만 돼 있고 화면에 쓰이는 곳은 없다. 2단계에서 컴포넌트와 함께 넘어간다.

---

## 페이지 구조 (IA)

```
/dashboard          ← 통합 자산 대시보드 (메인)
/holdings           ← 보유종목 통합
/stocks/[symbol]    ← 종목 상세 (핵심)
/watchlist          ← 관심종목
/quotes             ← 실시간 시세
/orders             ← 실시간 주문
/financials         ← 재무제표
/calendar           ← 실적 캘린더
/news               ← 뉴스
/disclosures        ← 공시
/ai-analysis        ← AI 분석
/portfolio          ← 포트폴리오 분석
/risk               ← 리스크 분석
/transactions       ← 거래내역
/stamps             ← 스탬프
/journal            ← 메모장
/targets            ← 목표가
/backtest           ← 백테스트
/settings           ← 설정
```

---

## 반응형

| Breakpoint | 레이아웃 |
|------------|----------|
| Mobile (`<640px`) | 사이드바 → 햄버거 메뉴 (추후), 카드 1열 |
| Tablet (`640~1024px`) | 2열 그리드 |
| Desktop (`>1024px`) | 사이드바 고정 + 3열 그리드 |

---

## 콘텐츠 컬럼 (레이아웃 아케타입)

모든 페이지가 **같은 중앙 정렬 규칙**을 쓰고 `max-width`만 성격에 따라 고른다.
페이지 본문과 그 `PageHeader`는 **반드시 같은 아케타입**이어야 한다 — 어긋나면 제목 컬럼과
본문 컬럼의 시작 x가 달라져 그 페이지만 겉돌아 보인다(마켓 레이더에서 실제로 발생했던 문제).

| 아케타입 | 클래스 / 토큰 | 폭 | 적용 |
|---|---|---|---|
| `reading` | `.page-shell` / `--content-max-reading` | `min(72rem, max(42rem, 68%))` | 홈·뉴스·설정·관리자·포트폴리오·스탬프 등 문서형 |
| `wide` | `.page-shell-wide` / `--content-max-wide` | `100%` | `/market`, `/quotes`, `/stocks/[symbol]` 터미널형 |

- **아케타입이 통일하는 것은 "폭"이 아니라 "규칙"이다.** 헤더 컬럼과 본문 컬럼이 같은 토큰을
  쓰게 만드는 것까지가 목적이고, 터미널형의 폭 자체는 문서형에 맞추지 않는다.
- **`wide`를 72rem으로 잘라 폭까지 맞춰본 적이 있으나 되돌렸다.** 1908px에서 컬럼 1152px·좌우
  268px로 `reading`(1148px)과 4px 차까지 갔지만, 워크스페이스가 1318 → 830으로 줄어 실사용에서
  보기 불편하다는 판단이었다. 터미널형은 밀도가 곧 기능이다.
- 다시 상한을 넣는다면 반드시 `min(<값>, 100%)` 형태여야 한다. `100%` 항이 없으면 `AppShell`의
  `overflow-x-auto`가 좁은 화면에서 가로 스크롤을 만든다.

| 뷰포트 | 콘텐츠 = `wide` | `reading` | 리스트 / 워크스페이스 |
|---|---|---|---|
| 1024 | 804 | 672 | 320 / 472 |
| 1440 | 1220 | 830 | 320 / 888 |
| 1908 | 1688 | 1148 | 368 / 1308 |

- `PageHeader`의 `wide` 분기는 `--content-gutter`(4vw)가 아니라 터미널 패널과 같은 `px-3`을
  쓴다. 4vw는 1920px에서 제목을 76px 들여써 **새로운 어긋남**을 만들었다.
- `.page-shell-wide`에 좌우 경계선을 긋지 않는다. 상한이 `100%`라 컬럼이 항상 꽉 차서, 좌측
  경계선이 사이드바 테두리와 겹쳐 이중선이 되고 우측은 뷰포트 가장자리에 붙는다.
- 모바일/웹뷰(`html[data-shell=mobile|webview]`)에서는 `reading`도 `100%`가 된다.

```tsx
<PageHeader title="마켓 레이더" width="wide" />
<main className="flex min-h-0 flex-1 justify-center overflow-hidden">
  <div className="page-shell-wide flex h-full min-w-0 flex-row">{/* 리스트 + 워크스페이스 */}</div>
</main>
```

---

## 디자인 완료 화면

- [x] 통합 대시보드 (목업 데이터)
- [x] 보유종목 통합
- [x] 종목 상세 (삼성전자 등)
- [x] 관심종목
- [ ] 나머지 16개 페이지 — 스텁 (다음 단계에서 순차 디자인)

---

## 다음 디자인 단계 (제안)

1. **실시간 시세 / 주문** — 트레이딩 터미널 레이아웃
2. **종목 상세 탭** — 재무/뉴스/공시/AI 통합 탭 UI
3. **모바일** — 하단 탭바 + 축소 대시보드
4. **라이트 모드** — CSS 변수 토글

---

## 기술 스택 (프론트)

- **Next.js 16** + TypeScript + Tailwind CSS v4
- **Recharts** — 자산 비중, 자산 변화 차트
- **Lucide React** — 아이콘
- 추후: TradingView Widget, shadcn/ui, TanStack Query

## 백엔드 (추후)

- **Go** + **MySQL** + **Redis**
- 목업 데이터 → Go API로 교체
