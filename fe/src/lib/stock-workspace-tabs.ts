export const STOCK_WORKSPACE_TABS = [
  { id: "overview", label: "개요" },
  // AI 분석 sits directly after 개요: the header shows only a one-line summary,
  // and this is where the full analysis plus the dated score history lives. It
  // supersedes the former 일별 시세 tab, which it absorbed.
  { id: "ai", label: "AI 분석" },
  { id: "chart", label: "차트·시세" },
  { id: "company", label: "기업 정보" },
  { id: "valuation", label: "밸류에이션" },
  { id: "financials", label: "재무제표" },
  { id: "earnings", label: "실적" },
  { id: "insider", label: "내부자 거래" },
  { id: "news", label: "뉴스" },
] as const;

export type StockWorkspaceTabId = (typeof STOCK_WORKSPACE_TABS)[number]["id"];

/** Tabs shown on 종목 분석 (/quotes, /stocks) — no live chart terminal. */
export const STOCK_ANALYSIS_TABS = STOCK_WORKSPACE_TABS.filter((t) => t.id !== "chart");

export type StockWorkspaceMode = "full" | "analysis";

/** 마켓 레이더 — chart 탭 라벨만 차트·호가 */
export const MARKET_RADAR_WORKSPACE_TABS = STOCK_WORKSPACE_TABS.map((tab) =>
  tab.id === "chart" ? { ...tab, label: "차트·호가" } : tab,
);

export function stockWorkspaceTabsForMode(mode: StockWorkspaceMode) {
  return mode === "analysis" ? STOCK_ANALYSIS_TABS : STOCK_WORKSPACE_TABS;
}

export function coerceAnalysisTab(tab: StockWorkspaceTabId): StockWorkspaceTabId {
  return tab === "chart" ? "overview" : tab;
}

/** @deprecated use STOCK_WORKSPACE_TABS */
export const ANALYSIS_TABS = STOCK_WORKSPACE_TABS.filter(
  (t) => t.id !== "overview" && t.id !== "chart",
);

/** @deprecated use StockWorkspaceTabId */
export type AnalysisTabId = Exclude<StockWorkspaceTabId, "overview" | "chart">;

const VALID_TAB_IDS = new Set<string>(STOCK_WORKSPACE_TABS.map((t) => t.id));

export function parseStockWorkspaceTab(value: string | null): StockWorkspaceTabId {
  if (value && VALID_TAB_IDS.has(value)) {
    return value as StockWorkspaceTabId;
  }
  return "overview";
}

/** @deprecated use parseStockWorkspaceTab */
export function parseAnalysisTab(value: string | null): AnalysisTabId {
  const tab = parseStockWorkspaceTab(value);
  if (tab === "overview" || tab === "chart") return "company";
  return tab;
}

export function stockWorkspaceHref(symbol: string, tab: StockWorkspaceTabId = "overview"): string {
  return `/stocks/${encodeURIComponent(symbol)}?tab=${tab}`;
}
