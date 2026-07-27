import type { PortfolioSummary } from "@/lib/portfolio-types";

export type GenericAllocationSlice = {
  name: string;
  key: string;
  amountKRW: number;
  value: number;
  color: string;
};

const SLICE_COLORS = [
  "#3B82F6",
  "#8B5CF6",
  "#06B6D4",
  "#10B981",
  "#F59E0B",
  "#EC4899",
  "#64748B",
  "#EF4444",
];

const ASSET_TYPE_LABELS: Record<string, string> = {
  domestic: "국내주식",
  us: "미국주식",
  etf: "ETF",
  bond: "채권",
  cash: "현금",
};

function toSlices(entries: Map<string, { name: string; amount: number }>): GenericAllocationSlice[] {
  const total = Array.from(entries.values()).reduce((sum, e) => sum + e.amount, 0);
  if (total === 0) return [];

  return Array.from(entries.entries())
    .filter(([, e]) => e.amount > 0)
    .map(([key, e], index) => ({
      key,
      name: e.name,
      amountKRW: e.amount,
      value: Math.round((e.amount / total) * 1000) / 10,
      color: SLICE_COLORS[index % SLICE_COLORS.length],
    }))
    .sort((a, b) => b.amountKRW - a.amountKRW);
}

export function calculateCountryAllocation(summary: PortfolioSummary): GenericAllocationSlice[] {
  const totals = new Map<string, { name: string; amount: number }>();

  for (const holding of summary.holdings) {
    const country =
      holding.assetType === "us" || holding.currency === "USD" ? "미국" : "한국";
    const current = totals.get(country) ?? { name: country, amount: 0 };
    totals.set(country, { name: country, amount: current.amount + holding.marketValueKRW });
  }

  if (summary.totalCashKRW > 0) {
    const current = totals.get("한국") ?? { name: "한국", amount: 0 };
    totals.set("한국", { name: "한국", amount: current.amount + summary.totalCashKRW });
  }

  return toSlices(totals);
}

/** Holdings lack sector metadata; group by asset type as allocation proxy. */
export function calculateSectorAllocation(summary: PortfolioSummary): GenericAllocationSlice[] {
  const totals = new Map<string, { name: string; amount: number }>();

  for (const holding of summary.holdings) {
    const label = ASSET_TYPE_LABELS[holding.assetType] ?? holding.assetType;
    const current = totals.get(holding.assetType) ?? { name: label, amount: 0 };
    totals.set(holding.assetType, {
      name: label,
      amount: current.amount + holding.marketValueKRW,
    });
  }

  if (summary.totalCashKRW > 0) {
    const current = totals.get("cash") ?? { name: "현금", amount: 0 };
    totals.set("cash", { name: "현금", amount: current.amount + summary.totalCashKRW });
  }

  return toSlices(totals);
}

export function calculateRiskSummary(summary: PortfolioSummary) {
  const total = summary.totalAssetsKRW;
  const holdings = [...summary.holdings].sort(
    (a, b) => b.marketValueKRW - a.marketValueKRW,
  );
  const top = holdings[0];
  const topWeight = total > 0 && top ? (top.marketValueKRW / total) * 100 : 0;
  const country = calculateCountryAllocation(summary);
  const usWeight = country.find((c) => c.key === "미국")?.value ?? 0;

  return {
    holdingsCount: holdings.length,
    topSymbol: top?.symbol ?? "—",
    topName: top?.name ?? "—",
    topWeight,
    usWeight,
    totalReturnRate: summary.totalReturnRate,
    todayChangeRate: summary.todayChangeRate,
  };
}
