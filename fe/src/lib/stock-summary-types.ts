export type NewsSentimentSummary = {
  positive: number;
  negative: number;
  neutral: number;
  total: number;
  averageScore: number;
};

/** Mirrors be/internal/model/stock_briefing.go. */
export type BriefingTone = "positive" | "negative" | "neutral";

export type BriefingPoint = {
  label: string;
  text: string;
  tone: BriefingTone;
};

export type PriceLevel = {
  label: string;
  price: number;
  distancePct: number;
};

export type BriefingScenario = {
  summary: string;
  levels?: PriceLevel[];
};

export type MarketBriefing = {
  asOf?: string;
  price: number;
  currency?: string;
  keyPoints?: BriefingPoint[];
  upside?: BriefingScenario;
  downside?: BriefingScenario;
};

export type StockSummaryData = {
  symbol: string;
  assetType: string;
  valuation: string;
  technical: string;
  insider: string;
  news: string;
  newsSentiment?: NewsSentimentSummary;
  keywords?: string[];
  briefing?: MarketBriefing;
  source: "llm" | "rule";
  generatedAt: string;
};
