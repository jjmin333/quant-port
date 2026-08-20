import { apiRequest } from "@/lib/api-client";
import type { InsiderTabData } from "@/lib/insider-types";

export function fetchInsiderClient(symbol: string): Promise<InsiderTabData> {
  return apiRequest<InsiderTabData>(
    `/api/v1/stocks/${encodeURIComponent(symbol)}/insider`,
    undefined,
    "내부자 거래 정보 조회 실패",
  );
}
