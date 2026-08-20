import { apiRequest } from "@/lib/api-client";
import type {
  HoldingPosition,
  HoldingTrade,
  NewTradeInput,
  ValuationSeries,
} from "@/lib/holding-trades-types";

const EMPTY_SERIES: ValuationSeries = { points: [], exchangeRate: 0 };

export async function fetchHoldingTradesClient(symbol?: string): Promise<HoldingTrade[]> {
  const qs = symbol ? `?symbol=${encodeURIComponent(symbol)}` : "";
  const data = await apiRequest<HoldingTrade[] | null>(
    `/api/v1/user/holdings/trades${qs}`,
    undefined,
    "매매 이력을 불러오지 못했습니다.",
  );
  return data ?? [];
}

export function createHoldingTradeClient(input: NewTradeInput): Promise<HoldingPosition> {
  return apiRequest<HoldingPosition>(
    "/api/v1/user/holdings/trades",
    {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(input),
    },
    "매매 기록 저장에 실패했습니다.",
  );
}

export function deleteHoldingTradeClient(id: number, symbol: string): Promise<HoldingPosition> {
  return apiRequest<HoldingPosition>(
    `/api/v1/user/holdings/trades/${id}?symbol=${encodeURIComponent(symbol)}`,
    { method: "DELETE" },
    "매매 기록 삭제에 실패했습니다.",
  );
}

export async function fetchValuationSeriesClient(days: number): Promise<ValuationSeries> {
  const data = await apiRequest<ValuationSeries | null>(
    `/api/v1/user/holdings/valuation?days=${days}`,
    undefined,
    "평가금액 추이를 불러오지 못했습니다.",
  );
  return data ?? EMPTY_SERIES;
}

export function bulkRegisterTradesClient(
  items: Array<{
    symbol: string;
    quantity: number;
    price: number;
    currency: "KRW" | "USD";
    tradeDate: string;
  }>,
): Promise<{ ok: boolean }> {
  return apiRequest<{ ok: boolean }>(
    "/api/v1/user/holdings/trades/bulk-register",
    {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ items }),
    },
    "일괄 등록에 실패했습니다.",
  );
}
