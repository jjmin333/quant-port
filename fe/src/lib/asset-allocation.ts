import type { PortfolioSummary, UnifiedHolding } from "@/lib/portfolio-types";

export type AssetCategory = "domestic" | "us" | "etf" | "bond" | "cash";

export type AllocationSlice = {
  name: string;
  category: AssetCategory;
  amountKRW: number;
  value: number;
  color: string;
};

const CATEGORY_META: Record<
  AssetCategory,
  { name: string; color: string; order: number }
> = {
  domestic: { name: "국내주식", color: "#3B82F6", order: 1 },
  us: { name: "미국주식", color: "#8B5CF6", order: 2 },
  etf: { name: "ETF", color: "#06B6D4", order: 3 },
  cash: { name: "현금", color: "#64748B", order: 4 },
  bond: { name: "채권", color: "#10B981", order: 5 },
};

export function calculateAssetAllocation(summary: PortfolioSummary): AllocationSlice[] {
  const categoryTotals = new Map<AssetCategory, number>();

  for (const holding of summary.holdings) {
    const current = categoryTotals.get(holding.assetType) ?? 0;
    categoryTotals.set(holding.assetType, current + holding.marketValueKRW);
  }

  if (summary.totalCashKRW > 0) {
    categoryTotals.set("cash", summary.totalCashKRW);
  }

  const totalAssets = Array.from(categoryTotals.values()).reduce((sum, v) => sum + v, 0);
  if (totalAssets === 0) return [];

  return Array.from(categoryTotals.entries())
    .filter(([, amount]) => amount > 0)
    .map(([category, amountKRW]) => {
      const meta = CATEGORY_META[category];
      return {
        name: meta.name,
        category,
        amountKRW,
        value: Math.round((amountKRW / totalAssets) * 1000) / 10,
        color: meta.color,
      };
    })
    .sort((a, b) => CATEGORY_META[a.category].order - CATEGORY_META[b.category].order);
}

export function sortHoldingsByValue(holdings: UnifiedHolding[]): UnifiedHolding[] {
  return [...holdings].sort((a, b) => b.marketValueKRW - a.marketValueKRW);
}
