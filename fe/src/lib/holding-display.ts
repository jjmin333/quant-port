import type { DisplayCurrency, UnifiedHolding } from "@/lib/portfolio-types";

export type HoldingDisplay = {
  avgPrice: number;
  currentPrice: number;
  marketValue: number;
  marketValueKRW: number;
  returnRate: number;
  currency: "KRW" | "USD";
  exchangeRate: number;
  purchaseFxRate: number;
  prevClose: number;
  regularClose: number;
  regularChange: number;
  regularChangeRate: number;
  extendedChange: number;
  extendedChangeRate: number;
  marketSession: string;
  sessionChange: number;
  sessionChangeRate: number;
  purchaseDate?: string;
  sincePurchaseHigh?: number;
  sincePurchaseHighDate?: string;
  sincePurchaseHighReturnRate?: number;
  sincePurchaseLow?: number;
  sincePurchaseLowDate?: string;
  sincePurchaseLowReturnRate?: number;
};

function priceReturnRate(price: number | undefined, avgPrice: number): number | undefined {
  if (price == null || price <= 0 || avgPrice <= 0) return undefined;
  return ((price - avgPrice) / avgPrice) * 100;
}

function withRangeReturnRates(
  base: Omit<
    HoldingDisplay,
    "sincePurchaseHighReturnRate" | "sincePurchaseLowReturnRate"
  >,
): HoldingDisplay {
  return {
    ...base,
    sincePurchaseHighReturnRate: priceReturnRate(base.sincePurchaseHigh, base.avgPrice),
    sincePurchaseLowReturnRate: priceReturnRate(base.sincePurchaseLow, base.avgPrice),
  };
}

export function extendedSessionLabel(session: string, currency: "KRW" | "USD"): string {
  switch (session) {
    case "after":
      return currency === "USD" ? "애프터마켓" : "시간외";
    case "pre":
      return currency === "USD" ? "프리마켓" : "장전";
    case "day":
      return "데이마켓";
    default:
      return currency === "USD" ? "애프터마켓" : "시간외";
  }
}

export function shouldShowExtendedChange(display: HoldingDisplay): boolean {
  if (display.marketSession === "regular") {
    return false;
  }
  if (
    display.marketSession === "after" ||
    display.marketSession === "pre" ||
    display.marketSession === "day"
  ) {
    return true;
  }
  return display.regularClose > 0;
}

function mapSessionFields(holding: UnifiedHolding) {
  return {
    prevClose: holding.prevClose ?? 0,
    regularClose: holding.regularClose ?? 0,
    regularChange: holding.regularChange ?? 0,
    regularChangeRate: holding.regularChangeRate ?? 0,
    extendedChange: holding.extendedChange ?? 0,
    extendedChangeRate: holding.extendedChangeRate ?? 0,
    marketSession: holding.marketSession ?? "regular",
    sessionChange: holding.sessionChange ?? 0,
    sessionChangeRate: holding.sessionChangeRate ?? 0,
  };
}

/** @deprecated Currency toggle removed — kept for call-site compatibility. */
export type LegacyDisplayCurrency = DisplayCurrency;

export function resolveHoldingDisplay(holding: UnifiedHolding, exchangeRate: number): HoldingDisplay {
  const purchaseFx = holding.purchaseFxRate || exchangeRate;

  if (holding.currency !== "USD") {
    return withRangeReturnRates({
      avgPrice: holding.avgPrice,
      currentPrice: holding.currentPrice,
      marketValue: holding.marketValueNative,
      marketValueKRW: holding.marketValueKRW,
      returnRate: holding.returnRateNative,
      currency: "KRW",
      exchangeRate: 1,
      purchaseFxRate: 1,
      ...mapSessionFields(holding),
      purchaseDate: holding.purchaseDate,
      sincePurchaseHigh: holding.sincePurchaseHigh,
      sincePurchaseHighDate: holding.sincePurchaseHighDate,
      sincePurchaseLow: holding.sincePurchaseLow,
      sincePurchaseLowDate: holding.sincePurchaseLowDate,
    });
  }

  return withRangeReturnRates({
    avgPrice: holding.avgPrice,
    currentPrice: holding.currentPrice,
    marketValue: holding.marketValueNative,
    marketValueKRW: holding.marketValueKRW,
    returnRate: holding.returnRateNative,
    currency: "USD",
    exchangeRate,
    purchaseFxRate: purchaseFx,
    ...mapSessionFields(holding),
    purchaseDate: holding.purchaseDate,
    sincePurchaseHigh: holding.sincePurchaseHigh,
    sincePurchaseHighDate: holding.sincePurchaseHighDate,
    sincePurchaseLow: holding.sincePurchaseLow,
    sincePurchaseLowDate: holding.sincePurchaseLowDate,
  });
}

export function hasUsHoldings(holdings: UnifiedHolding[]): boolean {
  return holdings.some((h) => h.currency === "USD");
}
