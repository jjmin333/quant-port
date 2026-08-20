import type { QuoteAssetType } from "@/lib/quotes-types";

export type MarketCountry = "KR" | "US";

export type RankingType = "MARKET_TRADING_AMOUNT" | "MARKET_TRADING_VOLUME";

export type RankingDuration =
  | "realtime"
  | "1d"
  | "1w"
  | "1mo"
  | "3mo"
  | "6mo"
  | "1y";

export type RankingPrice = {
  lastPrice: string;
  basePrice: string;
  changeRate: string | null;
};

export type RankingItem = {
  rank: number;
  symbol: string;
  currency: "KRW" | "USD";
  price: RankingPrice;
  tradingVolume: string;
  tradingAmount: string;
};

export type RankingsResponse = {
  rankedAt: string | null;
  /** BE poller refresh time. Prefer over rankedAt for display. */
  refreshedAt?: string | null;
  rankings: RankingItem[];
};

export type MarketRadarParams = {
  market: MarketCountry;
  type: RankingType;
  duration: RankingDuration;
  count: number;
};

export type EnrichedRankingItem = RankingItem & {
  name: string;
  market: string;
  assetType?: QuoteAssetType;
  logoUrl?: string;
  rankDelta: number | null;
  isWatchlisted: boolean;
  isHeld: boolean;
};
