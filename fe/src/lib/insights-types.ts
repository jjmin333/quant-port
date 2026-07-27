export type PortfolioHistoryPoint = {
  at: string;
  label?: string;
  value: number;
};

export type PortfolioHistory = {
  range: string;
  points: PortfolioHistoryPoint[];
  dataSince?: string;
};

export type RiskAlert = {
  code: string;
  type: "warning" | "info" | "critical";
  message: string;
  metrics?: Record<string, number>;
};

export type BriefingEvent = {
  date: string;
  symbol: string;
  event: string;
};

export type MorningBriefing = {
  summary: string;
  highlights: string[];
  earnings: BriefingEvent[];
  disclosures: BriefingEvent[];
  volatility: string;
  generatedAt: string;
  source: "llm" | "rules";
};

export type DashboardInsights = {
  briefing: MorningBriefing;
  risks: RiskAlert[];
  generatedAt: string;
};

export type InsightsDiagnostics = {
  llmEnabled: boolean;
  dartEnabled: boolean;
  earningsEnabled: boolean;
  checkedAt: string;
  hints?: string[];
};

export type HistoryRange = "1d" | "1w" | "1m" | "1y";
