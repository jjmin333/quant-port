import { getApiBaseUrl } from "@/lib/api-client";
import type { ApiResponse } from "@/lib/api-types";
import { fetchWithDeadline } from "@/lib/fetch-timeout";

/*
날짜별 「AI 점수 상위」 보관 (2026-10-21 티켓 QP-12). BE `/stocks/scores/daily-top`이 백테스트용
`score_ranking_history`에서 그날 국내·해외 종합 상위 20과 직전 보관일 순위를 준다.
*/

export type DailyTopRow = { rank: number; symbol: string; name: string; score: number; prevRank?: number };

export type DailyTop = {
  date: string;
  prevDate: string;
  nextDate: string;
  /** 보관된 날짜 전부, 최신 먼저. */
  dates: string[];
  domestic: DailyTopRow[];
  overseas: DailyTopRow[];
};

export const DAILY_TOP_DATE_RE = /^\d{4}-\d{2}-\d{2}$/;

/** date를 비우면 최신 보관일. 지난 날은 바뀌지 않으므로 오래 캐시한다. */
export async function fetchDailyTopServer(date?: string): Promise<DailyTop | null> {
  const qs = date ? `?date=${date}` : "";
  try {
    const res = await fetchWithDeadline(`${getApiBaseUrl()}/api/v1/stocks/scores/daily-top${qs}`, {
      next: { revalidate: date ? 6 * 3600 : 1800 },
    });
    if (!res.ok) return null;
    const body = (await res.json()) as ApiResponse<DailyTop>;
    return body.data ?? null;
  } catch {
    return null;
  }
}

/** 「2026-10-20」 → 「10월 20일」. */
export function koreanMonthDay(date: string): string {
  return `${Number(date.slice(5, 7))}월 ${Number(date.slice(8, 10))}일`;
}
