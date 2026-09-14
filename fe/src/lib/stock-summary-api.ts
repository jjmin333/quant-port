import { apiRequest } from "@/lib/api-client";
import { cachedGet } from "@/lib/fetch-cache";
import type { StockSummaryData } from "@/lib/stock-summary-types";

/**
 * 저장된 AI 요약을 읽기만 한다 — 만드는 것은 배치와 관리자뿐이다.
 *
 * 그래서 **1분 재사용이 안전하다**: 이 경로가 새 문장을 만들어 낼 수는 없고, 같은
 * 화면의 개요 카드·AI 분석 탭·헤더가 같은 순간에 각자 물어보던 것만 사라진다.
 */
export function fetchStockSummaryClient(symbol: string): Promise<StockSummaryData> {
  const path = `/api/v1/stocks/${encodeURIComponent(symbol)}/summary`;
  return cachedGet(
    path,
    () => apiRequest<StockSummaryData>(path, undefined, "AI 요약 조회 실패"),
    60_000,
  );
}
