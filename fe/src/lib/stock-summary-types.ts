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
  /**
   * 개요 탭 「AI 핵심 요약」 문단 — 밸류에이션·기술적 분석·내부자 거래·뉴스
   * **네 갈래를 모두 근거로** 서버가 만든다(BE StockSummaryView.Overview).
   *
   * 이 필드가 생기기 전 캐시에는 없을 수 있어 선택 필드다. 그때는 화면이 섹션들을
   * 직접 이어 붙인다(`stockSummaryHeadline`).
   */
  overview?: string;
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
