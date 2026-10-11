import { getApiBaseUrl } from "@/lib/api-client";
import type { ApiResponse } from "@/lib/api-types";
import { fetchWithDeadline } from "@/lib/fetch-timeout";
import type { StockScoreData } from "@/lib/stock-score-api";

/*
홈 비로그인 블록의 재료 (2026-10-10 사용자 결정 — 시안 `docs/mockups/home-guest-v2.html` A안).

전부 **저장분 조회**이고 서버에서 한 번에 받는다 — 처음 온 사람과 크롤러가 같은 HTML을 본다.
점수는 하루 한 번(00:25) 바뀌므로 10분 접는다. 하나가 실패해도 그 칸만 빠진다.
*/

const REVALIDATE = 600;

/** 대표 종목 — 고정(사용자 결정). 국내 둘·미국 넷, 처음 온 사람이 이름을 아는 종목. */
export const GUEST_FEATURED = ["005930", "000660", "NVDA", "AAPL", "TSLA", "MSFT"] as const;

async function get<T>(path: string): Promise<T | null> {
  try {
    const res = await fetchWithDeadline(`${getApiBaseUrl()}${path}`, { next: { revalidate: REVALIDATE } });
    if (!res.ok) return null;
    const body = (await res.json()) as ApiResponse<T>;
    return body.data ?? null;
  } catch {
    return null;
  }
}

export type GuestShowcase = { featured: StockScoreData[] };

function scoreList(data: unknown): StockScoreData[] {
  if (Array.isArray(data)) return data as StockScoreData[];
  return data ? (Object.values(data as Record<string, StockScoreData>)) : [];
}

export async function fetchGuestShowcase(): Promise<GuestShowcase> {
  const featured = scoreList(await get<unknown>(`/api/v1/stocks/scores/batch?symbols=${GUEST_FEATURED.join(",")}`));
  const order = new Map<string, number>(GUEST_FEATURED.map((s, i) => [s, i]));
  featured.sort((a, b) => (order.get(a.symbol) ?? 99) - (order.get(b.symbol) ?? 99));
  return { featured: featured.filter((s) => s.score != null) };
}
