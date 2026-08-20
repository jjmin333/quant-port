import { apiRequest } from "@/lib/api-client";
import type {
  Stamp,
  StampCreateInput,
  StampReviseInput,
  StampState,
  StampSummary,
  StampUpdateInput,
} from "@/lib/stamp-types";

function jsonBody(payload?: unknown): RequestInit {
  return {
    headers: { "Content-Type": "application/json" },
    body: payload ? JSON.stringify(payload) : undefined,
  };
}

export type StampListParams = {
  symbol?: string;
  stance?: string;
  from?: string;
  to?: string;
  limit?: number;
};

export async function fetchStampsClient(params: StampListParams = {}): Promise<Stamp[]> {
  const query = new URLSearchParams();
  for (const [key, value] of Object.entries(params)) {
    if (value !== undefined && value !== null && value !== "") query.set(key, String(value));
  }
  const suffix = query.toString() ? `?${query}` : "";
  const data = await apiRequest<Stamp[] | null>(`/api/v1/stamps${suffix}`);
  return data ?? [];
}

export async function fetchStampSummaryClient(): Promise<StampSummary> {
  return apiRequest<StampSummary>("/api/v1/stamps/summary");
}

export async function fetchStampStateClient(symbol: string): Promise<StampState> {
  return apiRequest<StampState>(`/api/v1/stamps/state?symbol=${encodeURIComponent(symbol)}`);
}

export async function createStampClient(input: StampCreateInput): Promise<Stamp> {
  return apiRequest<Stamp>("/api/v1/stamps", { method: "POST", ...jsonBody(input) });
}

export async function updateStampClient(id: number, input: StampUpdateInput): Promise<Stamp> {
  return apiRequest<Stamp>(`/api/v1/stamps/${id}`, { method: "PATCH", ...jsonBody(input) });
}

/**
 * 예상 주가·검증 기간 정정. 스탬프 직후 유예 시간이 지나면 서버가 409로 막는다 —
 * 가격이 움직이는 걸 보고 나서 고칠 수 있는 예측은 예측이 아니기 때문이다.
 */
export async function reviseStampClient(id: number, input: StampReviseInput): Promise<Stamp> {
  return apiRequest<Stamp>(`/api/v1/stamps/${id}/prediction`, {
    method: "PATCH",
    ...jsonBody(input),
  });
}

export async function openStampClient(id: number): Promise<Stamp> {
  return apiRequest<Stamp>(`/api/v1/stamps/${id}/open`, { method: "POST" });
}

export async function deleteStampClient(id: number): Promise<void> {
  await apiRequest<{ id: number }>(`/api/v1/stamps/${id}`, { method: "DELETE" });
}
