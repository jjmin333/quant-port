import { apiRequest } from "@/lib/api-client";
import type { FinancialHistoryData } from "@/lib/financial-history-types";

export function fetchFinancialHistoryClient(
  symbol: string,
  years = 3,
): Promise<FinancialHistoryData> {
  return apiRequest<FinancialHistoryData>(
    `/api/v1/stocks/${encodeURIComponent(symbol)}/financials/history?years=${years}`,
    undefined,
    "재무 히스토리 조회 실패",
  );
}
