import {
  parseStockWorkspaceTab,
  type StockWorkspaceTabId,
} from "@/lib/stock-workspace-tabs";

/**
 * AI 추천 랭킹(/analysis)의 URL.
 *
 * # 탭 목록을 따로 갖지 않는다 (2026-09-03 사용자 결정)
 *
 * 예전에는 이 파일이 자기만의 탭 일곱 개(`ANALYSIS_TABS`)를 들고 있었고, 그래서
 * 랭킹에서 고른 종목과 실시간 차트에서 고른 종목이 **서로 다른 화면**으로 열렸다 —
 * 개요·차트가 없고, 헤더 모양도 달랐다. 지금은 두 화면이 같은 워크스페이스
 * (`StockWorkspaceClient`)를 그리므로 탭도 그쪽 목록 하나뿐이다.
 *
 * 남은 것은 **주소 모양**뿐이다: 랭킹은 목록과 상세가 한 화면이라 `?symbol=`로
 * 종목을 가리키고, 종목 페이지(`/stocks/[symbol]`)는 경로로 가리킨다.
 */
export function analysisSymbolHref(
  symbol: string,
  tab: StockWorkspaceTabId = "overview",
  /**
   * 왼쪽 목록이 지금 무엇을 보여 주고 있는지(시장·축·기간). **같이 실어야** 종목을
   * 고른 뒤에도 그 목록이 그대로 있는다 — 빠뜨리면 「해외 · 성장 · 1주」를 골라 둔
   * 사람이 종목 하나를 누르는 순간 국내 · 종합 · 실시간으로 되돌아간다.
   *
   * 앱 다른 곳(검색·점수 칩)에서 부를 때는 목록 상태를 모르므로 생략한다.
   */
  ranking?: URLSearchParams,
): string {
  const params = new URLSearchParams(ranking);
  params.set("symbol", symbol);
  params.set("tab", tab);
  return `/analysis?${params.toString()}`;
}

/** @deprecated `parseStockWorkspaceTab`을 쓴다 — 탭 목록은 한 벌뿐이다. */
export const parseAnalysisTab = parseStockWorkspaceTab;

/** @deprecated `StockWorkspaceTabId`를 쓴다. */
export type AnalysisTabId = StockWorkspaceTabId;
