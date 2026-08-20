import { apiRequest } from "@/lib/api-client";

export type PriceTargetRecord = {
  id: number;
  symbol: string;
  targetPrice: number;
  /** 2차 목표. 사용자가 한 단계만 잡았으면 없다. */
  targetPrice2?: number;
  /** 논지가 틀렸다고 인정하는 자리. */
  stopPrice?: number;
  /** 등록 시점 주가 — 서버가 재고, 목표를 고쳐도 바뀌지 않는다. */
  basePrice?: number;
  baseDate?: string;
  /** 일봉 고/저가 목표에 닿은 첫날. 저장값이 아니라 읽을 때 계산된다. */
  reachedDate?: string;
  peakPrice?: number;
  troughPrice?: number;
  currency: "KRW" | "USD";
  note: string;
  createdAt: string;
  updatedAt: string;
};

/** 저장 가능한 필드. 등록가는 없다 — 사용자가 적을 수 있으면 박제가 아니다. */
export type PriceTargetWriteInput = {
  symbol: string;
  targetPrice: number;
  targetPrice2?: number;
  stopPrice?: number;
  currency: "KRW" | "USD";
  note?: string;
};

export async function fetchPriceTargetsClient(): Promise<PriceTargetRecord[]> {
  const data = await apiRequest<PriceTargetRecord[] | null>(
    "/api/v1/user/price-targets",
    undefined,
    "목표가를 불러오지 못했습니다.",
  );
  return data ?? [];
}

export function createPriceTargetClient(input: PriceTargetWriteInput): Promise<PriceTargetRecord> {
  return apiRequest<PriceTargetRecord>(
    "/api/v1/user/price-targets",
    {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(input),
    },
    "목표가 저장에 실패했습니다.",
  );
}

export function updatePriceTargetClient(
  id: number,
  input: Omit<PriceTargetWriteInput, "symbol">,
): Promise<PriceTargetRecord> {
  return apiRequest<PriceTargetRecord>(
    `/api/v1/user/price-targets/${id}`,
    {
      method: "PUT",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(input),
    },
    "목표가 수정에 실패했습니다.",
  );
}

export function deletePriceTargetClient(id: number): Promise<{ id: number }> {
  return apiRequest<{ id: number }>(
    `/api/v1/user/price-targets/${id}`,
    { method: "DELETE" },
    "목표가 삭제에 실패했습니다.",
  );
}
