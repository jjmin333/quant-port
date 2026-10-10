import { apiRequest } from "@/lib/api-client";

/**
 * 국내 ETF 점검 — `be/internal/service/etf_score.go`의 거울 (2026-09-24 사용자 결정).
 *
 * 주식 점수와 다른 잣대다: 괴리율·추적오차·유동성·규모를 **같은 비교 집단**(일반 / 채권·금리 /
 * 레버리지·인버스) 안의 백분위로 바꿔 평균한다. 총보수는 소스가 없어 빠졌다.
 */

export type EtfPeerGroup = "plain" | "lowvol" | "leveraged";

export const ETF_PEER_LABEL: Record<EtfPeerGroup, string> = {
  plain: "일반",
  lowvol: "채권·금리",
  leveraged: "레버리지·인버스",
};

export type EtfPart = { value: number | null; pct: number };

export type EtfCheck = {
  symbol: string;
  name?: string;
  tradeDate: string;
  score: number;
  peerGroup: EtfPeerGroup;
  peerCount: number;
  premiumPct?: number;
  trackingErrorPct?: number;
  beta?: number;
  tradeValueMedian?: number;
  netAssets?: number;
  indexName?: string;
  parts: Partial<Record<"premium" | "tracking" | "liquidity" | "size", EtfPart>>;
  rank?: number;
};

export type EtfTop = { tradeDate: string; items: EtfCheck[] };

export const ETF_PART_META: {
  id: "premium" | "tracking" | "liquidity" | "size";
  label: string;
  hint: string;
}[] = [
  { id: "premium", label: "괴리율", hint: "20일 평균 |가격 ÷ NAV − 1| — 낮을수록 좋음" },
  { id: "tracking", label: "추적오차", hint: "60일 NAV vs 기초지수 잔차(연율) — 낮을수록 좋음" },
  { id: "liquidity", label: "유동성", hint: "20일 거래대금 중앙값 — 높을수록 좋음" },
  { id: "size", label: "규모", hint: "순자산 — 클수록 좋음" },
];

export function fetchEtfCheck(symbol: string): Promise<EtfCheck | null> {
  return apiRequest<EtfCheck | null>(`/api/v1/stocks/${encodeURIComponent(symbol)}/etf-check`);
}

export function fetchEtfTop(group: EtfPeerGroup | "", limit = 50): Promise<EtfTop> {
  const q = new URLSearchParams({ limit: String(limit) });
  if (group) q.set("group", group);
  return apiRequest<EtfTop>(`/api/v1/etf/scores/top?${q.toString()}`);
}

/** 국내 ETF인가 — 점검 카드를 부를지 정한다(없으면 서버가 null을 준다). */
export function isDomesticEtf(stock: { securityType?: string; currency?: string }): boolean {
  return (stock.securityType ?? "").toUpperCase() === "ETF" && stock.currency === "KRW";
}

/** 원 → 「1.2조」·「350억」. */
export function formatKrwShort(v: number | null | undefined): string {
  if (v == null || !Number.isFinite(v)) return "-";
  if (Math.abs(v) >= 1e12) return `${(v / 1e12).toFixed(1)}조`;
  if (Math.abs(v) >= 1e8) return `${Math.round(v / 1e8).toLocaleString("ko-KR")}억`;
  return `${Math.round(v / 1e4).toLocaleString("ko-KR")}만`;
}

export function formatEtfPartValue(id: string, part: EtfPart | undefined): string {
  if (!part || part.value == null) return "-";
  switch (id) {
    case "premium":
      return `${part.value.toFixed(2)}%`;
    case "tracking":
      return `${part.value.toFixed(2)}%`;
    default:
      return formatKrwShort(part.value);
  }
}
