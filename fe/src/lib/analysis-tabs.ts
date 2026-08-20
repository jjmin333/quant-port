export const ANALYSIS_TABS = [
  { id: "ai", label: "AI 분석" },
  { id: "company", label: "기업 정보" },
  { id: "valuation", label: "밸류에이션" },
  { id: "financials", label: "재무제표" },
  { id: "earnings", label: "실적" },
  { id: "insider", label: "내부자 거래" },
  { id: "news", label: "뉴스" },
] as const;

export type AnalysisTabId = (typeof ANALYSIS_TABS)[number]["id"];

export const VALUATION_SUB_TABS = [
  { id: "multiples", label: "배수" },
  { id: "indicators", label: "지표" },
] as const;

export type ValuationSubTabId = (typeof VALUATION_SUB_TABS)[number]["id"];

export function parseAnalysisTab(value: string | null): AnalysisTabId {
  if (value && ANALYSIS_TABS.some((t) => t.id === value)) {
    return value as AnalysisTabId;
  }
  return "ai";
}

export function parseValuationSubTab(value: string | null): ValuationSubTabId {
  if (value && VALUATION_SUB_TABS.some((t) => t.id === value)) {
    return value as ValuationSubTabId;
  }
  return "multiples";
}

export function analysisSymbolHref(symbol: string, tab: AnalysisTabId = "ai"): string {
  const params = new URLSearchParams({ symbol, tab });
  return `/analysis?${params.toString()}`;
}
