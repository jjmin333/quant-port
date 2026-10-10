import { apiRequest } from "@/lib/api-client";

/**
 * 「이 종목에 대한 내 기록」 — `be/internal/model/symbol_context.go`의 거울 (V2 요구 9).
 *
 * # 흐름을 닫는 한 줄입니다
 *
 * 발견 → 근거 → 판단 → 반영 → 추적의 다섯 단계 화면은 전부 있는데, 종목 화면이
 * 사용자에 대해 아는 것은 「보유 여부」 한 비트뿐이었습니다. 이미 보유 중인데도
 * 「보유종목 등록」을 똑같이 내밀고 있었습니다.
 *
 * # 없는 것도 남깁니다
 *
 * 관심에 안 담았으면 점선 칩으로 그립니다. 지우지 않는 이유는 「아직 안 했다」도
 * 흐름의 한 칸이고, 다음에 할 일을 가리키기 때문입니다.
 */

export type SymbolContext = {
  symbol: string;
  held: boolean;
  quantity?: number;
  avgPrice?: number;
  currency?: string;
  watchlisted: boolean;
  stampCount: number;
  reportCount: number;
  targetPrice?: number;
  /** "stronger" | "steady" | "weaker" | "unknown" — 보유 중일 때만 옵니다. */
  diagnosisVerdict?: string;
  diagnosisSignals?: number;
};

export async function fetchSymbolContextClient(symbol: string): Promise<SymbolContext | null> {
  return apiRequest<SymbolContext | null>(
    `/api/v1/user/symbols/${encodeURIComponent(symbol)}/context`,
    undefined,
    "내 기록을 불러오지 못했습니다",
  );
}

/** 하나라도 있을 때만 줄을 그립니다 — 점선 칩 여섯 개는 안내가 아니라 잡음입니다. */
export function hasAnyRecord(c: SymbolContext | null): boolean {
  if (!c) return false;
  return c.held || c.watchlisted || c.stampCount > 0 || c.reportCount > 0 || c.targetPrice != null;
}

const VERDICT_LABEL: Record<string, string> = {
  stronger: "강화",
  steady: "유지",
  weaker: "약화",
  unknown: "판단 보류",
};

export function verdictLabel(verdict: string | undefined): string | null {
  if (!verdict) return null;
  return VERDICT_LABEL[verdict] ?? null;
}

/**
 * 판정 칩의 색 — **약화만 경고색**입니다.
 *
 * 강화·유지에 색을 주면 줄 전체가 알록달록해져 정작 봐야 할 것이 안 보입니다.
 * 확인이 필요한 것 하나만 도드라지면 됩니다.
 */
export function verdictTone(verdict: string | undefined): "warn" | "on" {
  return verdict === "weaker" ? "warn" : "on";
}

export function formatQuantity(q: number | undefined): string {
  if (q == null) return "";
  // 소수 수량(해외 소수점 매매)은 버리지 않되, 정수면 소수점을 안 붙입니다.
  return Number.isInteger(q) ? q.toLocaleString("ko-KR") : q.toLocaleString("ko-KR", { maximumFractionDigits: 4 });
}

export function formatMoney(v: number | undefined, currency?: string): string {
  if (v == null) return "";
  const usd = (currency ?? "").toUpperCase() === "USD";
  return v.toLocaleString("ko-KR", {
    minimumFractionDigits: usd ? 2 : 0,
    maximumFractionDigits: usd ? 2 : 0,
  });
}
