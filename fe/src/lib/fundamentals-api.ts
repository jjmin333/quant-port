import type { ApiResponse } from "@/lib/portfolio-types";
import { PortfolioFetchError, getApiBaseUrl, parseJson } from "@/lib/api-client";
import type { FundamentalsData } from "@/lib/fundamentals-types";

export async function fetchFundamentalsClient(symbol: string): Promise<FundamentalsData> {
  const res = await fetch(
    `${getApiBaseUrl()}/api/v1/stocks/${encodeURIComponent(symbol)}/fundamentals`,
    { cache: "no-store" },
  );
  if (!res.ok) {
    const body = await parseJson<{ error?: string }>(res).catch(() => ({ error: res.statusText }));
    throw new PortfolioFetchError(body.error || "펀더멘털 조회 실패", res.status);
  }
  const body = await parseJson<ApiResponse<FundamentalsData>>(res);
  return body.data;
}
