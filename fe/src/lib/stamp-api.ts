import { apiRequest } from "@/lib/api-client";
import { cachedGet, invalidateFetchCache } from "@/lib/fetch-cache";
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

/**
 * 이 종목의 스탬프 수와 쿨다운. **종목 헤더가 뜰 때마다 두 벌씩** 나가고 있었다 —
 * 스탬프 버튼이 헤더의 좁은 화면용·넓은 화면용 두 갈래에 각각 그려지기 때문이다.
 *
 * 짧게 재사용하되, 스탬프를 찍거나 고치면 `invalidateStampCache()`가 즉시 버린다 —
 * 방금 찍은 것이 개수에 안 잡히면 「저장이 안 됐나」로 읽힌다.
 */
export async function fetchStampStateClient(symbol: string): Promise<StampState> {
  const path = `/api/v1/stamps/state?symbol=${encodeURIComponent(symbol)}`;
  return cachedGet(path, () => apiRequest<StampState>(path), 10_000);
}

/** 스탬프를 바꾼 뒤 목록·상태를 다시 읽게 한다. */
function invalidateStampCache(): void {
  invalidateFetchCache("/api/v1/stamps");
}

export async function createStampClient(input: StampCreateInput): Promise<Stamp> {
  const created = await apiRequest<Stamp>("/api/v1/stamps", { method: "POST", ...jsonBody(input) });
  invalidateStampCache();
  return created;
}

export async function updateStampClient(id: number, input: StampUpdateInput): Promise<Stamp> {
  const updated = await apiRequest<Stamp>(`/api/v1/stamps/${id}`, {
    method: "PATCH",
    ...jsonBody(input),
  });
  invalidateStampCache();
  return updated;
}

/**
 * 예상 주가·검증 기간 정정. 스탬프 직후 유예 시간이 지나면 서버가 409로 막는다 —
 * 가격이 움직이는 걸 보고 나서 고칠 수 있는 예측은 예측이 아니기 때문이다.
 */
export async function reviseStampClient(id: number, input: StampReviseInput): Promise<Stamp> {
  const revised = await apiRequest<Stamp>(`/api/v1/stamps/${id}/prediction`, {
    method: "PATCH",
    ...jsonBody(input),
  });
  invalidateStampCache();
  return revised;
}

export async function openStampClient(id: number): Promise<Stamp> {
  const opened = await apiRequest<Stamp>(`/api/v1/stamps/${id}/open`, { method: "POST" });
  invalidateStampCache();
  return opened;
}

export async function deleteStampClient(id: number): Promise<void> {
  await apiRequest<{ id: number }>(`/api/v1/stamps/${id}`, { method: "DELETE" });
  invalidateStampCache();
}
