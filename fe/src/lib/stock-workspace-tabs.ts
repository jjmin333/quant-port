/**
 * 종목 화면 탭 — **앱 전체에 한 벌뿐이다** (2026-10-09 사용자 요청 1).
 *
 * 예전에는 실시간 차트(마켓 레이더)만 「차트·호가」를 숨긴 목록을 따로 들고 있어, 같은 종목이
 * 검색(/stocks)·AI 추천 랭킹·스크리너로 들어오면 차트 탭이 다시 보였다. 이제 진입 경로가
 * 어디든 이 목록 하나를 그린다. 차트는 개요의 일봉 카드가 맡는다.
 */
export const STOCK_WORKSPACE_TABS = [
  { id: "overview", label: "개요" },
  // AI 분석 sits directly after 개요: the header shows only a one-line summary,
  // and this is where the full analysis plus the dated score history lives.
  { id: "ai", label: "AI 분석" },
  /*
    기업 정보 · 밸류에이션 · 재무제표를 **한 탭**으로 합쳤다 (2026-10-07 사용자 요청 10).
    셋은 탭 안 왼쪽 위 토글로 오간다(`FUNDAMENTAL_VIEWS`). 예전 주소(?tab=valuation ·
    ?tab=financials)는 이 탭의 해당 보기로 열린다(`parseFundamentalView`).
  */
  { id: "company", label: "기업·재무" },
  { id: "earnings", label: "실적" },
  { id: "insider", label: "내부자 거래" },
  { id: "news", label: "뉴스" },
  // 이 종목을 붙이거나 태그한 커뮤니티 글 + 리포트 (2026-09-28 요청 3). 글이 없으면 숨김.
  { id: "community", label: "커뮤니티" },
  // 해외 종목의 Reddit 버즈·AI 요약·인기 글 (2026-10-09 요청 4). 자료가 없으면 숨김.
  { id: "reddit", label: "레딧" },
] as const;

export type StockWorkspaceTab = { id: StockWorkspaceTabId; label: string };
export type StockWorkspaceTabId = (typeof STOCK_WORKSPACE_TABS)[number]["id"];

/** 「기업·재무」 탭 안의 세 보기. */
export const FUNDAMENTAL_VIEWS = [
  { id: "company", label: "기업 정보" },
  { id: "valuation", label: "밸류에이션" },
  { id: "financials", label: "재무제표" },
] as const;

export type FundamentalView = (typeof FUNDAMENTAL_VIEWS)[number]["id"];

const VALID_TAB_IDS = new Set<string>(STOCK_WORKSPACE_TABS.map((t) => t.id));
const FUNDAMENTAL_VIEW_IDS = new Set<string>(FUNDAMENTAL_VIEWS.map((v) => v.id));

/** 모르는 값(옛 `?tab=chart` 포함)은 개요로 연다. */
export function parseStockWorkspaceTab(value: string | null): StockWorkspaceTabId {
  if (value && VALID_TAB_IDS.has(value)) {
    return value as StockWorkspaceTabId;
  }
  // 합쳐진 두 탭의 옛 주소는 「기업·재무」로 연다.
  if (value && FUNDAMENTAL_VIEW_IDS.has(value)) return "company";
  return "overview";
}

/** `?tab=valuation` 같은 옛 주소가 고른 보기. 아니면 기업 정보. */
export function parseFundamentalView(value: string | null): FundamentalView {
  return value && FUNDAMENTAL_VIEW_IDS.has(value) ? (value as FundamentalView) : "company";
}

export function stockWorkspaceHref(symbol: string, tab: StockWorkspaceTabId = "overview"): string {
  return `/stocks/${encodeURIComponent(symbol)}?tab=${tab}`;
}
