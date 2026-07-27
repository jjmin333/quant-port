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
| `--accent` | `#3B82F6` | 브랜드, 링크, 활성 |
| `--profit` | `#10B981` | 수익, 상승 |
| `--loss` | `#EF4444` | 손실, 하락 |

---

## 타이포그래피

- **본문**: Geist Sans
- **숫자/금액/수익률**: Geist Mono (`font-mono`)
- **총자산**: `text-4xl ~ 5xl`, `font-bold`
- **카드 수치**: `text-xl`
- **라벨**: `text-xs`, `text-muted`

---

## 컴포넌트

| 컴포넌트 | 파일 | 용도 |
|----------|------|------|
| `Card` | `components/ui/card.tsx` | 기본 컨테이너 |
| `StatCard` | `components/ui/stat-card.tsx` | KPI 숫자 |
| `Badge` | `components/ui/badge.tsx` | 상태, 태그 |
| `Sidebar` | `components/layout/sidebar.tsx` | 전역 네비게이션 |
| `Header` | `components/layout/header.tsx` | 페이지 헤더 |

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
/journal            ← AI 투자일지
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
