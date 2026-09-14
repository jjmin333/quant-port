import { apiRequest } from "@/lib/api-client";
import { cachedGet } from "@/lib/fetch-cache";
import type { FundamentalsData } from "@/lib/fundamentals-types";

/**
 * 재무 지표 — 배치가 **하루 주기**로 채운다(화면의 폴링도 `fundamentalsPollHours`,
 * 기본 24시간이다). 같은 화면의 개요·밸류에이션·재무제표 탭이 각자 부르므로 짧게
 * 재사용해 그 중복만 없앤다.
 */
export function fetchFundamentalsClient(symbol: string): Promise<FundamentalsData> {
  const path = `/api/v1/stocks/${encodeURIComponent(symbol)}/fundamentals`;
  return cachedGet(
    path,
    () => apiRequest<FundamentalsData>(path, undefined, "펀더멘털 조회 실패"),
    60_000,
  );
}
