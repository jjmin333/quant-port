import type { FundamentalsData } from "@/lib/fundamentals-types";
import type { QuoteDetail } from "@/lib/quotes-types";

/** Trailing EPS from stored fundamentals (price-independent). */
export function deriveTrailingEPS(f: FundamentalsData): number | null {
  if (f.epsDiluted != null && f.epsDiluted > 0) return f.epsDiluted;
  if (f.epsBasic != null && f.epsBasic > 0) return f.epsBasic;
  if (f.netIncome != null && f.sharesOutstanding != null && f.sharesOutstanding > 0) {
    return f.netIncome / f.sharesOutstanding;
  }
  if (f.per != null && f.price != null && f.per > 0 && f.price > 0) {
    return f.price / f.per;
  }
  return null;
}

/** Book value per share from stored fundamentals (price-independent). */
export function deriveBookValuePerShare(f: FundamentalsData): number | null {
  if (f.totalEquity != null && f.sharesOutstanding != null && f.sharesOutstanding > 0) {
    return f.totalEquity / f.sharesOutstanding;
  }
  if (f.pbr != null && f.price != null && f.pbr > 0 && f.price > 0) {
    return f.price / f.pbr;
  }
  return null;
}

export function computeLivePER(
  quote: QuoteDetail | null,
  fundamentals: FundamentalsData | null,
): number | null {
  if (!fundamentals) return null;
  const price = quote?.currentPrice;
  if (price == null || price <= 0) return fundamentals.per ?? null;

  const eps = deriveTrailingEPS(fundamentals);
  if (eps != null && eps > 0) return price / eps;

  return fundamentals.per ?? null;
}

export function computeLivePBR(
  quote: QuoteDetail | null,
  fundamentals: FundamentalsData | null,
): number | null {
  if (!fundamentals) return null;
  const price = quote?.currentPrice;
  if (price == null || price <= 0) return fundamentals.pbr ?? null;

  const bvps = deriveBookValuePerShare(fundamentals);
  if (bvps != null && bvps > 0) return price / bvps;

  return fundamentals.pbr ?? null;
}

/** Merge live PER/PBR (and price) into fundamentals for display panels. */
export function applyLiveValuationMultiples(
  fundamentals: FundamentalsData | null,
  quote: QuoteDetail | null,
): FundamentalsData | null {
  if (!fundamentals) return null;

  const per = computeLivePER(quote, fundamentals);
  const pbr = computeLivePBR(quote, fundamentals);
  const price = quote?.currentPrice ?? fundamentals.price;

  return {
    ...fundamentals,
    price: price && price > 0 ? price : fundamentals.price,
    per: per ?? fundamentals.per,
    pbr: pbr ?? fundamentals.pbr,
  };
}
