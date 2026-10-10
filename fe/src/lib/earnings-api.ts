import { apiRequest } from "@/lib/api-client";
import { cachedGet } from "@/lib/fetch-cache";
import type {
  EarningsDaySummary,
  EarningsEvent,
  EarningsMarketFilter,
  EarningsHighlights,
  EarningsStats,
  EarningsTabData,
} from "@/lib/earnings-types";

export function fetchEarningsClient(symbol: string): Promise<EarningsTabData> {
  return apiRequest<EarningsTabData>(
    `/api/v1/stocks/${encodeURIComponent(symbol)}/earnings`,
    undefined,
    "실적 정보 조회 실패",
  );
}

/**
 * 여러 종목의 실적·공시를 한 요청으로.
 *
 * 응답은 심볼을 키로 하는 객체이고, **일정을 못 가져온 종목은 아예 빠진다** —
 * 종목당 한 번씩 부르던 시절에 그 종목만 실패하고 나머지는 떴던 것과 같은 결과다.
 * 없는 키는 호출부가 빈 배열로 읽으면 된다.
 */
export function fetchEarningsBatchClient(
  symbols: string[],
): Promise<Record<string, EarningsTabData>> {
  const list = symbols.map((s) => s.trim()).filter(Boolean);
  if (list.length === 0) return Promise.resolve({});
  const path = `/api/v1/stocks/earnings/batch?symbols=${encodeURIComponent(list.join(","))}`;
  // 실적 일정은 날짜 단위로 움직인다. 홈을 오갈 때마다 다시 물어볼 값이 아니다.
  return cachedGet(
    path,
    () => apiRequest<Record<string, EarningsTabData>>(path, undefined, "실적 정보 조회 실패"),
    5 * 60_000,
  );
}

/**
 * 실적 일정은 날짜 단위로 움직인다 — 화면을 오갈 때마다 다시 물어볼 값이 아니다.
 *
 * 홈의 갱신 주기가 이 값보다 **길어야** 실제로 왕복이 일어난다(`lib/screen-refresh`).
 * 그 관계를 시험이 지키므로 `export`한다.
 */
export const EARNINGS_TTL = 5 * 60_000;

function marketParam(market: EarningsMarketFilter): string {
  return market === "all" ? "" : `&market=${market}`;
}

/**
 * 월 히트맵용 경량 응답 — 날짜별 건수만.
 *
 * 상세 행으로 세지 않는 이유: 시즌 달은 상세가 2,000행이라 격자 하나를 그리려고
 * 그만큼을 내리는 셈이 된다. 서버가 집계해 31행으로 준다.
 */
export function fetchEarningsSummary(
  from: string,
  to: string,
  market: EarningsMarketFilter,
): Promise<EarningsDaySummary[]> {
  const path = `/api/v1/earnings/calendar/summary?from=${from}&to=${to}${marketParam(market)}`;
  return cachedGet(
    path,
    () => apiRequest<EarningsDaySummary[]>(path, undefined, "실적 캘린더 조회 실패"),
    EARNINGS_TTL,
  );
}

/**
 * 선택 구간의 발표 목록.
 *
 * `symbols`는 「내 종목」 필터다 — 서버 인증에 묶지 않고 화면이 이미 들고 있는
 * 관심·보유 심볼을 넘긴다. 서버 구간 상한은 31일이다.
 */
export function fetchEarningsCalendar(
  from: string,
  to: string,
  market: EarningsMarketFilter,
  symbols?: string[],
  limit?: number,
): Promise<EarningsEvent[]> {
  const list = (symbols ?? []).map((s) => s.trim()).filter(Boolean);
  const symbolsParam = list.length > 0 ? `&symbols=${encodeURIComponent(list.join(","))}` : "";
  const limitParam = limit ? `&limit=${limit}` : "";
  const path = `/api/v1/earnings/calendar?from=${from}&to=${to}${marketParam(market)}${symbolsParam}${limitParam}`;
  return cachedGet(
    path,
    () => apiRequest<EarningsEvent[]>(path, undefined, "실적 캘린더 조회 실패"),
    EARNINGS_TTL,
  );
}

/**
 * 상단 스코어보드 집계.
 *
 * 목록과 따로 부르는 이유: 집계는 숫자 몇 개인데 목록은 수백 행이다. 목록에
 * 얹으면 스코어보드만 필요한 자리도 상세를 받게 된다.
 */
export function fetchEarningsStats(
  from: string,
  to: string,
  market: EarningsMarketFilter,
): Promise<EarningsStats> {
  const path = `/api/v1/earnings/stats?from=${from}&to=${to}${marketParam(market)}`;
  return cachedGet(
    path,
    () => apiRequest<EarningsStats>(path, undefined, "실적 요약 조회 실패"),
    EARNINGS_TTL,
  );
}

/** 컨센서스를 가장 크게 넘거나 밑돈 발표. `order`는 "best" | "worst". */
export function fetchEarningsSurprises(
  market: EarningsMarketFilter,
  order: "best" | "worst",
  limit = 20,
): Promise<EarningsEvent[]> {
  const path = `/api/v1/earnings/surprises?order=${order}&limit=${limit}${marketParam(market)}`;
  return cachedGet(
    path,
    () => apiRequest<EarningsEvent[]>(path, undefined, "서프라이즈 조회 실패"),
    EARNINGS_TTL,
  );
}

/** 우측 레일 3종(대형 발표 · 엇갈린 발표 · 꾸준한 기업)을 한 요청으로. */
export function fetchEarningsHighlights(
  market: EarningsMarketFilter,
): Promise<EarningsHighlights> {
  const path = `/api/v1/earnings/highlights?market=${market === "all" ? "" : market}`;
  return cachedGet(
    path,
    () => apiRequest<EarningsHighlights>(path, undefined, "실적 하이라이트 조회 실패"),
    EARNINGS_TTL,
  );
}
