import { apiRequest } from "@/lib/api-client";
import type { DailyCloseSeries } from "@/lib/daily-close-types";

const EMPTY: DailyCloseSeries = { currency: "KRW", rows: [] };

export async function fetchStockDailyCloses(
  symbol: string,
  limit = 60,
): Promise<DailyCloseSeries> {
  const data = await apiRequest<DailyCloseSeries | null>(
    `/api/v1/stocks/${encodeURIComponent(symbol)}/daily-closes?limit=${limit}`,
    undefined,
    "일별 시세를 불러오지 못했습니다.",
  );
  return data ?? EMPTY;
}

export async function fetchIndexDailyCloses(
  indexId: string,
  limit = 60,
): Promise<DailyCloseSeries> {
  const data = await apiRequest<DailyCloseSeries | null>(
    `/api/v1/indices/${encodeURIComponent(indexId)}/daily-closes?limit=${limit}`,
    undefined,
    "지수 일별 시세를 불러오지 못했습니다.",
  );
  return data ?? EMPTY;
}
