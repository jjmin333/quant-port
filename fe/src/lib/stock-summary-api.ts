import { apiRequest } from "@/lib/api-client";
import type { StockSummaryData } from "@/lib/stock-summary-types";

export function fetchStockSummaryClient(symbol: string): Promise<StockSummaryData> {
  return apiRequest<StockSummaryData>(
    `/api/v1/stocks/${encodeURIComponent(symbol)}/summary`,
    undefined,
    "AI 요약 조회 실패",
  );
}
