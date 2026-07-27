export type CoverageTier = "full" | "basic" | "unavailable" | "not_applicable";

export type FundamentalsCalcStatus = "ok" | "partial" | "failed" | "pending" | "";

export type FundamentalsData = {
  symbol: string;
  assetType?: string;
  productLabel?: string;
  message?: string;
  dataSource?: string;
  fiscalYear?: number;
  coverageTier: CoverageTier;
  calcStatus?: FundamentalsCalcStatus;
  calcError?: string;
  fetchedAt?: string;

  price?: number;
  per?: number;
  pbr?: number;

  roe?: number;
  operatingMargin?: number;
  revenueGrowthYoy?: number;
  epsGrowthYoy?: number;
  fwdPer?: number;
  cyPer?: number;
  debtRatio?: number;
  roa?: number;
  netProfitMargin?: number;
  currentRatio?: number;

  revenue?: number;
  netIncome?: number;
  totalEquity?: number;
};

export type MetricTone = "positive" | "negative" | "neutral" | "accent";

export type FundamentalMetric = {
  id: string;
  label: string;
  value: string;
  verdict: string;
  tone: MetricTone;
  gauge: number;
  available: boolean;
};

export type StockFundamentalsPanel = {
  symbol: string;
  name: string;
  currency: "KRW" | "USD";
  coverageTier: CoverageTier;
  calcStatus: FundamentalsCalcStatus;
  dataSource: string;
  fiscalYear?: number;
  score: number;
  rating: string;
  summary: string;
  tags: string[];
  metrics: FundamentalMetric[];
  price?: number;
  message?: string;
  productLabel?: string;
};

export const COVERAGE_TIER_LABEL: Record<CoverageTier, string> = {
  full: "전체 지표",
  basic: "기본 지표",
  unavailable: "데이터 없음",
  not_applicable: "재무제표 미제공",
};
