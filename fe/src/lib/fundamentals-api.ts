import { apiRequest } from "@/lib/api-client";
import type { FundamentalsData } from "@/lib/fundamentals-types";

export function fetchFundamentalsClient(symbol: string): Promise<FundamentalsData> {
  return apiRequest<FundamentalsData>(
    `/api/v1/stocks/${encodeURIComponent(symbol)}/fundamentals`,
    undefined,
    "펀더멘털 조회 실패",
  );
}
